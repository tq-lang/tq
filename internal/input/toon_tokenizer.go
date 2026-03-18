package input

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

// TOONTokenReader emits [path, value] streaming pairs from TOON input with
// O(depth) memory, matching the JSON TokenReader's output format.
type TOONTokenReader struct {
	scanner    *bufio.Scanner
	path       []any           // current path
	stack      []containerInfo // open containers
	pending    []any           // queued emissions
	done       bool
	peeked     bool
	peekLine   string
	peekOK     bool
	indentSize int
	hasTopKey  bool // whether we've pushed a top-level key onto path
}

type containerKind int

const (
	containerObject containerKind = iota
	containerListArray
	containerTabularArray
)

type containerInfo struct {
	kind       containerKind
	indent     int      // indent level where children live
	fields     []string // tabular column names
	delimiter  rune     // tabular delimiter
	index      int      // next array element index
	childCount int      // how many children have been processed
}

// NewTOONTokenReader creates a streaming TOON tokenizer.
func NewTOONTokenReader(r io.Reader) *TOONTokenReader {
	return &TOONTokenReader{
		scanner:    bufio.NewScanner(r),
		path:       []any{},
		indentSize: 2,
	}
}

// Next returns the next streaming pair.
func (tr *TOONTokenReader) Next() (any, bool, error) {
	for {
		if len(tr.pending) > 0 {
			v := tr.pending[0]
			tr.pending = tr.pending[1:]
			return v, true, nil
		}
		if tr.done {
			return nil, false, nil
		}
		if err := tr.advance(); err != nil {
			tr.done = true
			if errors.Is(err, io.EOF) {
				tr.emitClosing()
				if len(tr.pending) > 0 {
					v := tr.pending[0]
					tr.pending = tr.pending[1:]
					return v, true, nil
				}
				return nil, false, nil
			}
			return nil, false, err
		}
	}
}

func (tr *TOONTokenReader) copyPath() []any {
	p := make([]any, len(tr.path))
	copy(p, tr.path)
	return p
}

func (tr *TOONTokenReader) emit(pair []any) {
	tr.pending = append(tr.pending, pair)
}

func (tr *TOONTokenReader) emitLeaf(value any) {
	tr.emit([]any{tr.copyPath(), value})
}

func (tr *TOONTokenReader) emitTruncate() {
	tr.emit([]any{tr.copyPath()})
}

// emitClosing emits truncate markers for all remaining containers,
// then for the top-level key if present.
func (tr *TOONTokenReader) emitClosing() {
	for len(tr.stack) > 0 {
		tr.popContainer()
	}
	// Top-level implicit object: emit truncate for last key.
	if tr.hasTopKey && len(tr.path) > 0 {
		tr.emitTruncate()
	}
}

// popContainer pops the top container and emits its truncate marker.
func (tr *TOONTokenReader) popContainer() {
	if len(tr.stack) == 0 {
		return
	}
	top := tr.stack[len(tr.stack)-1]
	tr.stack = tr.stack[:len(tr.stack)-1]

	switch top.kind {
	case containerObject:
		// Path has the last key from this object. Emit truncate, pop key.
		tr.emitTruncate()
		if len(tr.path) > 0 {
			tr.path = tr.path[:len(tr.path)-1]
		}
	case containerListArray:
		// Path has the last array index.
		tr.emitTruncate()
		if len(tr.path) > 0 {
			tr.path = tr.path[:len(tr.path)-1]
		}
	case containerTabularArray:
		tr.emitTruncate()
		if len(tr.path) > 0 {
			tr.path = tr.path[:len(tr.path)-1]
		}
	}
}

func (tr *TOONTokenReader) nextLine() (string, bool) {
	if tr.peeked {
		tr.peeked = false
		return tr.peekLine, tr.peekOK
	}
	if tr.scanner.Scan() {
		return tr.scanner.Text(), true
	}
	return "", false
}

// advance processes the next meaningful line.
func (tr *TOONTokenReader) advance() error {
	for {
		line, ok := tr.nextLine()
		if !ok {
			if err := tr.scanner.Err(); err != nil {
				return err
			}
			return io.EOF
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		indent, content := computeIndent(line, tr.indentSize)
		if content == "" {
			continue
		}

		tr.handleDedent(indent)
		return tr.dispatchLine(indent, content)
	}
}

// dispatchLine routes a line to the appropriate handler based on the current container context.
func (tr *TOONTokenReader) dispatchLine(indent int, content string) error {
	if len(tr.stack) == 0 {
		return tr.processLine(indent, content)
	}

	top := &tr.stack[len(tr.stack)-1]
	switch top.kind {
	case containerListArray:
		return tr.dispatchListArray(indent, content, top)
	case containerTabularArray:
		return tr.dispatchTabularArray(indent, content, top)
	case containerObject:
		tr.advanceObjectSibling(indent, top)
	}
	return tr.processLine(indent, content)
}

func (tr *TOONTokenReader) dispatchListArray(indent int, content string, top *containerInfo) error {
	if indent == top.indent && strings.HasPrefix(content, "-") {
		return tr.processListItem(indent, content, len(tr.stack)-1)
	}
	tr.popContainer()
	return tr.processLine(indent, content)
}

func (tr *TOONTokenReader) dispatchTabularArray(indent int, content string, top *containerInfo) error {
	trimmed := strings.TrimSpace(content)
	if indent == top.indent && indexOutsideQuotes(trimmed, ':') == -1 {
		return tr.processTabularRow(indent, content, top)
	}
	tr.popContainer()
	return tr.processLine(indent, content)
}

func (tr *TOONTokenReader) advanceObjectSibling(indent int, top *containerInfo) {
	if indent == top.indent {
		if top.childCount > 0 && len(tr.path) > 0 {
			tr.path = tr.path[:len(tr.path)-1]
		}
		top.childCount++
	}
}

// handleDedent pops containers whose child indent > current indent.
func (tr *TOONTokenReader) handleDedent(indent int) {
	for len(tr.stack) > 0 {
		top := tr.stack[len(tr.stack)-1]
		if indent >= top.indent {
			break
		}
		tr.popContainer()
	}
}

func (tr *TOONTokenReader) processLine(indent int, content string) error {
	// Check for array header.
	header, isHeader, err := tryParseHeader(content)
	if err != nil {
		return err
	}
	if isHeader {
		return tr.processArrayHeader(indent, header)
	}
	return tr.processKV(indent, content)
}

func (tr *TOONTokenReader) processKV(indent int, content string) error {
	key, rest, err := splitKeyValue(content)
	if err != nil {
		return err
	}

	// Pop previous top-level key if at indent 0 with no containers.
	if indent == 0 && len(tr.stack) == 0 && tr.hasTopKey && len(tr.path) > 0 {
		tr.path = tr.path[:len(tr.path)-1]
	}

	tr.path = append(tr.path, key)
	if indent == 0 && len(tr.stack) == 0 {
		tr.hasTopKey = true
	}

	if rest == "" {
		tr.stack = append(tr.stack, containerInfo{
			kind:   containerObject,
			indent: indent + 1,
		})
		return nil
	}

	value, err := decodePrimitiveToken(rest)
	if err != nil {
		return err
	}
	tr.emitLeaf(value)
	return nil
}

func (tr *TOONTokenReader) processArrayHeader(indent int, header toonParsedHeader) error {
	// Pop previous top-level key if needed.
	if indent == 0 && len(tr.stack) == 0 && tr.hasTopKey && len(tr.path) > 0 {
		tr.path = tr.path[:len(tr.path)-1]
	}

	if header.key != "" {
		tr.path = append(tr.path, header.key)
		if indent == 0 && len(tr.stack) == 0 {
			tr.hasTopKey = true
		}
	}

	delimiter := header.delimiter.toRune()

	if header.inlineValues != "" {
		return tr.emitInlineArray(header.inlineValues, delimiter)
	}

	// Tabular array.
	if len(header.fields) > 0 {
		tr.path = append(tr.path, 0)
		tr.stack = append(tr.stack, containerInfo{
			kind:      containerTabularArray,
			indent:    indent + 1,
			fields:    header.fields,
			delimiter: delimiter,
			index:     0,
		})
		return nil
	}

	// List array.
	tr.path = append(tr.path, 0)
	tr.stack = append(tr.stack, containerInfo{
		kind:   containerListArray,
		indent: indent + 1,
		index:  0,
	})
	return nil
}

// emitInlineArray emits all values of an inline array like "[3]: a,b,c".
func (tr *TOONTokenReader) emitInlineArray(values string, delimiter rune) error {
	raw, err := splitInlineValues(values, delimiter)
	if err != nil {
		return err
	}
	for i, token := range raw {
		tr.path = append(tr.path, i)
		value, err := decodePrimitiveToken(token)
		if err != nil {
			return err
		}
		tr.emitLeaf(value)
		tr.path = tr.path[:len(tr.path)-1]
	}
	if len(raw) > 0 {
		tr.path = append(tr.path, len(raw)-1)
		tr.emitTruncate()
		tr.path = tr.path[:len(tr.path)-1]
	} else {
		tr.emit([]any{tr.copyPath(), []any{}})
	}
	return nil
}

func (tr *TOONTokenReader) processListItem(indent int, content string, listIdx int) error {
	top := &tr.stack[listIdx]
	itemContent := strings.TrimSpace(content[1:]) // skip '-'

	// Update array index.
	tr.path[len(tr.path)-1] = top.index

	if itemContent == "" {
		tr.emit([]any{tr.copyPath(), map[string]any{}})
		top.index++
		return nil
	}

	// Inline sub-array: - [3]: a,b,c
	if strings.HasPrefix(itemContent, "[") {
		itemHeader, ok, err := tryParseHeader(itemContent)
		if err != nil {
			return err
		}
		if ok {
			return tr.processInlineArrayInList(itemHeader, listIdx)
		}
	}

	// Object item in list: - key: value
	if isKeyValue(itemContent) {
		key, rest, err := splitKeyValue(itemContent)
		if err != nil {
			return err
		}
		tr.path = append(tr.path, key)
		if rest == "" {
			tr.stack = append(tr.stack, containerInfo{
				kind:   containerObject,
				indent: indent + 1,
			})
			// Re-fetch after append (backing array may have moved).
			tr.stack[listIdx].index++
			return nil
		}
		value, err := decodePrimitiveToken(rest)
		if err != nil {
			return err
		}
		tr.emitLeaf(value)
		tr.stack = append(tr.stack, containerInfo{
			kind:       containerObject,
			indent:     indent + 1,
			childCount: 1,
		})
		tr.stack[listIdx].index++
		return nil
	}

	// Simple scalar.
	value, err := decodePrimitiveToken(itemContent)
	if err != nil {
		return err
	}
	tr.emitLeaf(value)
	top.index++
	return nil
}

func (tr *TOONTokenReader) processInlineArrayInList(header toonParsedHeader, listIdx int) error {
	delimiter := header.delimiter.toRune()
	if header.inlineValues != "" {
		raw, err := splitInlineValues(header.inlineValues, delimiter)
		if err != nil {
			return err
		}
		for i, token := range raw {
			tr.path = append(tr.path, i)
			value, err := decodePrimitiveToken(token)
			if err != nil {
				return err
			}
			tr.emitLeaf(value)
			tr.path = tr.path[:len(tr.path)-1]
		}
		if len(raw) > 0 {
			tr.path = append(tr.path, len(raw)-1)
			tr.emitTruncate()
			tr.path = tr.path[:len(tr.path)-1]
		} else {
			tr.emit([]any{tr.copyPath(), []any{}})
		}
	}
	tr.stack[listIdx].index++
	return nil
}

func (tr *TOONTokenReader) processTabularRow(_ int, content string, top *containerInfo) error {
	trimmed := strings.TrimSpace(content)
	tr.path[len(tr.path)-1] = top.index

	raw, err := splitInlineValues(trimmed, top.delimiter)
	if err != nil {
		return err
	}

	for i, field := range top.fields {
		if i >= len(raw) {
			break
		}
		tr.path = append(tr.path, field)
		value, err := decodePrimitiveToken(raw[i])
		if err != nil {
			return err
		}
		tr.emitLeaf(value)
		tr.path = tr.path[:len(tr.path)-1]
	}

	if len(top.fields) > 0 {
		lastField := top.fields[len(top.fields)-1]
		tr.path = append(tr.path, lastField)
		tr.emitTruncate()
		tr.path = tr.path[:len(tr.path)-1]
	}

	top.index++
	return nil
}

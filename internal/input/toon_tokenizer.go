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
		v, ok, err := tr.step()
		if ok || err != nil || tr.done {
			return v, ok, err
		}
	}
}

func (tr *TOONTokenReader) step() (any, bool, error) {
	if v, ok := tr.drainPending(); ok {
		return v, true, nil
	}
	if tr.done {
		return nil, false, nil
	}
	return tr.advanceOrHandle()
}

func (tr *TOONTokenReader) advanceOrHandle() (any, bool, error) {
	if err := tr.advance(); err != nil {
		return tr.handleAdvanceErr(err)
	}
	return nil, false, nil
}

func (tr *TOONTokenReader) drainPending() (any, bool) {
	if len(tr.pending) == 0 {
		return nil, false
	}
	v := tr.pending[0]
	tr.pending = tr.pending[1:]
	return v, true
}

func (tr *TOONTokenReader) handleAdvanceErr(err error) (any, bool, error) {
	tr.done = true
	if !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	tr.emitClosing()
	if v, ok := tr.drainPending(); ok {
		return v, true, nil
	}
	return nil, false, nil
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
	tr.stack = tr.stack[:len(tr.stack)-1]
	tr.emitTruncateAndPop()
}

// emitTruncateAndPop emits a truncate marker and removes the last path element.
func (tr *TOONTokenReader) emitTruncateAndPop() {
	tr.emitTruncate()
	if len(tr.path) > 0 {
		tr.path = tr.path[:len(tr.path)-1]
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
			return tr.scannerEOF()
		}
		if err, handled := tr.processIfContent(line); handled {
			return err
		}
	}
}

func (tr *TOONTokenReader) processIfContent(line string) (error, bool) {
	indent, content, skip := tr.parseLine(line)
	if skip {
		return nil, false
	}
	tr.handleDedent(indent)
	return tr.dispatchLine(indent, content), true
}

func (tr *TOONTokenReader) scannerEOF() error {
	if err := tr.scanner.Err(); err != nil {
		return err
	}
	return io.EOF
}

func (tr *TOONTokenReader) parseLine(line string) (int, string, bool) {
	if strings.TrimSpace(line) == "" {
		return 0, "", true
	}
	indent, content := computeIndent(line, tr.indentSize)
	if content == "" {
		return 0, "", true
	}
	return indent, content, false
}

// dispatchLine routes a line to the appropriate handler based on the current container context.
func (tr *TOONTokenReader) dispatchLine(indent int, content string) error {
	if len(tr.stack) == 0 {
		return tr.processLine(indent, content)
	}
	return tr.dispatchContainer(indent, content)
}

func (tr *TOONTokenReader) dispatchContainer(indent int, content string) error {
	top := tr.topContainer()
	if top.kind == containerListArray {
		return tr.dispatchListArray(indent, content, top)
	}
	if top.kind == containerTabularArray {
		return tr.dispatchTabularArray(indent, content, top)
	}
	tr.advanceObjectSibling(indent, top)
	return tr.processLine(indent, content)
}

func (tr *TOONTokenReader) topContainer() *containerInfo {
	return &tr.stack[len(tr.stack)-1]
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
	tr.pushTopKey(indent, key)
	return tr.handleKVValue(indent, rest)
}

func (tr *TOONTokenReader) handleKVValue(indent int, rest string) error {
	if rest == "" {
		tr.pushObjectContainer(indent)
		return nil
	}
	return tr.emitPrimitive(rest)
}

// pushTopKey manages the path when entering a new key-value pair.
func (tr *TOONTokenReader) pushTopKey(indent int, key string) {
	tr.popPreviousTopKey(indent)
	tr.path = append(tr.path, key)
	if tr.isTopLevel(indent) {
		tr.hasTopKey = true
	}
}

func (tr *TOONTokenReader) isTopLevel(indent int) bool {
	return indent == 0 && len(tr.stack) == 0
}

func (tr *TOONTokenReader) pushObjectContainer(indent int) {
	tr.stack = append(tr.stack, containerInfo{
		kind:   containerObject,
		indent: indent + 1,
	})
}

func (tr *TOONTokenReader) emitPrimitive(token string) error {
	value, err := decodePrimitiveToken(token)
	if err != nil {
		return err
	}
	tr.emitLeaf(value)
	return nil
}

func (tr *TOONTokenReader) processArrayHeader(indent int, header toonParsedHeader) error {
	tr.popPreviousTopKey(indent)
	tr.pushHeaderKey(indent, header.key)

	delimiter := header.delimiter.toRune()
	if header.inlineValues != "" {
		return tr.emitInlineArray(header.inlineValues, delimiter)
	}
	tr.pushArrayContainer(indent, header, delimiter)
	return nil
}

func (tr *TOONTokenReader) popPreviousTopKey(indent int) {
	if tr.isTopLevel(indent) && tr.hasTopKey && len(tr.path) > 0 {
		tr.path = tr.path[:len(tr.path)-1]
	}
}

func (tr *TOONTokenReader) pushHeaderKey(indent int, key string) {
	if key == "" {
		return
	}
	tr.path = append(tr.path, key)
	if tr.isTopLevel(indent) {
		tr.hasTopKey = true
	}
}

func (tr *TOONTokenReader) pushArrayContainer(indent int, header toonParsedHeader, delimiter rune) {
	tr.path = append(tr.path, 0)
	if len(header.fields) > 0 {
		tr.pushTabularContainer(indent, header.fields, delimiter)
	} else {
		tr.pushListContainer(indent)
	}
}

func (tr *TOONTokenReader) pushTabularContainer(indent int, fields []string, delimiter rune) {
	tr.stack = append(tr.stack, containerInfo{
		kind:      containerTabularArray,
		indent:    indent + 1,
		fields:    fields,
		delimiter: delimiter,
		index:     0,
	})
}

func (tr *TOONTokenReader) pushListContainer(indent int) {
	tr.stack = append(tr.stack, containerInfo{
		kind:   containerListArray,
		indent: indent + 1,
		index:  0,
	})
}

// emitInlineArray emits all values of an inline array like "[3]: a,b,c".
func (tr *TOONTokenReader) emitInlineArray(values string, delimiter rune) error {
	raw, err := splitInlineValues(values, delimiter)
	if err != nil {
		return err
	}
	if err := tr.emitIndexedValues(raw); err != nil {
		return err
	}
	tr.emitArrayClose(len(raw))
	return nil
}

func (tr *TOONTokenReader) emitIndexedValues(raw []string) error {
	for i, token := range raw {
		tr.path = append(tr.path, i)
		if err := tr.emitPrimitive(token); err != nil {
			return err
		}
		tr.path = tr.path[:len(tr.path)-1]
	}
	return nil
}

func (tr *TOONTokenReader) emitArrayClose(count int) {
	if count > 0 {
		tr.path = append(tr.path, count-1)
		tr.emitTruncate()
		tr.path = tr.path[:len(tr.path)-1]
	} else {
		tr.emit([]any{tr.copyPath(), []any{}})
	}
}

func (tr *TOONTokenReader) processListItem(indent int, content string, listIdx int) error {
	top := &tr.stack[listIdx]
	itemContent := strings.TrimSpace(content[1:]) // skip '-'
	tr.path[len(tr.path)-1] = top.index

	if itemContent == "" {
		return tr.emitEmptyListItem(top)
	}
	return tr.processListContent(indent, itemContent, listIdx, top)
}

func (tr *TOONTokenReader) emitEmptyListItem(top *containerInfo) error {
	tr.emit([]any{tr.copyPath(), map[string]any{}})
	top.index++
	return nil
}

func (tr *TOONTokenReader) processListContent(indent int, content string, listIdx int, top *containerInfo) error {
	if handled, err := tr.tryInlineSubArray(content, listIdx); handled || err != nil {
		return err
	}
	if isKeyValue(content) {
		return tr.processListObjectItem(indent, content, listIdx)
	}
	return tr.processListScalar(content, top)
}

func (tr *TOONTokenReader) tryInlineSubArray(content string, listIdx int) (bool, error) {
	if !strings.HasPrefix(content, "[") {
		return false, nil
	}
	return tr.parseAndEmitInlineSubArray(content, listIdx)
}

func (tr *TOONTokenReader) parseAndEmitInlineSubArray(content string, listIdx int) (bool, error) {
	header, ok, err := tryParseHeader(content)
	if err != nil || !ok {
		return false, err
	}
	return true, tr.processInlineArrayInList(header, listIdx)
}

func (tr *TOONTokenReader) processListObjectItem(indent int, content string, listIdx int) error {
	count, err := tr.emitListObjectKV(content)
	if err != nil {
		return err
	}
	tr.pushObjectContainerWithCount(indent, count)
	tr.stack[listIdx].index++
	return nil
}

func (tr *TOONTokenReader) emitListObjectKV(content string) (int, error) {
	key, rest, err := splitKeyValue(content)
	if err != nil {
		return 0, err
	}
	tr.path = append(tr.path, key)
	return tr.emitOptionalValue(rest)
}

func (tr *TOONTokenReader) emitOptionalValue(rest string) (int, error) {
	if rest == "" {
		return 0, nil
	}
	return 1, tr.emitPrimitive(rest)
}

func (tr *TOONTokenReader) pushObjectContainerWithCount(indent int, childCount int) {
	tr.stack = append(tr.stack, containerInfo{
		kind:       containerObject,
		indent:     indent + 1,
		childCount: childCount,
	})
}

func (tr *TOONTokenReader) processListScalar(content string, top *containerInfo) error {
	if err := tr.emitPrimitive(content); err != nil {
		return err
	}
	top.index++
	return nil
}

func (tr *TOONTokenReader) processInlineArrayInList(header toonParsedHeader, listIdx int) error {
	if header.inlineValues != "" {
		if err := tr.emitInlineArray(header.inlineValues, header.delimiter.toRune()); err != nil {
			return err
		}
	}
	tr.stack[listIdx].index++
	return nil
}

func (tr *TOONTokenReader) processTabularRow(_ int, content string, top *containerInfo) error {
	tr.path[len(tr.path)-1] = top.index
	raw, err := splitInlineValues(strings.TrimSpace(content), top.delimiter)
	if err != nil {
		return err
	}
	return tr.emitTabularFields(top, raw)
}

func (tr *TOONTokenReader) emitTabularFields(top *containerInfo, raw []string) error {
	if err := tr.emitFieldValues(top.fields, raw); err != nil {
		return err
	}
	tr.emitTabularRowClose(top.fields)
	top.index++
	return nil
}

func (tr *TOONTokenReader) emitFieldValues(fields, raw []string) error {
	for i, field := range fields {
		if i >= len(raw) {
			break
		}
		if err := tr.emitNamedField(field, raw[i]); err != nil {
			return err
		}
	}
	return nil
}

func (tr *TOONTokenReader) emitNamedField(name, token string) error {
	tr.path = append(tr.path, name)
	err := tr.emitPrimitive(token)
	tr.path = tr.path[:len(tr.path)-1]
	return err
}

func (tr *TOONTokenReader) emitTabularRowClose(fields []string) {
	if len(fields) == 0 {
		return
	}
	lastField := fields[len(fields)-1]
	tr.path = append(tr.path, lastField)
	tr.emitTruncate()
	tr.path = tr.path[:len(tr.path)-1]
}

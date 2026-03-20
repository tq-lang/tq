package input

import (
	"encoding/json"
	"errors"
	"io"
)

// TokenReader emits [path, value] streaming pairs from JSON using
// json.Decoder.Token(). Memory usage is O(depth), not O(document_size).
//
// Output format matches gojq/jq tostream:
//   - Leaf values:      []any{path, value}
//   - Container end:    []any{path}          (truncate marker)
//   - Empty container:  []any{path, empty}   (where empty is []any{} or map[string]any{})
//
// Ported from github.com/itchyny/gojq/cli/stream.go.
type TokenReader struct {
	dec    *json.Decoder
	path   []any
	states []int
	done   bool
}

const (
	stateTopValue = iota
	stateArrayStart
	stateArrayValue
	stateArrayEnd
	stateArrayEmptyEnd
	stateObjectStart
	stateObjectKey
	stateObjectValue
	stateObjectEnd
	stateObjectEmptyEnd
)

// NewTokenReader creates a TokenReader that streams path-value pairs from r.
func NewTokenReader(r io.Reader) *TokenReader {
	return &TokenReader{
		dec:    json.NewDecoder(r),
		states: []int{stateTopValue},
		path:   []any{},
	}
}

// Next returns the next streaming pair from the JSON input.
// Returns (value, true, nil) for each pair, (nil, false, nil) at EOF,
// or (nil, false, err) on parse error.
func (tr *TokenReader) Next() (any, bool, error) {
	if tr.done {
		return nil, false, nil
	}
	return tr.nextOrFinish()
}

func (tr *TokenReader) nextOrFinish() (any, bool, error) {
	v, err := tr.next()
	if err != nil {
		return tr.handleNextErr(err)
	}
	return v, true, nil
}

func (tr *TokenReader) handleNextErr(err error) (any, bool, error) {
	tr.done = true
	if errors.Is(err, io.EOF) {
		return nil, false, nil
	}
	return nil, false, err
}

func (tr *TokenReader) next() (any, error) {
	tr.cleanupEndStates()
	tr.advanceSiblingPath()
	return tr.readToken()
}

func (tr *TokenReader) readToken() (any, error) {
	for {
		token, err := tr.dec.Token()
		if err != nil {
			return nil, tr.wrapEOF(err)
		}
		if v, done := tr.dispatchToken(token); done {
			return v, nil
		}
	}
}

func (tr *TokenReader) wrapEOF(err error) error {
	if errors.Is(err, io.EOF) && tr.currentState() != stateTopValue {
		return io.ErrUnexpectedEOF
	}
	return err
}

func (tr *TokenReader) currentState() int {
	return tr.states[len(tr.states)-1]
}

func (tr *TokenReader) dispatchToken(token json.Token) (any, bool) {
	if d, ok := token.(json.Delim); ok {
		return tr.handleDelim(d)
	}
	return tr.handleValue(token)
}

// cleanupEndStates pops completed container states.
func (tr *TokenReader) cleanupEndStates() {
	switch tr.currentState() {
	case stateArrayEnd, stateObjectEnd:
		tr.path = tr.path[:len(tr.path)-1]
		tr.states = tr.states[:len(tr.states)-1]
	case stateArrayEmptyEnd, stateObjectEmptyEnd:
		tr.states = tr.states[:len(tr.states)-1]
	}
}

// advanceSiblingPath updates the path index between sibling values.
func (tr *TokenReader) advanceSiblingPath() {
	if !tr.dec.More() {
		return
	}
	tr.advanceSiblingByState()
}

func (tr *TokenReader) advanceSiblingByState() {
	switch tr.currentState() {
	case stateArrayValue:
		tr.incrementArrayIndex()
	case stateObjectValue:
		tr.path = tr.path[:len(tr.path)-1]
	}
}

func (tr *TokenReader) incrementArrayIndex() {
	idx, _ := tr.path[len(tr.path)-1].(int)
	tr.path[len(tr.path)-1] = idx + 1
}

// handleDelim processes a JSON delimiter token ([, ], {, }).
// Returns the emitted value and true if a value should be returned to the caller.
func (tr *TokenReader) handleDelim(d json.Delim) (any, bool) {
	switch d {
	case '[', '{':
		tr.handleOpen(d)
	case ']':
		return tr.handleCloseArray()
	case '}':
		return tr.handleCloseObject()
	}
	return nil, false
}

func (tr *TokenReader) handleOpen(d json.Delim) {
	tr.promoteContainerState()
	if d == '[' {
		tr.states = append(tr.states, stateArrayStart)
		tr.path = append(tr.path, 0)
	} else {
		tr.states = append(tr.states, stateObjectStart)
	}
}

func (tr *TokenReader) promoteContainerState() {
	switch tr.currentState() {
	case stateArrayStart:
		tr.setState(stateArrayValue)
	case stateObjectKey:
		tr.setState(stateObjectValue)
	}
}

func (tr *TokenReader) setState(s int) {
	tr.states[len(tr.states)-1] = s
}

func (tr *TokenReader) handleCloseArray() (any, bool) {
	if tr.currentState() == stateArrayStart {
		tr.setState(stateArrayEmptyEnd)
		tr.path = tr.path[:len(tr.path)-1]
		return []any{tr.copyPath(), []any{}}, true
	}
	tr.setState(stateArrayEnd)
	return []any{tr.copyPath()}, true
}

func (tr *TokenReader) handleCloseObject() (any, bool) {
	if tr.currentState() == stateObjectStart {
		tr.setState(stateObjectEmptyEnd)
		return []any{tr.copyPath(), map[string]any{}}, true
	}
	tr.setState(stateObjectEnd)
	return []any{tr.copyPath()}, true
}

// handleValue processes a non-delimiter token (string, number, bool, null).
// Returns the emitted value and true if a value should be returned to the caller.
func (tr *TokenReader) handleValue(token json.Token) (any, bool) {
	if tr.isArrayContext() {
		return tr.handleArrayValue(token)
	}
	return tr.handleNonArrayValue(token)
}

func (tr *TokenReader) isArrayContext() bool {
	s := tr.currentState()
	return s == stateArrayStart || s == stateArrayValue
}

func (tr *TokenReader) handleArrayValue(token json.Token) (any, bool) {
	if tr.currentState() == stateArrayStart {
		tr.setState(stateArrayValue)
	}
	return tr.emitPair(token), true
}

func (tr *TokenReader) handleNonArrayValue(token json.Token) (any, bool) {
	s := tr.currentState()
	if s == stateObjectStart || s == stateObjectValue {
		tr.pushObjectKey(token)
		return nil, false
	}
	return tr.emitLeafValue(token, s)
}

func (tr *TokenReader) emitLeafValue(token json.Token, s int) (any, bool) {
	if s == stateObjectKey {
		tr.setState(stateObjectValue)
	} else {
		tr.setState(stateTopValue)
	}
	return tr.emitPair(token), true
}

func (tr *TokenReader) pushObjectKey(token json.Token) {
	tr.setState(stateObjectKey)
	tr.path = append(tr.path, token)
}

func (tr *TokenReader) emitPair(token json.Token) any {
	return []any{tr.copyPath(), token}
}

func (tr *TokenReader) copyPath() []any {
	path := make([]any, len(tr.path))
	copy(path, tr.path)
	return path
}

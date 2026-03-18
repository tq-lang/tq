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
	v, err := tr.next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			tr.done = true
			return nil, false, nil
		}
		tr.done = true
		return nil, false, err
	}
	return v, true, nil
}

func (tr *TokenReader) next() (any, error) {
	tr.cleanupEndStates()
	tr.advanceSiblingPath()

	for {
		token, err := tr.dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) && tr.states[len(tr.states)-1] != stateTopValue {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}

		if d, ok := token.(json.Delim); ok {
			if v, done := tr.handleDelim(d); done {
				return v, nil
			}
		} else {
			if v, done := tr.handleValue(token); done {
				return v, nil
			}
		}
	}
}

// cleanupEndStates pops completed container states.
func (tr *TokenReader) cleanupEndStates() {
	switch tr.states[len(tr.states)-1] {
	case stateArrayEnd, stateObjectEnd:
		tr.path = tr.path[:len(tr.path)-1]
		fallthrough
	case stateArrayEmptyEnd, stateObjectEmptyEnd:
		tr.states = tr.states[:len(tr.states)-1]
	}
}

// advanceSiblingPath updates the path index between sibling values.
func (tr *TokenReader) advanceSiblingPath() {
	if tr.dec.More() {
		switch tr.states[len(tr.states)-1] {
		case stateArrayValue:
			idx, _ := tr.path[len(tr.path)-1].(int)
			tr.path[len(tr.path)-1] = idx + 1
		case stateObjectValue:
			tr.path = tr.path[:len(tr.path)-1]
		}
	}
}

// handleDelim processes a JSON delimiter token ([, ], {, }).
// Returns the emitted value and true if a value should be returned to the caller.
func (tr *TokenReader) handleDelim(d json.Delim) (any, bool) {
	switch d {
	case '[', '{':
		switch tr.states[len(tr.states)-1] {
		case stateArrayStart:
			tr.states[len(tr.states)-1] = stateArrayValue
		case stateObjectKey:
			tr.states[len(tr.states)-1] = stateObjectValue
		}
		if d == '[' {
			tr.states = append(tr.states, stateArrayStart)
			tr.path = append(tr.path, 0)
		} else {
			tr.states = append(tr.states, stateObjectStart)
		}
	case ']':
		if tr.states[len(tr.states)-1] == stateArrayStart {
			tr.states[len(tr.states)-1] = stateArrayEmptyEnd
			tr.path = tr.path[:len(tr.path)-1]
			return []any{tr.copyPath(), []any{}}, true
		}
		tr.states[len(tr.states)-1] = stateArrayEnd
		return []any{tr.copyPath()}, true
	case '}':
		if tr.states[len(tr.states)-1] == stateObjectStart {
			tr.states[len(tr.states)-1] = stateObjectEmptyEnd
			return []any{tr.copyPath(), map[string]any{}}, true
		}
		tr.states[len(tr.states)-1] = stateObjectEnd
		return []any{tr.copyPath()}, true
	}
	return nil, false
}

// handleValue processes a non-delimiter token (string, number, bool, null).
// Returns the emitted value and true if a value should be returned to the caller.
func (tr *TokenReader) handleValue(token json.Token) (any, bool) {
	switch tr.states[len(tr.states)-1] {
	case stateArrayStart:
		tr.states[len(tr.states)-1] = stateArrayValue
		fallthrough
	case stateArrayValue:
		return []any{tr.copyPath(), token}, true
	case stateObjectStart, stateObjectValue:
		tr.states[len(tr.states)-1] = stateObjectKey
		tr.path = append(tr.path, token)
	case stateObjectKey:
		tr.states[len(tr.states)-1] = stateObjectValue
		return []any{tr.copyPath(), token}, true
	default:
		tr.states[len(tr.states)-1] = stateTopValue
		return []any{tr.copyPath(), token}, true
	}
	return nil, false
}

func (tr *TokenReader) copyPath() []any {
	path := make([]any, len(tr.path))
	copy(path, tr.path)
	return path
}

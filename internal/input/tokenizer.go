package input

import (
	"encoding/json"
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
		if err == io.EOF {
			tr.done = true
			return nil, false, nil
		}
		tr.done = true
		return nil, false, err
	}
	return v, true, nil
}

func (tr *TokenReader) next() (any, error) {
	// Clean up after container end states.
	switch tr.states[len(tr.states)-1] {
	case stateArrayEnd, stateObjectEnd:
		tr.path = tr.path[:len(tr.path)-1]
		fallthrough
	case stateArrayEmptyEnd, stateObjectEmptyEnd:
		tr.states = tr.states[:len(tr.states)-1]
	}

	// Advance path indices between sibling values.
	if tr.dec.More() {
		switch tr.states[len(tr.states)-1] {
		case stateArrayValue:
			tr.path[len(tr.path)-1] = tr.path[len(tr.path)-1].(int) + 1
		case stateObjectValue:
			tr.path = tr.path[:len(tr.path)-1]
		}
	}

	for {
		token, err := tr.dec.Token()
		if err != nil {
			if err == io.EOF && tr.states[len(tr.states)-1] != stateTopValue {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}

		if d, ok := token.(json.Delim); ok {
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
					return []any{tr.copyPath(), []any{}}, nil
				}
				tr.states[len(tr.states)-1] = stateArrayEnd
				return []any{tr.copyPath()}, nil
			case '}':
				if tr.states[len(tr.states)-1] == stateObjectStart {
					tr.states[len(tr.states)-1] = stateObjectEmptyEnd
					return []any{tr.copyPath(), map[string]any{}}, nil
				}
				tr.states[len(tr.states)-1] = stateObjectEnd
				return []any{tr.copyPath()}, nil
			}
		} else {
			switch tr.states[len(tr.states)-1] {
			case stateArrayStart:
				tr.states[len(tr.states)-1] = stateArrayValue
				fallthrough
			case stateArrayValue:
				return []any{tr.copyPath(), token}, nil
			case stateObjectStart, stateObjectValue:
				tr.states[len(tr.states)-1] = stateObjectKey
				tr.path = append(tr.path, token)
			case stateObjectKey:
				tr.states[len(tr.states)-1] = stateObjectValue
				return []any{tr.copyPath(), token}, nil
			default:
				tr.states[len(tr.states)-1] = stateTopValue
				return []any{tr.copyPath(), token}, nil
			}
		}
	}
}

func (tr *TokenReader) copyPath() []any {
	path := make([]any, len(tr.path))
	copy(path, tr.path)
	return path
}

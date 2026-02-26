package input

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  any
	}{
		{"json object", []byte(`{"name":"Alice"}`), map[string]any{"name": "Alice"}},
		{"json array", []byte(`[1,2,3]`), []any{1.0, 2.0, 3.0}},
		{"json string", []byte(`"hello"`), "hello"},
		{"json number", []byte(`42`), 42.0},
		{"json boolean", []byte(`true`), true},
		{"json null", []byte(`null`), nil},
		{"toon key-value", []byte("name: Alice\nage: 30"), map[string]any{"name": "Alice", "age": 30.0}},
		{"invalid json falls back to toon", []byte(`{not json}`), "{not json}"},
		{"empty input", []byte(``), map[string]any{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

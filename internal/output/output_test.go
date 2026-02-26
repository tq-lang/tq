package output

import (
	"bytes"
	"testing"

	"github.com/toon-format/toon-go"
)

func TestWrite(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		opts    Options
		want    string
		wantErr bool
	}{
		// Raw output
		{"raw string", "hello", Options{Raw: true}, "hello\n", false},
		{"raw string join", "hello", Options{Raw: true, Join: true}, "hello", false},
		{"raw non-string falls through", 42.0, Options{Raw: true, JSON: true, Compact: true}, "42\n", false},

		// JSON compact
		{"json compact", map[string]any{"a": 1.0}, Options{JSON: true, Compact: true}, "{\"a\":1}\n", false},
		{"json compact join", map[string]any{"a": 1.0}, Options{JSON: true, Compact: true, Join: true}, "{\"a\":1}", false},

		// JSON indented
		{"json default indent", map[string]any{"b": 2.0}, Options{JSON: true}, "{\n  \"b\": 2\n}\n", false},
		{"json tab indent", map[string]any{"c": 3.0}, Options{JSON: true, Tab: true}, "{\n\t\"c\": 3\n}\n", false},
		{"json custom indent", map[string]any{"d": 4.0}, Options{JSON: true, Indent: 4}, "{\n    \"d\": 4\n}\n", false},
		{"json indented join", map[string]any{"e": 5.0}, Options{JSON: true, Join: true}, "{\n  \"e\": 5\n}", false},

		// JSON primitives
		{"json null", nil, Options{JSON: true, Compact: true}, "null\n", false},
		{"json string", "text", Options{JSON: true, Compact: true}, "\"text\"\n", false},
		{"json number", 3.14, Options{JSON: true, Compact: true}, "3.14\n", false},
		{"json boolean", true, Options{JSON: true, Compact: true}, "true\n", false},
		{"json array", []any{1.0, 2.0}, Options{JSON: true, Compact: true}, "[1,2]\n", false},

		// TOON output
		{"toon default", map[string]any{"x": "y"}, Options{}, "x: y\n", false},
		{"toon pipe delimiter", []any{1.0, 2.0}, Options{Delimiter: toon.DelimiterPipe}, "[2|]: 1|2\n", false},
		{"toon tab delimiter", []any{"a", "b"}, Options{Delimiter: toon.DelimiterTab}, "[2\t]: a\tb\n", false},
		{"toon custom indent", map[string]any{"x": "y"}, Options{Indent: 4}, "x: y\n", false},
		{"toon tab indent", map[string]any{"x": "y"}, Options{Tab: true}, "x: y\n", false},
		{"toon join", "hello", Options{Join: true}, "hello", false},
		{"toon null", nil, Options{}, "null\n", false},

		// Errors
		{"json marshal error", make(chan int), Options{JSON: true, Compact: true}, "", true},
		{"json indent marshal error", make(chan int), Options{JSON: true}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := Write(&buf, tt.value, tt.opts)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

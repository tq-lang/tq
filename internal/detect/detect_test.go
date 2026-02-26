package detect

import (
	"bufio"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		expect Format
	}{
		// JSON starters
		{"json object", []byte(`{"key": "value"}`), JSON},
		{"json array", []byte(`[1, 2, 3]`), JSON},
		{"json string", []byte(`"hello"`), JSON},
		{"json true", []byte(`true`), JSON},
		{"json false", []byte(`false`), JSON},
		{"json null", []byte(`null`), JSON},
		{"json negative number", []byte(`-42`), JSON},
		{"json zero", []byte(`0`), JSON},
		{"json digit 9", []byte(`9`), JSON},

		// TOON — must start with char NOT in JSON starters ({["tfn-0-9)
		{"toon uppercase key", []byte(`Name: Alice`), TOON},
		{"toon lowercase key starting with k", []byte(`key: value`), TOON},
		{"toon starting with hash comment", []byte(`# comment`), TOON},

		// Note: 'n' is a JSON starter (for null), so 'name: ...' is detected as JSON.
		// This is expected — input.Reader handles the fallback from JSON to TOON.
		{"n-starting detected as json", []byte(`name: Alice`), JSON},

		// Whitespace handling
		{"leading spaces then json", []byte(`   {"key": 1}`), JSON},
		{"leading tabs then json", []byte("\t\t[1]"), JSON},
		{"leading newlines then toon key", []byte("\n\nKey: Alice"), TOON},
		{"leading mixed whitespace then json", []byte(" \t\n\r{\"a\":1}"), JSON},

		// Edge cases
		{"empty input", []byte(``), TOON},
		{"whitespace only", []byte("   \t\n\r  "), TOON},
		{"nil input", nil, TOON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Detect(tt.input)
			if got != tt.expect {
				t.Errorf("Detect(%q) = %d, want %d", tt.input, got, tt.expect)
			}
		})
	}
}

func TestDetectReader(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect Format
	}{
		{"json object", `{"a":1}`, JSON},
		{"json array", `[1,2]`, JSON},
		{"json string", `"hello"`, JSON},
		{"json true", `true`, JSON},
		{"json false", `false`, JSON},
		{"json null", `null`, JSON},
		{"json number", `42`, JSON},
		{"toon key", `key: value`, TOON},
		{"toon uppercase", `Name: Alice`, TOON},
		// Same as Detect: 'n' triggers JSON detection; StreamReader handles fallback.
		{"n-starting detected as json", `name: Alice`, JSON},
		{"empty", ``, TOON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bufio.NewReader(strings.NewReader(tt.input))
			got := DetectReader(r)
			if got != tt.expect {
				t.Errorf("DetectReader(%q) = %d, want %d", tt.input, got, tt.expect)
			}
		})
	}
}

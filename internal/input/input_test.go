package input

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
		wantKey string // if set, assert result is map with this key
		wantVal any    // expected value at wantKey
	}{
		{"json object", []byte(`{"name":"Alice"}`), false, "name", "Alice"},
		{"json array", []byte(`[1,2,3]`), false, "", nil},
		{"json string", []byte(`"hello"`), false, "", nil},
		{"json number", []byte(`42`), false, "", nil},
		{"json boolean", []byte(`true`), false, "", nil},
		{"json null", []byte(`null`), false, "", nil},
		{"toon key-value", []byte("name: Alice\nage: 30"), false, "name", "Alice"},
		{"invalid json falls back to toon", []byte(`{not json}`), false, "", nil},
		{"empty input", []byte(``), false, "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantKey != "" {
				m, ok := v.(map[string]any)
				if !ok {
					t.Fatalf("expected map, got %T", v)
				}
				if m[tt.wantKey] != tt.wantVal {
					t.Errorf("%s = %v, want %v", tt.wantKey, m[tt.wantKey], tt.wantVal)
				}
			}
		})
	}
}

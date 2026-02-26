package input

import (
	"strings"
	"testing"
)

func TestReaderJSON_SingleDoc(t *testing.T) {
	sr := NewReader(strings.NewReader(`{"name":"Alice","age":30}`))

	v, ok, err := sr.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected a value")
	}
	m := v.(map[string]any)
	if m["name"] != "Alice" {
		t.Errorf("name = %v, want Alice", m["name"])
	}

	_, ok, err = sr.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected no more values")
	}
}

func TestReaderJSON_MultiDoc(t *testing.T) {
	input := "{\"a\":1}\n{\"b\":2}\n{\"c\":3}"
	sr := NewReader(strings.NewReader(input))

	var results []map[string]any
	for {
		v, ok, err := sr.Next()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			break
		}
		results = append(results, v.(map[string]any))
	}

	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	if results[0]["a"] != 1.0 {
		t.Errorf("results[0][a] = %v, want 1", results[0]["a"])
	}
	if results[1]["b"] != 2.0 {
		t.Errorf("results[1][b] = %v, want 2", results[1]["b"])
	}
	if results[2]["c"] != 3.0 {
		t.Errorf("results[2][c] = %v, want 3", results[2]["c"])
	}
}

func TestReaderJSON_Concatenated(t *testing.T) {
	sr := NewReader(strings.NewReader(`{"a":1}{"b":2}`))

	v1, ok, err := sr.Next()
	if err != nil || !ok {
		t.Fatalf("first Next: ok=%v, err=%v", ok, err)
	}
	if v1.(map[string]any)["a"] != 1.0 {
		t.Errorf("first value wrong: %v", v1)
	}

	v2, ok, err := sr.Next()
	if err != nil || !ok {
		t.Fatalf("second Next: ok=%v, err=%v", ok, err)
	}
	if v2.(map[string]any)["b"] != 2.0 {
		t.Errorf("second value wrong: %v", v2)
	}

	_, ok, _ = sr.Next()
	if ok {
		t.Fatal("expected no more values")
	}
}

func TestReaderTOON_SingleDoc(t *testing.T) {
	sr := NewReader(strings.NewReader("key: value\ncount: 42\n"))

	v, ok, err := sr.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected a value")
	}
	m := v.(map[string]any)
	if m["key"] != "value" {
		t.Errorf("key = %v, want value", m["key"])
	}

	_, ok, _ = sr.Next()
	if ok {
		t.Fatal("expected no more values")
	}
}

func TestReaderTOON_MultiDoc(t *testing.T) {
	input := "key: Alice\nage: 30\n\nkey: Bob\nage: 25\n"
	sr := NewReader(strings.NewReader(input))

	v1, ok, err := sr.Next()
	if err != nil || !ok {
		t.Fatalf("first Next: ok=%v, err=%v", ok, err)
	}
	if v1.(map[string]any)["key"] != "Alice" {
		t.Errorf("first doc key = %v, want Alice", v1.(map[string]any)["key"])
	}

	v2, ok, err := sr.Next()
	if err != nil || !ok {
		t.Fatalf("second Next: ok=%v, err=%v", ok, err)
	}
	if v2.(map[string]any)["key"] != "Bob" {
		t.Errorf("second doc key = %v, want Bob", v2.(map[string]any)["key"])
	}

	_, ok, _ = sr.Next()
	if ok {
		t.Fatal("expected no more values")
	}
}

func TestReaderJSON_Keywords(t *testing.T) {
	sr := NewReader(strings.NewReader("true\nfalse\nnull\n"))

	var vals []any
	for {
		v, ok, err := sr.Next()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			break
		}
		vals = append(vals, v)
	}

	if len(vals) != 3 {
		t.Fatalf("got %d values, want 3", len(vals))
	}
	if vals[0] != true {
		t.Errorf("vals[0] = %v, want true", vals[0])
	}
	if vals[1] != false {
		t.Errorf("vals[1] = %v, want false", vals[1])
	}
	if vals[2] != nil {
		t.Errorf("vals[2] = %v, want nil", vals[2])
	}
}

func TestReaderEmpty(t *testing.T) {
	sr := NewReader(strings.NewReader(""))

	_, ok, err := sr.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected no values from empty input")
	}
}

func TestReaderJSON_Fallback_TOON(t *testing.T) {
	// Input starts with 'n' which Detect classifies as JSON (for "null").
	// Reader should fall back to TOON when json.Decode fails.
	sr := NewReader(strings.NewReader("name: Alice\nage: 30\n"))

	v, ok, err := sr.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected a value")
	}
	m := v.(map[string]any)
	if m["name"] != "Alice" {
		t.Errorf("name = %v, want Alice", m["name"])
	}
}

func TestReaderJSON_Fallback_TOON_MultiDoc(t *testing.T) {
	// Multi-doc TOON starting with 'n' — tests fallback + continued streaming.
	input := "name: Alice\nage: 30\n\nname: Bob\nage: 25\n"
	sr := NewReader(strings.NewReader(input))

	v1, ok, err := sr.Next()
	if err != nil || !ok {
		t.Fatalf("first Next: ok=%v, err=%v", ok, err)
	}
	if v1.(map[string]any)["name"] != "Alice" {
		t.Errorf("first doc name = %v, want Alice", v1.(map[string]any)["name"])
	}

	v2, ok, err := sr.Next()
	if err != nil || !ok {
		t.Fatalf("second Next: ok=%v, err=%v", ok, err)
	}
	if v2.(map[string]any)["name"] != "Bob" {
		t.Errorf("second doc name = %v, want Bob", v2.(map[string]any)["name"])
	}

	_, ok, _ = sr.Next()
	if ok {
		t.Fatal("expected no more values")
	}
}

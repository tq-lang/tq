package input

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
)

func collectTokens(t *testing.T, input string) []any {
	t.Helper()
	tr := NewTokenReader(strings.NewReader(input))
	var results []any
	for {
		v, ok, err := tr.Next()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			break
		}
		results = append(results, v)
	}
	return results
}

func pairStr(v any) string {
	pair := v.([]any)
	if len(pair) == 2 {
		return fmt.Sprintf("[%v,%v]", fmtPath(pair[0]), fmtVal(pair[1]))
	}
	return fmt.Sprintf("[%v]", fmtPath(pair[0]))
}

func fmtPath(v any) string {
	path := v.([]any)
	parts := make([]string, len(path))
	for i, p := range path {
		if s, ok := p.(string); ok {
			parts[i] = fmt.Sprintf("%q", s)
		} else {
			parts[i] = fmt.Sprintf("%v", p)
		}
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func fmtVal(v any) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case []any:
		return "[]"
	case map[string]any:
		return "{}"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func TestTokenReader_SimpleObject(t *testing.T) {
	results := collectTokens(t, `{"a":1}`)

	// Expected: [["a"],1] [["a"]]
	want := []string{
		`[["a"],1]`,
		`[["a"]]`,
	}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d: %v", len(results), len(want), results)
	}
	for i, w := range want {
		got := pairStr(results[i])
		if got != w {
			t.Errorf("result[%d] = %s, want %s", i, got, w)
		}
	}
}

func TestTokenReader_SimpleArray(t *testing.T) {
	results := collectTokens(t, `[1,2]`)

	want := []string{
		`[[0],1]`,
		`[[1],2]`,
		`[[1]]`,
	}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}
	for i, w := range want {
		got := pairStr(results[i])
		if got != w {
			t.Errorf("result[%d] = %s, want %s", i, got, w)
		}
	}
}

func TestTokenReader_Nested(t *testing.T) {
	results := collectTokens(t, `{"a":[1,2],"b":3}`)

	want := []string{
		`[["a",0],1]`,
		`[["a",1],2]`,
		`[["a",1]]`,
		`[["b"],3]`,
		`[["b"]]`,
	}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}
	for i, w := range want {
		got := pairStr(results[i])
		if got != w {
			t.Errorf("result[%d] = %s, want %s", i, got, w)
		}
	}
}

func TestTokenReader_EmptyObject(t *testing.T) {
	results := collectTokens(t, `{}`)

	want := []string{`[[],{}]`}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}
	got := pairStr(results[0])
	if got != want[0] {
		t.Errorf("result = %s, want %s", got, want[0])
	}
}

func TestTokenReader_EmptyArray(t *testing.T) {
	results := collectTokens(t, `[]`)

	want := []string{`[[],[]]`}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}
	got := pairStr(results[0])
	if got != want[0] {
		t.Errorf("result = %s, want %s", got, want[0])
	}
}

func TestTokenReader_NestedEmptyObject(t *testing.T) {
	results := collectTokens(t, `{"a":{}}`)

	want := []string{
		`[["a"],{}]`,
		`[["a"]]`,
	}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}
	for i, w := range want {
		got := pairStr(results[i])
		if got != w {
			t.Errorf("result[%d] = %s, want %s", i, got, w)
		}
	}
}

func TestTokenReader_ScalarTopLevel(t *testing.T) {
	results := collectTokens(t, `42`)

	want := []string{`[[],42]`}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}
	got := pairStr(results[0])
	if got != want[0] {
		t.Errorf("result = %s, want %s", got, want[0])
	}
}

func TestTokenReader_MultiDoc(t *testing.T) {
	results := collectTokens(t, `{"a":1}{"b":2}`)

	want := []string{
		`[["a"],1]`,
		`[["a"]]`,
		`[["b"],2]`,
		`[["b"]]`,
	}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}
	for i, w := range want {
		got := pairStr(results[i])
		if got != w {
			t.Errorf("result[%d] = %s, want %s", i, got, w)
		}
	}
}

func TestTokenReader_StringValues(t *testing.T) {
	results := collectTokens(t, `{"name":"Alice"}`)

	want := []string{
		`[["name"],"Alice"]`,
		`[["name"]]`,
	}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}
	for i, w := range want {
		got := pairStr(results[i])
		if got != w {
			t.Errorf("result[%d] = %s, want %s", i, got, w)
		}
	}
}

func TestTokenReader_BoolNull(t *testing.T) {
	results := collectTokens(t, `{"t":true,"f":false,"n":null}`)

	want := []string{
		`[["t"],true]`,
		`[["f"],false]`,
		`[["n"],<nil>]`,
		`[["n"]]`,
	}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}
	for i, w := range want {
		got := pairStr(results[i])
		if got != w {
			t.Errorf("result[%d] = %s, want %s", i, got, w)
		}
	}
}

func TestTokenReader_Empty(t *testing.T) {
	results := collectTokens(t, ``)
	if len(results) != 0 {
		t.Fatalf("expected no results from empty input, got %d", len(results))
	}
}

func TestTokenReader_DeeplyNested(t *testing.T) {
	results := collectTokens(t, `{"a":{"b":{"c":1}}}`)

	want := []string{
		`[["a","b","c"],1]`,
		`[["a","b","c"]]`,
		`[["a","b"]]`,
		`[["a"]]`,
	}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}
	for i, w := range want {
		got := pairStr(results[i])
		if got != w {
			t.Errorf("result[%d] = %s, want %s", i, got, w)
		}
	}
}

// nestedArrayReader is a zero-allocation io.Reader that generates a JSON array
// of complex nested objects on demand. Each element looks like:
//
//	{
//	  "id": 42,
//	  "name": "user_000042",
//	  "active": true,
//	  "score": 42.5,
//	  "address": {
//	    "city": "city_42",
//	    "zip": 10042,
//	    "geo": {"lat": 42.123, "lon": -73.456}
//	  },
//	  "tags": ["alpha", "beta", "gamma"],
//	  "meta": null
//	}
//
// Max depth is 4 (array → object → address → geo). Each element is ~280 bytes,
// so 550,000 elements produce ~154 MB of JSON.
type nestedArrayReader struct {
	total   int
	current int
	buf     []byte
	pos     int
	phase   int // 0=open bracket, 1=elements, 2=close bracket, 3=EOF
}

func newNestedArrayReader(count int) *nestedArrayReader {
	return &nestedArrayReader{total: count}
}

func (r *nestedArrayReader) Read(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		if r.pos < len(r.buf) {
			n := copy(p[written:], r.buf[r.pos:])
			r.pos += n
			written += n
			continue
		}

		switch r.phase {
		case 0:
			r.buf = []byte("[")
			r.pos = 0
			r.phase = 1
		case 1:
			if r.current >= r.total {
				r.phase = 2
				continue
			}
			i := r.current
			active := "true"
			if i%3 == 0 {
				active = "false"
			}
			comma := ","
			if i == r.total-1 {
				comma = ""
			}
			r.buf = []byte(fmt.Sprintf(
				`{"id":%d,"name":"user_%06d","active":%s,"score":%d.5,`+
					`"address":{"city":"city_%d","zip":%d,"geo":{"lat":%d.123,"lon":-%d.456}},`+
					`"tags":["alpha","beta","gamma"],"meta":null}%s`,
				i, i, active, i, i, 10000+i, i%90, i%180, comma,
			))
			r.pos = 0
			r.current++
		case 2:
			r.buf = []byte("]")
			r.pos = 0
			r.phase = 3
		default:
			return written, io.EOF
		}
	}
	return written, nil
}

func heapInuse() uint64 {
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapInuse
}

// TestTokenReader_ConstantMemory streams ~154 MB of nested JSON (depth 4)
// through TokenReader and verifies heap stays bounded. The document is an
// array of 550,000 complex objects with sub-objects, arrays, strings,
// booleans, numbers, and nulls. Peak heap must stay well below the doc size.
// measureReaderSize drains a reader and returns the total bytes read.
func measureReaderSize(r io.Reader) int64 {
	var size int64
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		size += int64(n)
		if err != nil {
			break
		}
	}
	return size
}

// consumeTokens reads all tokens from tr, sampling heap at quarter points.
func consumeTokens(t *testing.T, tr *TokenReader, estimatedTokens int) (consumed int, samples [3]uint64) {
	t.Helper()
	samplePoints := [3]int{estimatedTokens / 4, estimatedTokens / 2, 3 * estimatedTokens / 4}
	nextSample := 0
	for {
		_, ok, err := tr.Next()
		if err != nil {
			t.Fatalf("unexpected error at token %d: %v", consumed, err)
		}
		if !ok {
			break
		}
		consumed++
		if nextSample < len(samplePoints) && consumed >= samplePoints[nextSample] {
			samples[nextSample] = heapInuse()
			nextSample++
		}
	}
	return consumed, samples
}

func TestTokenReader_ConstantMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory test in short mode")
	}

	const (
		elements  = 550_000
		heapLimit = 10 * 1024 * 1024 // 10 MB ceiling
	)

	docSize := measureReaderSize(newNestedArrayReader(elements))
	t.Logf("document size: %d bytes (%.1f MB)", docSize, float64(docSize)/(1024*1024))

	baseline := heapInuse()
	tr := NewTokenReader(newNestedArrayReader(elements))
	consumed, samples := consumeTokens(t, tr, elements*16)
	final := heapInuse()

	t.Logf("total tokens consumed: %d (%.0f per element)", consumed, float64(consumed)/float64(elements))

	check := func(label string, inuse uint64) {
		t.Helper()
		net := int64(inuse) - int64(baseline)
		if net < 0 {
			net = 0
		}
		t.Logf("%-5s HeapInuse=%.2f MB  net=%.2f MB",
			label, float64(inuse)/(1024*1024), float64(net)/(1024*1024))
		if uint64(net) >= heapLimit {
			t.Errorf("%s: net heap %.2f MB >= limit %.2f MB (doc size %.1f MB)",
				label, float64(net)/(1024*1024), float64(heapLimit)/(1024*1024),
				float64(docSize)/(1024*1024))
		}
	}

	check("25%", samples[0])
	check("50%", samples[1])
	check("75%", samples[2])
	check("done", final)
}

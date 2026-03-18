package input

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
)

func collectTOONTokens(t *testing.T, input string) []any {
	t.Helper()
	tr := NewTOONTokenReader(strings.NewReader(input))
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

func assertTOONTokens(t *testing.T, input string, want []string) {
	t.Helper()
	results := collectTOONTokens(t, input)
	if len(results) != len(want) {
		strs := make([]string, len(results))
		for i, r := range results {
			strs[i] = pairStr(r)
		}
		t.Fatalf("got %d results, want %d\ngot:  %v\nwant: %v", len(results), len(want), strs, want)
	}
	for i, w := range want {
		got := pairStr(results[i])
		if got != w {
			t.Errorf("result[%d] = %s, want %s", i, got, w)
		}
	}
}

func TestTOONTokenReader_SimpleKV(t *testing.T) {
	// Matches: {"a":1,"b":"hello"} → [["a"],1] [["b"],"hello"] [["b"]]
	assertTOONTokens(t, "a: 1\nb: hello", []string{
		`[["a"],1]`,
		`[["b"],"hello"]`,
		`[["b"]]`,
	})
}

func TestTOONTokenReader_NestedObject(t *testing.T) {
	// Matches: {"a":{"b":1}} → [["a","b"],1] [["a","b"]] [["a"]]
	assertTOONTokens(t, "a:\n  b: 1", []string{
		`[["a","b"],1]`,
		`[["a","b"]]`,
		`[["a"]]`,
	})
}

func TestTOONTokenReader_DeeplyNested(t *testing.T) {
	// Matches: {"a":{"b":{"c":1}}}
	assertTOONTokens(t, "a:\n  b:\n    c: 1", []string{
		`[["a","b","c"],1]`,
		`[["a","b","c"]]`,
		`[["a","b"]]`,
		`[["a"]]`,
	})
}

func TestTOONTokenReader_InlineArray(t *testing.T) {
	// Matches: {"items":["a","b","c"]}
	assertTOONTokens(t, "items[3]: a,b,c", []string{
		`[["items",0],"a"]`,
		`[["items",1],"b"]`,
		`[["items",2],"c"]`,
		`[["items",2]]`,
		`[["items"]]`,
	})
}

func TestTOONTokenReader_ListArray(t *testing.T) {
	// Matches: {"items":["first","second"]}
	assertTOONTokens(t, "items[2]:\n  - first\n  - second", []string{
		`[["items",0],"first"]`,
		`[["items",1],"second"]`,
		`[["items",1]]`,
		`[["items"]]`,
	})
}

func TestTOONTokenReader_TabularArray(t *testing.T) {
	// Matches: {"users":[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]}
	assertTOONTokens(t, "users[2]{id,name}:\n  1,Alice\n  2,Bob", []string{
		`[["users",0,"id"],1]`,
		`[["users",0,"name"],"Alice"]`,
		`[["users",0,"name"]]`,
		`[["users",1,"id"],2]`,
		`[["users",1,"name"],"Bob"]`,
		`[["users",1,"name"]]`,
		`[["users",1]]`,
		`[["users"]]`,
	})
}

func TestTOONTokenReader_Primitives(t *testing.T) {
	assertTOONTokens(t, "t: true\nf: false\nn: null\ni: 42", []string{
		`[["t"],true]`,
		`[["f"],false]`,
		`[["n"],<nil>]`,
		`[["i"],42]`,
		`[["i"]]`,
	})
}

func TestTOONTokenReader_QuotedKeyValue(t *testing.T) {
	assertTOONTokens(t, `"special key": "hello world"`, []string{
		`[["special key"],"hello world"]`,
		`[["special key"]]`,
	})
}

func TestTOONTokenReader_Empty(t *testing.T) {
	results := collectTOONTokens(t, "")
	if len(results) != 0 {
		t.Fatalf("expected no results from empty input, got %d", len(results))
	}
}

func TestTOONTokenReader_BlankLines(t *testing.T) {
	assertTOONTokens(t, "a: 1\n\nb: 2", []string{
		`[["a"],1]`,
		`[["b"],2]`,
		`[["b"]]`,
	})
}

func TestTOONTokenReader_MixedNesting(t *testing.T) {
	// Matches: {"name":"Alice","address":{"city":"NYC","zip":10001},"age":30}
	input := "name: Alice\naddress:\n  city: NYC\n  zip: 10001\nage: 30"
	assertTOONTokens(t, input, []string{
		`[["name"],"Alice"]`,
		`[["address","city"],"NYC"]`,
		`[["address","zip"],10001]`,
		`[["address","zip"]]`,
		`[["age"],30]`,
		`[["age"]]`,
	})
}

func TestTOONTokenReader_ListObjectItems(t *testing.T) {
	// Matches: {"users":[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]}
	input := "users[2]:\n  - id: 1\n    name: Alice\n  - id: 2\n    name: Bob"
	assertTOONTokens(t, input, []string{
		`[["users",0,"id"],1]`,
		`[["users",0,"name"],"Alice"]`,
		`[["users",0,"name"]]`,
		`[["users",1,"id"],2]`,
		`[["users",1,"name"],"Bob"]`,
		`[["users",1,"name"]]`,
		`[["users",1]]`,
		`[["users"]]`,
	})
}

func TestTOONTokenReader_MultipleTopKeys(t *testing.T) {
	assertTOONTokens(t, "x: 1\ny: 2\nz: 3", []string{
		`[["x"],1]`,
		`[["y"],2]`,
		`[["z"],3]`,
		`[["z"]]`,
	})
}

// toonArrayReader generates a large TOON list array lazily.
type toonArrayReader struct {
	total   int
	current int
	buf     []byte
	pos     int
	phase   int // 0=header, 1=items, 2=EOF
}

func newTOONArrayReader(count int) *toonArrayReader {
	return &toonArrayReader{total: count}
}

func (r *toonArrayReader) Read(p []byte) (int, error) {
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
			r.buf = []byte(fmt.Sprintf("items[%d]:\n", r.total))
			r.pos = 0
			r.phase = 1
		case 1:
			if r.current >= r.total {
				r.phase = 2
				continue
			}
			r.buf = []byte(fmt.Sprintf("  - id: %d\n    name: user_%06d\n    active: true\n", r.current, r.current))
			r.pos = 0
			r.current++
		default:
			return written, io.EOF
		}
	}
	return written, nil
}

func TestTOONTokenReader_ConstantMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory test in short mode")
	}

	const (
		elements  = 200_000
		heapLimit = 10 * 1024 * 1024 // 10 MB
	)

	baseline := heapInuse()
	tr := NewTOONTokenReader(newTOONArrayReader(elements))

	estimatedTokens := elements * 8
	samplePoints := [3]int{estimatedTokens / 4, estimatedTokens / 2, 3 * estimatedTokens / 4}
	var samples [3]uint64
	nextSample := 0
	consumed := 0

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
			t.Errorf("%s: net heap %.2f MB >= limit %.2f MB",
				label, float64(net)/(1024*1024), float64(heapLimit)/(1024*1024))
		}
	}

	check("25%", samples[0])
	check("50%", samples[1])
	check("75%", samples[2])
	check("done", final)
}

func BenchmarkStreamJSON(b *testing.B) {
	for range b.N {
		tr := NewTokenReader(newNestedArrayReader(35_000))
		for {
			_, ok, err := tr.Next()
			if err != nil {
				b.Fatal(err)
			}
			if !ok {
				break
			}
		}
	}
}

func BenchmarkStreamTOON(b *testing.B) {
	for range b.N {
		tr := NewTOONTokenReader(newTOONArrayReader(35_000))
		for {
			_, ok, err := tr.Next()
			if err != nil {
				b.Fatal(err)
			}
			if !ok {
				break
			}
		}
	}
}

// heapInuse is defined in tokenizer_test.go — reuse via package scope.
var _ = runtime.GC // ensure import

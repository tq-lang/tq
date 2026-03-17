package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binaryPath string

var coverDir string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "tq-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)

	binaryPath = filepath.Join(tmp, "tq")
	buildArgs := []string{"build", "-o", binaryPath}

	// Build with coverage instrumentation when GOCOVERDIR is set,
	// so CLI integration tests contribute to coverage reports.
	if dir := os.Getenv("GOCOVERDIR"); dir != "" {
		absDir, absErr := filepath.Abs(dir)
		if absErr != nil {
			panic("bad GOCOVERDIR: " + absErr.Error())
		}
		coverDir = absDir
		buildArgs = append(buildArgs, "-cover")
	}

	buildArgs = append(buildArgs, ".")
	cmd := exec.Command("go", buildArgs...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build tq: " + err.Error())
	}

	os.Exit(m.Run())
}

func runTQ(t *testing.T, stdin string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Stdin = strings.NewReader(stdin)
	if coverDir != "" {
		cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
	}
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run tq: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

func TestCLI(t *testing.T) {
	tests := []struct {
		name     string
		stdin    string
		args     []string
		wantCode int
		wantOut  string // substring in stdout
		wantErr  string // substring in stderr
	}{
		// Green: basic queries
		{"identity", `{"name":"Alice"}`, []string{"."}, 0, "name: Alice", ""},
		{"field access", `{"name":"Alice"}`, []string{".name"}, 0, "Alice", ""},
		{"array index", `[10,20,30]`, []string{".[1]"}, 0, "20", ""},
		{"array iteration", `[1,2,3]`, []string{".[]"}, 0, "3", ""},
		{"pipe and select", `{"users":[{"name":"Alice","active":true},{"name":"Bob","active":false}]}`, []string{`.users[] | select(.active) | .name`}, 0, "Alice", ""},
		{"object construction", `{"first":"Alice","age":30}`, []string{`{name: .first, age: .age}`}, 0, "Alice", ""},
		{"conditional", `{"val":10}`, []string{`if .val > 5 then "big" else "small" end`}, 0, "big", ""},
		{"map and length", `[1,2,3,4,5]`, []string{`map(select(. > 3)) | length`}, 0, "2", ""},
		{"string ops", `{"name":"Alice Smith"}`, []string{"-r", `.name | split(" ") | .[0]`}, 0, "Alice", ""},
		{"deep nesting", `{"a":{"b":{"c":"deep"}}}`, []string{".a.b.c"}, 0, "deep", ""},
		{"unicode", `{"e":"😀"}`, []string{"-r", ".e"}, 0, "😀", ""},
		{"large number", `{"n":1234567890}`, []string{"--json", "-c", ".n"}, 0, "1234567890", ""},

		// TOON input
		{"toon input", "name: Alice\nage: 30", []string{".name"}, 0, "Alice", ""},

		// Output flags
		{"json output", `{"a":1}`, []string{"--json", "."}, 0, `"a"`, ""},
		{"compact json", `{"a":1}`, []string{"--json", "-c", "."}, 0, `{"a":1}`, ""},
		{"raw output", `{"name":"Alice"}`, []string{"-r", ".name"}, 0, "Alice", ""},
		{"join output", `["a","b"]`, []string{"-r", "-j", ".[]"}, 0, "ab", ""},
		{"tab indent", `{"a":1}`, []string{"--json", "--tab", "."}, 0, "\t", ""},
		{"custom indent", `{"a":1}`, []string{"--json", "--indent", "4", "."}, 0, "    ", ""},

		// Special modes
		{"null input", "", []string{"-n", "1 + 1"}, 0, "2", ""},
		{"slurp", `{"a":1}`, []string{"--json", "-c", "-s", "."}, 0, `[{"a":1}]`, ""},
		{"version", "", []string{"--version"}, 0, "tq ", ""},
		{"delimiter tab", `[1,2,3]`, []string{"--delimiter", "tab", "."}, 0, "\t", ""},
		{"delimiter pipe", `[1,2,3]`, []string{"--delimiter", "pipe", "."}, 0, "|", ""},

		// Variables
		{"arg variable", "null", []string{"-n", "--arg", "name", "--arg", "Alice", "$name"}, 0, "Alice", ""},
		{"argjson variable", "null", []string{"-n", "--json", "-c", "--argjson", "data", "--argjson", `{"k":"v"}`, "$data"}, 0, `{"k":"v"}`, ""},

		// Exit status
		{"exit status with output", `{"a":1}`, []string{"-e", "."}, 0, "a", ""},
		{"exit status no output", `{"a":1}`, []string{"-e", "empty"}, 4, "", ""},

		// Format conversion
		{"json to toon", `{"name":"Alice"}`, []string{"."}, 0, "name", ""},
		{"toon to json", "name: Alice\nage: 30", []string{"--json", "-c", "."}, 0, `"name"`, ""},

		// Edge cases
		{"empty object", `{}`, []string{"--json", "-c", "."}, 0, "{}", ""},
		{"empty array", `[]`, []string{"--json", "-c", "."}, 0, "[]", ""},
		{"null value", `null`, []string{"--json", "-c", "."}, 0, "null", ""},

		// Streaming — multi-document input
		{"multi-doc json first", "{\"a\":1}\n{\"b\":2}", []string{"--json", "-c", "."}, 0, "{\"a\":1}", ""},
		{"multi-doc json second", "{\"a\":1}\n{\"b\":2}", []string{"--json", "-c", "."}, 0, "{\"b\":2}", ""},
		{"multi-doc toon", "key: Alice\nage: 30\n\nkey: Bob\nage: 25", []string{".key"}, 0, "Bob", ""},
		{"slurp multi-doc", "{\"a\":1}\n{\"b\":2}", []string{"--json", "-c", "-s", "length"}, 0, "2", ""},

		// --stream flag
		{"stream flag", `{"a":1,"b":2}`, []string{"--stream", "--json", "-c", "."}, 0, `[["a"],1]`, ""},
		{"stream with filter", `{"a":1,"b":2}`, []string{"--stream", "--json", "-c", `select(.[0][0] == "a")`}, 0, `[["a"],1]`, ""},
		{"stream nested json", `{"a":[1,2],"b":3}`, []string{"--stream", "--json", "-c", "."}, 0, `[["a",0],1]`, ""},
		{"stream empty object", `{}`, []string{"--stream", "--json", "-c", "."}, 0, `[[],{}]`, ""},
		{"stream empty array", `[]`, []string{"--stream", "--json", "-c", "."}, 0, `[[],[]]`, ""},
		{"stream toon fallback", "name: Alice\nage: 30", []string{"--stream", "--json", "-c", "."}, 0, `[["name"],"Alice"]`, ""},
		{"stream scalar", `42`, []string{"--stream", "--json", "-c", "."}, 0, `[[],42]`, ""},
		{"stream slurp json", `{"a":1}`, []string{"--stream", "--slurp", "--json", "-c", "."}, 0, `[["a"],1]`, ""},
		{"stream slurp toon", "name: Alice\nage: 30", []string{"--stream", "--slurp", "--json", "-c", "."}, 0, `[["name"],"Alice"]`, ""},
		{"stream toon true prefix", "true_value: 1\nother: 2", []string{"--stream", "--json", "-c", "."}, 0, `[["true_value"],1]`, ""},
		{"stream toon false prefix", "false_alarm: yes", []string{"--stream", "--json", "-c", "."}, 0, `[["false_alarm"],"yes"]`, ""},

		// TOON native streaming
		{"stream toon nested", "a:\n  b: 1", []string{"--stream", "--json", "-c", "."}, 0, `[["a","b"],1]`, ""},
		{"stream toon list array", "items[2]:\n  - first\n  - second", []string{"--stream", "--json", "-c", "."}, 0, `[["items",0],"first"]`, ""},

		// Filter warnings
		{"stream sort warning", `{}`, []string{"--stream", "--json", "-c", "select(false) | sort"}, 0, "", "warning: 'sort'"},
		{"stream sort_by warning", `{}`, []string{"--stream", "--json", "-c", "select(false) | sort_by(.x)"}, 0, "", "warning: 'sort_by'"},

		// Quiet mode
		{"quiet suppresses warning", `{}`, []string{"--stream", "--quiet", "--json", "-c", "select(false) | sort"}, 0, "", ""},

		// Help output
		{"help shows groups", "", []string{"--help"}, 0, "", "Output flags:"},
		{"help shows env", "", []string{"--help"}, 0, "", "TQ_STREAM_THRESHOLD"},
		{"help shows docs link", "", []string{"--help"}, 0, "", "github.com/tq-lang/tq"},

		// Errors
		{"invalid filter", `{}`, []string{".[invalid|||"}, 3, "", "parse error"},
		{"runtime error", `42`, []string{".foo"}, 5, "", "expected an object"},
		{"file not found", "", []string{".", "/nonexistent/file.json"}, 2, "", "no such file"},
		{"filter file not found", `{}`, []string{"-f", "/nonexistent/filter.jq"}, 2, "", "no such file"},
		{"unknown delimiter", `{}`, []string{"--delimiter", "invalid", "."}, 2, "", "unknown delimiter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := runTQ(t, tt.stdin, tt.args...)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr)
			}
			if tt.wantOut != "" && !strings.Contains(stdout, tt.wantOut) {
				t.Errorf("stdout = %q, want substring %q", stdout, tt.wantOut)
			}
			if tt.wantErr != "" && !strings.Contains(stderr, tt.wantErr) {
				t.Errorf("stderr = %q, want substring %q", stderr, tt.wantErr)
			}
		})
	}
}

func TestCLIWithFiles(t *testing.T) {
	tmp := t.TempDir()

	t.Run("file input", func(t *testing.T) {
		f := filepath.Join(tmp, "input.json")
		if err := os.WriteFile(f, []byte(`{"x":42}`), 0644); err != nil {
			t.Fatal(err)
		}
		stdout, _, code := runTQ(t, "", ".x", f)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout, "42") {
			t.Errorf("got %q, want 42", stdout)
		}
	})

	t.Run("from-file filter", func(t *testing.T) {
		f := filepath.Join(tmp, "filter.jq")
		if err := os.WriteFile(f, []byte(".name"), 0644); err != nil {
			t.Fatal(err)
		}
		stdout, _, code := runTQ(t, `{"name":"Bob"}`, "-f", f)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout, "Bob") {
			t.Errorf("got %q, want Bob", stdout)
		}
	})

	t.Run("multiple files", func(t *testing.T) {
		f1 := filepath.Join(tmp, "a.json")
		f2 := filepath.Join(tmp, "b.json")
		if err := os.WriteFile(f1, []byte(`{"v":1}`), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f2, []byte(`{"v":2}`), 0644); err != nil {
			t.Fatal(err)
		}
		stdout, _, code := runTQ(t, "", ".v", f1, f2)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout, "1") || !strings.Contains(stdout, "2") {
			t.Errorf("got %q, want both values", stdout)
		}
	})

	t.Run("multi-doc file", func(t *testing.T) {
		f := filepath.Join(tmp, "multi.json")
		if err := os.WriteFile(f, []byte("{\"a\":1}\n{\"b\":2}\n"), 0644); err != nil {
			t.Fatal(err)
		}
		stdout, _, code := runTQ(t, "", "--json", "-c", ".", f)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout, `{"a":1}`) || !strings.Contains(stdout, `{"b":2}`) {
			t.Errorf("got %q, want both docs", stdout)
		}
	})

	t.Run("auto-stream threshold", func(t *testing.T) {
		// Create a small file and set threshold very low to trigger auto-stream.
		f := filepath.Join(tmp, "small.json")
		if err := os.WriteFile(f, []byte(`{"a":1}`), 0644); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runTQ(t, "", "--stream-threshold", "1B", "--json", "-c", ".", f)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
		}
		if !strings.Contains(stderr, "streaming enabled") {
			t.Errorf("expected auto-stream info in stderr, got %q", stderr)
		}
		if !strings.Contains(stdout, `[["a"],1]`) {
			t.Errorf("expected stream output, got %q", stdout)
		}
	})

	t.Run("no-stream suppresses auto", func(t *testing.T) {
		f := filepath.Join(tmp, "small2.json")
		if err := os.WriteFile(f, []byte(`{"a":1}`), 0644); err != nil {
			t.Fatal(err)
		}
		_, stderr, code := runTQ(t, "", "--no-stream", "--stream-threshold", "1B", "--json", "-c", ".", f)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if strings.Contains(stderr, "streaming enabled") {
			t.Errorf("--no-stream should suppress auto-stream, got stderr %q", stderr)
		}
	})

	t.Run("env TQ_STREAM_THRESHOLD", func(t *testing.T) {
		f := filepath.Join(tmp, "small3.json")
		if err := os.WriteFile(f, []byte(`{"b":2}`), 0644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binaryPath, "--json", "-c", ".", f)
		cmd.Env = append(os.Environ(), "TQ_STREAM_THRESHOLD=1B")
		if coverDir != "" {
			cmd.Env = append(cmd.Env, "GOCOVERDIR="+coverDir)
		}
		var outBuf, errBuf strings.Builder
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			t.Fatalf("tq failed: %v", err)
		}
		if !strings.Contains(errBuf.String(), "streaming enabled") {
			t.Errorf("expected auto-stream via env var, got stderr %q", errBuf.String())
		}
	})

	t.Run("quiet suppresses auto-stream info", func(t *testing.T) {
		f := filepath.Join(tmp, "small4.json")
		if err := os.WriteFile(f, []byte(`{"a":1}`), 0644); err != nil {
			t.Fatal(err)
		}
		_, stderr, code := runTQ(t, "", "--quiet", "--stream-threshold", "1B", "--json", "-c", ".", f)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if strings.Contains(stderr, "streaming enabled") {
			t.Errorf("--quiet should suppress auto-stream info, got stderr %q", stderr)
		}
	})
}

//go:build unix

package main

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

// nestedArrayWriter generates a JSON array of complex nested objects on
// demand, writing directly into an io.Writer. Each element is ~280 bytes:
//
//	{"id":N,"name":"user_000042","active":true,"score":42.5,
//	 "address":{"city":"city_42","zip":10042,"geo":{"lat":42.123,"lon":-73.456}},
//	 "tags":["alpha","beta","gamma"],"meta":null}
func nestedArrayWriter(w io.Writer, count int) error {
	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		active := "true"
		if i%3 == 0 {
			active = "false"
		}
		comma := ","
		if i == count-1 {
			comma = ""
		}
		s := fmt.Sprintf(
			`{"id":%d,"name":"user_%06d","active":%s,"score":%d.5,`+
				`"address":{"city":"city_%d","zip":%d,"geo":{"lat":%d.123,"lon":-%d.456}},`+
				`"tags":["alpha","beta","gamma"],"meta":null}%s`,
			i, i, active, i, i, 10000+i, i%90, i%180, comma,
		)
		if _, err := io.WriteString(w, s); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "]")
	return err
}

// maxRSSBytes extracts the peak RSS from a finished process via
// ProcessState.SysUsage() → syscall.Rusage.Maxrss. On Linux Maxrss is
// in kilobytes; on macOS/BSDs it is in bytes.
func maxRSSBytes(cmd *exec.Cmd) uint64 {
	usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0
	}
	rss := uint64(usage.Maxrss)
	if runtime.GOOS == "linux" {
		rss *= 1024 // Linux reports kB
	}
	return rss
}

// TestStreamMemory launches tq --stream with ~100MB of nested JSON piped to
// stdin and checks peak RSS after the process exits. Uses syscall.Rusage.Maxrss
// which works on all Unix systems (Linux, macOS, BSDs). No external deps.
func TestStreamMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stream memory test in short mode")
	}

	const (
		elements = 550_000          // ~100 MB of JSON
		rssLimit = 30 * 1024 * 1024 // 30 MB peak RSS ceiling
	)

	cmd := exec.Command(binaryPath, "--stream", "--json", "-c", ".")
	if coverDir != "" {
		cmd.Env = append(cmd.Environ(), "GOCOVERDIR="+coverDir)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	cmd.Stdout = io.Discard

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := nestedArrayWriter(stdin, elements); err != nil {
		t.Fatalf("writing to stdin: %v", err)
	}
	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
		t.Fatalf("tq exited with error: %v\nstderr: %s", err, stderrBuf.String())
	}

	peak := maxRSSBytes(cmd)
	peakMB := float64(peak) / (1024 * 1024)
	limitMB := float64(rssLimit) / (1024 * 1024)

	t.Logf("peak RSS: %.1f MB (limit: %.0f MB, doc size: ~100 MB)", peakMB, limitMB)

	if peak > rssLimit {
		t.Errorf("peak RSS %.1f MB exceeds limit %.0f MB — stream mode may be materializing input",
			peakMB, limitMB)
	}
}

// toonNestedArrayWriter generates TOON list array data with simple object items.
func toonNestedArrayWriter(w io.Writer, count int) error {
	if _, err := fmt.Fprintf(w, "items[%d]:\n", count); err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		s := fmt.Sprintf("  - id: %d\n    name: user_%06d\n    active: true\n", i, i)
		if _, err := io.WriteString(w, s); err != nil {
			return err
		}
	}
	return nil
}

// TestStreamMemoryTOON launches tq --stream with ~100MB of TOON piped to stdin
// and checks peak RSS stays bounded.
func TestStreamMemoryTOON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TOON stream memory test in short mode")
	}

	const (
		elements = 550_000          // ~100 MB of TOON
		rssLimit = 30 * 1024 * 1024 // 30 MB peak RSS ceiling
	)

	cmd := exec.Command(binaryPath, "--stream", "--json", "-c", ".")
	if coverDir != "" {
		cmd.Env = append(cmd.Environ(), "GOCOVERDIR="+coverDir)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	cmd.Stdout = io.Discard

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := toonNestedArrayWriter(stdin, elements); err != nil {
		t.Fatalf("writing to stdin: %v", err)
	}
	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
		t.Fatalf("tq exited with error: %v\nstderr: %s", err, stderrBuf.String())
	}

	peak := maxRSSBytes(cmd)
	peakMB := float64(peak) / (1024 * 1024)
	limitMB := float64(rssLimit) / (1024 * 1024)

	t.Logf("peak RSS: %.1f MB (limit: %.0f MB, doc size: ~100 MB TOON)", peakMB, limitMB)

	if peak > rssLimit {
		t.Errorf("peak RSS %.1f MB exceeds limit %.0f MB — TOON stream mode may be materializing input",
			peakMB, limitMB)
	}
}

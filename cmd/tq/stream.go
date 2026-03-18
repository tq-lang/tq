package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/itchyny/gojq"

	"github.com/tq-lang/tq/internal/detect"
	"github.com/tq-lang/tq/internal/input"
)

// streamCfg holds pre-compiled filter state for streaming mode.
// Both JSON and TOON use native TokenReaders with O(depth) memory.
type streamCfg struct {
	code *gojq.Code // filter applied to [path, value] pairs
}

// buildStreamCfg creates a streamCfg from the user's filter expression.
func buildStreamCfg(filterExpr string, argVars, argjsonVars []keyValue) (*streamCfg, int) {
	code, _, rc := compileFilter(filterExpr, argVars, argjsonVars)
	if rc != 0 {
		return nil, rc
	}
	return &streamCfg{code: code}, 0
}

// resolveReader returns the appropriate value iterator and filter code.
// When sc is nil (no --stream), uses the standard Reader with code.
// When sc is set, detects JSON vs TOON and picks the right native tokenizer.
func resolveReader(r io.Reader, code *gojq.Code, sc *streamCfg) (valueIterator, *gojq.Code) {
	if sc == nil {
		return input.NewReader(r), code
	}
	br := bufio.NewReader(r)
	if !isConfirmedJSON(br) {
		return input.NewTOONTokenReader(br), sc.code
	}
	return input.NewTokenReader(br), sc.code
}

// isConfirmedJSON detects the input format from a buffered reader and validates
// the detection. Returns true only when the input is confidently JSON.
//
// The heuristic-based Detect can misclassify TOON as JSON when the first
// non-whitespace byte happens to match a JSON value start (e.g. 't' in a TOON
// key like "true_value:" looks like JSON "true"). We guard against this by
// attempting to decode a token from the already-peeked buffer and verifying the
// byte immediately following the token is valid JSON context (whitespace, comma,
// colon, bracket, or EOF).
func isConfirmedJSON(br *bufio.Reader) bool {
	format := detect.DetectReader(br)
	if format == detect.TOON {
		return false
	}

	peeked, _ := br.Peek(br.Buffered())
	if len(peeked) == 0 {
		return true // empty input - TokenReader handles EOF gracefully
	}

	testDec := json.NewDecoder(bytes.NewReader(peeked))
	if _, err := testDec.Token(); err != nil {
		return false
	}

	// Check that the byte right after the decoded token is valid in a JSON
	// context. This catches "true_value:" where Token() happily returns true
	// but the trailing '_' proves it's not actually JSON.
	offset := int(testDec.InputOffset())
	rest := peeked[offset:]
	for _, b := range rest {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case ',', ':', '[', ']', '{', '}', '"',
			'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
			'-', 't', 'f', 'n':
			return true
		default:
			return false
		}
	}
	// All remaining peeked bytes were whitespace (or there were none after
	// the token). This is valid JSON.
	return true
}

// shouldAutoStream returns true if filename refers to a file >= threshold bytes.
func shouldAutoStream(filename string, threshold int64) bool {
	if filename == "-" {
		return false
	}
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return info.Size() >= threshold
}

// resolveStreamThreshold determines the auto-stream threshold.
// Priority: --stream-threshold flag > TQ_STREAM_THRESHOLD env > 256MB default.
func resolveStreamThreshold(cfg *config) int64 {
	if cfg.streamThreshold != "" {
		if v, err := parseSize(cfg.streamThreshold); err == nil {
			return v
		}
		fmt.Fprintf(os.Stderr, "tq: warning: invalid --stream-threshold %q, using default\n", cfg.streamThreshold)
	}
	if env := os.Getenv("TQ_STREAM_THRESHOLD"); env != "" {
		if v, err := parseSize(env); err == nil {
			return v
		}
	}
	return 256 * 1024 * 1024 // 256MB
}

// parseSize parses a human-readable size string like "256MB", "1GB".
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)

	multipliers := []struct {
		suffix string
		mult   int64
	}{
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
		{"B", 1},
	}

	for _, m := range multipliers {
		if strings.HasSuffix(s, m.suffix) {
			numStr := strings.TrimSpace(s[:len(s)-len(m.suffix)])
			var n int64
			if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil {
				return 0, fmt.Errorf("invalid size: %s", s)
			}
			return n * m.mult, nil
		}
	}

	// Plain number = bytes.
	var n int64
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, fmt.Errorf("invalid size: %s", s)
	}
	return n, nil
}

// formatSize returns a human-readable size string.
func formatSize(n int64) string {
	switch {
	case n >= 1024*1024*1024:
		return fmt.Sprintf("%dGB", n/(1024*1024*1024))
	case n >= 1024*1024:
		return fmt.Sprintf("%dMB", n/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%dKB", n/1024)
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// warnNonStreamable prints a warning for filter builtins that may not work
// as expected in streaming mode where input is [path,value] pairs.
func warnNonStreamable(filterExpr string) {
	builtins := []string{
		"sort_by", "group_by", "unique_by", "min_by", "max_by",
		"sort", "unique", "reverse", "transpose", "flatten",
		"combinations", "walk",
	}
	for _, name := range builtins {
		if matchesBuiltin(filterExpr, name) {
			fmt.Fprintf(os.Stderr,
				"tq: warning: '%s' may not work as expected in streaming mode "+
					"(input is [path,value] pairs, not the full document)\n", name)
		}
	}
}

// matchesBuiltin checks if filterExpr contains name as a whole word.
func matchesBuiltin(filterExpr, name string) bool {
	idx := 0
	for {
		pos := strings.Index(filterExpr[idx:], name)
		if pos == -1 {
			return false
		}
		pos += idx
		before := pos - 1
		after := pos + len(name)
		beforeOK := before < 0 || !isIdentChar(filterExpr[before])
		afterOK := after >= len(filterExpr) || !isIdentChar(filterExpr[after])
		if beforeOK && afterOK {
			return true
		}
		idx = pos + len(name)
	}
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

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
	if detect.DetectReader(br) == detect.TOON {
		return false
	}
	peeked, _ := br.Peek(br.Buffered())
	if len(peeked) == 0 {
		return true
	}
	return validateJSONToken(peeked)
}

func validateJSONToken(peeked []byte) bool {
	testDec := json.NewDecoder(bytes.NewReader(peeked))
	if _, err := testDec.Token(); err != nil {
		return false
	}
	return isValidJSONContext(peeked[int(testDec.InputOffset()):])
}

func isValidJSONContext(rest []byte) bool {
	for _, b := range rest {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		return isJSONFollowByte(b)
	}
	return true
}

func isJSONFollowByte(b byte) bool {
	switch b {
	case ',', ':', '[', ']', '{', '}', '"',
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
		'-', 't', 'f', 'n':
		return true
	}
	return false
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

const defaultStreamThreshold = 256 * 1024 * 1024 // 256MB

// resolveStreamThreshold determines the auto-stream threshold.
// Priority: --stream-threshold flag > TQ_STREAM_THRESHOLD env > 256MB default.
func resolveStreamThreshold(cfg *config) int64 {
	if v, ok := parseFlagThreshold(cfg); ok {
		return v
	}
	return resolveEnvThreshold()
}

func parseFlagThreshold(cfg *config) (int64, bool) {
	if cfg.streamThreshold == "" {
		return 0, false
	}
	v, err := parseSize(cfg.streamThreshold)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: warning: invalid --stream-threshold %q, using default\n", cfg.streamThreshold)
		return 0, false
	}
	return v, true
}

func resolveEnvThreshold() int64 {
	if env := os.Getenv("TQ_STREAM_THRESHOLD"); env != "" {
		if v, err := parseSize(env); err == nil {
			return v
		}
	}
	return defaultStreamThreshold
}

type sizeUnit struct {
	suffix string
	mult   int64
}

var sizeUnits = []sizeUnit{
	{"GB", 1024 * 1024 * 1024},
	{"MB", 1024 * 1024},
	{"KB", 1024},
	{"B", 1},
}

// parseSize parses a human-readable size string like "256MB", "1GB".
func parseSize(s string) (int64, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	for _, u := range sizeUnits {
		if strings.HasSuffix(s, u.suffix) {
			return parseSizeWithUnit(s, u)
		}
	}
	return scanInt64(s)
}

func parseSizeWithUnit(s string, u sizeUnit) (int64, error) {
	n, err := scanInt64(strings.TrimSpace(s[:len(s)-len(u.suffix)]))
	if err != nil {
		return 0, err
	}
	return n * u.mult, nil
}

func scanInt64(s string) (int64, error) {
	var n int64
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, fmt.Errorf("invalid size: %s", s)
	}
	return n, nil
}

// formatSize returns a human-readable size string.
func formatSize(n int64) string {
	for _, u := range sizeUnits {
		if u.suffix == "B" {
			break
		}
		if n >= u.mult {
			return fmt.Sprintf("%d%s", n/u.mult, u.suffix)
		}
	}
	return fmt.Sprintf("%dB", n)
}

var nonStreamableBuiltins = []string{
	"sort_by", "group_by", "unique_by", "min_by", "max_by",
	"sort", "unique", "reverse", "transpose", "flatten",
	"combinations", "walk",
}

// warnNonStreamable prints a warning for filter builtins that may not work
// as expected in streaming mode where input is [path,value] pairs.
func warnNonStreamable(filterExpr string) {
	for _, name := range nonStreamableBuiltins {
		warnIfMatched(filterExpr, name)
	}
}

func warnIfMatched(filterExpr, name string) {
	if matchesBuiltin(filterExpr, name) {
		fmt.Fprintf(os.Stderr,
			"tq: warning: '%s' may not work as expected in streaming mode "+
				"(input is [path,value] pairs, not the full document)\n", name)
	}
}

// matchesBuiltin checks if expr contains name as a whole word.
func matchesBuiltin(expr, name string) bool {
	idx := 0
	for idx >= 0 {
		var found bool
		idx, found = matchBuiltinAt(expr, name, idx)
		if found {
			return true
		}
	}
	return false
}

// matchBuiltinAt searches for name starting at idx.
// Returns (nextOffset, found) where nextOffset < 0 means no more occurrences.
func matchBuiltinAt(expr, name string, idx int) (int, bool) {
	pos := findSubstr(expr, name, idx)
	if pos < 0 {
		return -1, false
	}
	if isWholeWord(expr, pos, len(name)) {
		return -1, true
	}
	return pos + len(name), false
}

func findSubstr(s, sub string, offset int) int {
	pos := strings.Index(s[offset:], sub)
	if pos < 0 {
		return -1
	}
	return pos + offset
}

func isWholeWord(s string, pos, nameLen int) bool {
	beforeOK := pos == 0 || !isIdentChar(s[pos-1])
	after := pos + nameLen
	afterOK := after >= len(s) || !isIdentChar(s[after])
	return beforeOK && afterOK
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// Package detect provides input format auto-detection for TOON and JSON.
package detect

import "bufio"

// Format represents a supported input format.
type Format int

const (
	TOON Format = iota
	JSON
)

// Detect examines raw input bytes and returns the detected format.
// JSON is detected when the first non-whitespace character is one of:
// { [ " 0-9 - t f n (matching JSON value starts).
// Everything else is assumed to be TOON.
func Detect(data []byte) Format {
	for _, b := range data {
		if isWhitespace(b) {
			continue
		}
		return classifyByte(b)
	}
	return TOON
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func classifyByte(b byte) Format {
	switch b {
	case '{', '[', '"', 't', 'f', 'n', '-',
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return JSON
	}
	return TOON
}

// DetectReader peeks at the buffered reader to determine format without
// consuming any bytes. Uses the same heuristic as Detect.
func DetectReader(r *bufio.Reader) Format {
	buf, _ := r.Peek(4096)
	return Detect(buf)
}

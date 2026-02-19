// Package detect provides input format auto-detection for TOON and JSON.
package detect

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
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '{', '[', '"', 't', 'f', 'n', '-',
			'0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return JSON
		default:
			return TOON
		}
	}
	return TOON
}

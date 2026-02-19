// Package input parses TOON and JSON documents into dynamic Go values.
package input

import (
	"encoding/json"
	"fmt"

	"github.com/toon-format/toon-go"

	"github.com/tq-lang/tq/internal/detect"
)

// Parse reads raw bytes and returns the parsed value as any.
// The format is auto-detected unless explicitly specified.
func Parse(data []byte) (any, error) {
	format := detect.Detect(data)
	switch format {
	case detect.JSON:
		return parseJSON(data)
	case detect.TOON:
		return parseTOON(data)
	default:
		return nil, fmt.Errorf("unknown format")
	}
}

func parseJSON(data []byte) (any, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		// Fallback: maybe it was TOON that looked like JSON
		return parseTOON(data)
	}
	return v, nil
}

func parseTOON(data []byte) (any, error) {
	return toon.Decode(data)
}

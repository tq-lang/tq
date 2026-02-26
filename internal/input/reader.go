// Package input parses TOON and JSON documents from io.Readers into dynamic Go values.
//
// Currently a single file. If the package grows, the JSON and TOON decoding
// paths (nextJSON/nextTOON) are natural split points into separate files.
package input

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"

	"github.com/toon-format/toon-go"

	"github.com/tq-lang/tq/internal/detect"
)

// Reader reads values from an io.Reader one at a time, supporting
// multiple concatenated JSON or TOON documents.
type Reader struct {
	format     detect.Format
	jsonDec    *json.Decoder  // used for JSON streams
	scanner    *bufio.Scanner // used for TOON streams
	buf        bytes.Buffer   // accumulates TOON lines between blank-line separators
	done       bool
	underlying *bufio.Reader // original buffered reader
	captured   *bytes.Buffer // captures bytes for JSON→TOON fallback
	firstRead  bool          // true until first successful JSON decode
}

// NewReader creates a reader that yields parsed values one at a time.
// The format is auto-detected by peeking at the first non-whitespace byte.
func NewReader(r io.Reader) *Reader {
	br := bufio.NewReader(r)
	format := detect.DetectReader(br)

	sr := &Reader{
		format:     format,
		underlying: br,
		firstRead:  true,
	}

	if format == detect.JSON {
		// Capture bytes via TeeReader for possible TOON fallback. The capture
		// is bounded: it only grows until the first Decode call succeeds or
		// fails, after which the TeeReader is discarded.
		sr.captured = &bytes.Buffer{}
		tee := io.TeeReader(br, sr.captured)
		sr.jsonDec = json.NewDecoder(tee)
	} else {
		sr.scanner = bufio.NewScanner(br)
	}

	return sr
}

// Next returns the next parsed value from the stream.
// Returns (value, true, nil) for each value, (nil, false, nil) at EOF,
// or (nil, false, err) on parse error.
func (sr *Reader) Next() (any, bool, error) {
	if sr.done {
		return nil, false, nil
	}

	if sr.format == detect.JSON {
		return sr.nextJSON()
	}
	return sr.nextTOON()
}

func (sr *Reader) nextJSON() (any, bool, error) {
	// Use Decode directly instead of More() — More() is designed for values
	// inside JSON arrays/objects, not top-level concatenated streams where
	// buffer-boundary splits could cause premature EOF detection.
	var v any
	if err := sr.jsonDec.Decode(&v); err != nil {
		if err == io.EOF {
			sr.done = true
			return nil, false, nil
		}
		if sr.firstRead {
			// First decode failed — input was misdetected as JSON.
			// Fall back to TOON using the captured bytes + remaining reader.
			sr.fallbackToTOON()
			return sr.nextTOON()
		}
		sr.done = true
		return nil, false, err
	}

	if sr.firstRead {
		sr.firstRead = false
		// First decode succeeded — discard the TeeReader to stop capturing.
		// This is safe because the old decoder is replaced: Buffered() returns
		// any look-ahead data already read, and underlying has the rest.
		remaining := io.MultiReader(sr.jsonDec.Buffered(), sr.underlying)
		sr.jsonDec = json.NewDecoder(remaining)
		sr.captured = nil
	}

	return v, true, nil
}

// fallbackToTOON switches from JSON mode to TOON mode, reconstructing
// the full stream from captured bytes and the remaining underlying reader.
func (sr *Reader) fallbackToTOON() {
	reconstructed := io.MultiReader(
		bytes.NewReader(sr.captured.Bytes()),
		sr.underlying,
	)
	sr.format = detect.TOON
	sr.scanner = bufio.NewScanner(reconstructed)
	sr.jsonDec = nil
	sr.captured = nil
}

func (sr *Reader) nextTOON() (any, bool, error) {
	// Accumulate lines until we hit a blank line (document separator) or EOF.
	// A blank line between non-empty content separates documents.
	// Note: only truly empty lines ("") act as separators; lines containing
	// only whitespace (spaces/tabs) are treated as content.
	sr.buf.Reset()
	hasContent := false

	for sr.scanner.Scan() {
		line := sr.scanner.Text()

		if line == "" && hasContent {
			// Blank line after content = document boundary.
			return sr.decodeTOONBuffer()
		}

		if line != "" {
			hasContent = true
		}

		sr.buf.WriteString(line)
		sr.buf.WriteByte('\n')
	}

	if err := sr.scanner.Err(); err != nil {
		sr.done = true
		return nil, false, err
	}

	sr.done = true

	// EOF reached — decode whatever we accumulated.
	if !hasContent {
		return nil, false, nil
	}
	return sr.decodeTOONBuffer()
}

func (sr *Reader) decodeTOONBuffer() (any, bool, error) {
	data := sr.buf.Bytes()
	v, err := toon.Decode(data)
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

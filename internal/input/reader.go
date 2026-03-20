// Package input parses TOON and JSON documents from io.Readers into dynamic Go values.
//
// Currently a single file. If the package grows, the JSON and TOON decoding
// paths (nextJSON/nextTOON) are natural split points into separate files.
package input

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
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
	sr := &Reader{format: format, underlying: br, firstRead: true}
	sr.initDecoder(br, format)
	return sr
}

func (sr *Reader) initDecoder(br *bufio.Reader, format detect.Format) {
	if format == detect.JSON {
		sr.captured = &bytes.Buffer{}
		tee := io.TeeReader(br, sr.captured)
		sr.jsonDec = json.NewDecoder(tee)
	} else {
		sr.scanner = bufio.NewScanner(br)
	}
}

// NewTOONReader creates a reader that parses TOON documents without format
// detection. Use this when the caller has already confirmed the input is TOON.
func NewTOONReader(r io.Reader) *Reader {
	br := bufio.NewReader(r)
	return &Reader{
		format:     detect.TOON,
		underlying: br,
		scanner:    bufio.NewScanner(br),
	}
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
	var v any
	if err := sr.jsonDec.Decode(&v); err != nil {
		return sr.handleJSONDecodeErr(err)
	}
	sr.finalizeFirstRead()
	return v, true, nil
}

func (sr *Reader) handleJSONDecodeErr(err error) (any, bool, error) {
	if errors.Is(err, io.EOF) {
		sr.done = true
		return nil, false, nil
	}
	if sr.firstRead {
		sr.fallbackToTOON()
		return sr.nextTOON()
	}
	sr.done = true
	return nil, false, err
}

func (sr *Reader) finalizeFirstRead() {
	if !sr.firstRead {
		return
	}
	sr.firstRead = false
	remaining := io.MultiReader(sr.jsonDec.Buffered(), sr.underlying)
	sr.jsonDec = json.NewDecoder(remaining)
	sr.captured = nil
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
	sr.buf.Reset()
	separated, hasContent := sr.scanTOONLines()
	if separated {
		return sr.decodeTOONBuffer()
	}
	return sr.finalizeTOON(hasContent)
}

func (sr *Reader) scanTOONLines() (separated bool, hasContent bool) {
	for sr.scanner.Scan() {
		line := sr.scanner.Text()
		if line == "" && hasContent {
			return true, true
		}
		if line != "" {
			hasContent = true
		}
		sr.buf.WriteString(line)
		sr.buf.WriteByte('\n')
	}
	return false, hasContent
}

func (sr *Reader) finalizeTOON(hasContent bool) (any, bool, error) {
	if err := sr.scanner.Err(); err != nil {
		sr.done = true
		return nil, false, err
	}
	sr.done = true
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

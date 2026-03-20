// Package output formats Go values as TOON or JSON.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/toon-format/toon-go"
)

// Options controls output formatting.
type Options struct {
	JSON      bool
	Raw       bool
	Compact   bool
	Tab       bool
	Indent    int
	Join      bool
	Delimiter toon.Delimiter
}

// Write formats v and writes it to w. Returns an error if formatting fails.
func Write(w io.Writer, v any, opts Options) error {
	if opts.Raw {
		if s, ok := v.(string); ok {
			return writeRaw(w, s, opts.Join)
		}
	}
	return writeFormatted(w, v, opts)
}

func writeRaw(w io.Writer, s string, join bool) error {
	if join {
		_, err := fmt.Fprint(w, s)
		return err
	}
	_, err := fmt.Fprintln(w, s)
	return err
}

func writeFormatted(w io.Writer, v any, opts Options) error {
	if opts.JSON {
		return writeJSON(w, v, opts)
	}
	return writeTOON(w, v, opts)
}

func writeJSON(w io.Writer, v any, opts Options) error {
	data, err := marshalJSON(v, opts)
	if err != nil {
		return err
	}
	return writeAndTerminate(w, data, opts.Join)
}

func marshalJSON(v any, opts Options) ([]byte, error) {
	if opts.Compact {
		return json.Marshal(v)
	}
	return json.MarshalIndent(v, "", indentString(opts))
}

func indentString(opts Options) string {
	if opts.Tab {
		return "\t"
	}
	if opts.Indent > 0 {
		return strings.Repeat(" ", opts.Indent)
	}
	return "  "
}

func writeTOON(w io.Writer, v any, opts Options) error {
	encoderOpts := buildTOONOpts(opts)
	data, err := toon.Marshal(v, encoderOpts...)
	if err != nil {
		return err
	}
	return writeAndTerminate(w, data, opts.Join)
}

func buildTOONOpts(opts Options) []toon.EncoderOption {
	var encoderOpts []toon.EncoderOption
	if opts.Tab {
		encoderOpts = append(encoderOpts, toon.WithIndent(0))
	} else if opts.Indent > 0 {
		encoderOpts = append(encoderOpts, toon.WithIndent(opts.Indent))
	}
	if opts.Delimiter != 0 {
		encoderOpts = append(encoderOpts, toon.WithArrayDelimiter(opts.Delimiter))
	}
	return encoderOpts
}

func writeAndTerminate(w io.Writer, data []byte, join bool) error {
	if _, err := w.Write(data); err != nil {
		return err
	}
	return terminateLine(w, join)
}

// terminateLine writes a newline unless join mode is active.
func terminateLine(w io.Writer, join bool) error {
	if join {
		return nil
	}
	_, err := fmt.Fprintln(w)
	return err
}

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
			if opts.Join {
				_, err := fmt.Fprint(w, s)
				return err
			}
			_, err := fmt.Fprintln(w, s)
			return err
		}
	}

	if opts.JSON {
		return writeJSON(w, v, opts)
	}
	return writeTOON(w, v, opts)
}

func writeJSON(w io.Writer, v any, opts Options) error {
	var data []byte
	var err error

	if opts.Compact {
		data, err = json.Marshal(v)
	} else {
		data, err = json.MarshalIndent(v, "", indentString(opts))
	}
	if err != nil {
		return err
	}

	if _, err = w.Write(data); err != nil {
		return err
	}
	return terminateLine(w, opts.Join)
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
	var encoderOpts []toon.EncoderOption

	if opts.Tab {
		encoderOpts = append(encoderOpts, toon.WithIndent(0))
	} else if opts.Indent > 0 {
		encoderOpts = append(encoderOpts, toon.WithIndent(opts.Indent))
	}

	if opts.Delimiter != 0 {
		encoderOpts = append(encoderOpts, toon.WithArrayDelimiter(opts.Delimiter))
	}

	data, err := toon.Marshal(v, encoderOpts...)
	if err != nil {
		return err
	}
	if _, err = w.Write(data); err != nil {
		return err
	}
	return terminateLine(w, opts.Join)
}

// terminateLine writes a newline unless join mode is active.
func terminateLine(w io.Writer, join bool) error {
	if join {
		return nil
	}
	_, err := fmt.Fprintln(w)
	return err
}

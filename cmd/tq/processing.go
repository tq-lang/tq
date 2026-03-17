package main

import (
	"fmt"
	"io"
	"os"

	"github.com/itchyny/gojq"

	"github.com/tq-lang/tq/internal/output"
)

// processFiles reads each file (or "-" for stdin) as a streaming source.
func processFiles(files []string, code *gojq.Code, varValues []any, opts output.Options, slurp bool, sc *streamCfg, hasOutput *bool, cfg *config, threshold int64, filterExpr string, argVars, argjsonVars []keyValue) int {
	if slurp {
		var all []any
		for _, f := range files {
			vals, rc := slurpFileValues(f, code, sc)
			if rc != 0 {
				return rc
			}
			all = append(all, vals...)
		}
		return executeFilter(code, all, varValues, opts, hasOutput)
	}

	exitCode := 0
	for _, f := range files {
		fileSC := sc
		// Auto-detect streaming for large files when not explicitly set.
		if fileSC == nil && !cfg.noStream && shouldAutoStream(f, threshold) {
			var src int
			fileSC, src = buildStreamCfg(filterExpr, argVars, argjsonVars)
			if src != 0 {
				return src
			}
			if !cfg.quiet {
				fmt.Fprintf(os.Stderr, "tq: info: streaming enabled for %s (file > %s)\n", fileLabel(f), formatSize(threshold))
				warnNonStreamable(filterExpr)
			}
		}
		rc := filterFile(f, code, varValues, opts, hasOutput, fileSC)
		if rc == exitUsage {
			return rc
		}
		if rc != 0 {
			exitCode = rc
		}
	}
	return exitCode
}

// filterFile opens a single file, runs the filter on each value, and closes it.
func filterFile(filename string, code *gojq.Code, varValues []any, opts output.Options, hasOutput *bool, sc *streamCfg) int {
	r, cleanup, rc := openFileReader(filename)
	if rc != 0 {
		return rc
	}
	defer cleanup()
	return filterAll(r, fileLabel(filename), code, varValues, opts, hasOutput, sc)
}

// slurpFileValues reads all values from a single file into a slice.
func slurpFileValues(filename string, code *gojq.Code, sc *streamCfg) ([]any, int) {
	r, cleanup, rc := openFileReader(filename)
	if rc != 0 {
		return nil, rc
	}
	defer cleanup()
	iter, _ := resolveReader(r, code, sc)
	return collectReaderValues(iter, fileLabel(filename))
}

// filterAll reads values from r and runs the filter on each.
// Delegates to resolveReader for format detection and stream mode handling.
func filterAll(r io.Reader, label string, code *gojq.Code, varValues []any, opts output.Options, hasOutput *bool, sc *streamCfg) int {
	iter, filterCode := resolveReader(r, code, sc)
	return filterLoop(iter, label, filterCode, varValues, opts, hasOutput)
}

// filterLoop reads values from a reader and runs the filter on each.
func filterLoop(next valueIterator, label string, code *gojq.Code, varValues []any, opts output.Options, hasOutput *bool) int {
	exitCode := 0
	for {
		v, ok, err := next.Next()
		if err != nil {
			fmt.Fprintf(os.Stderr, "tq: %s: %v\n", label, err)
			return exitUsage
		}
		if !ok {
			break
		}
		if rc := executeFilter(code, v, varValues, opts, hasOutput); rc != 0 {
			exitCode = rc
		}
	}
	return exitCode
}

// valueIterator abstracts over Reader and TokenReader for filterLoop.
type valueIterator interface {
	Next() (any, bool, error)
}

// slurpAll collects all values from r into an array, then runs the filter once.
// resolveReader picks the right iterator and filter code for the detected format.
func slurpAll(r io.Reader, label string, code *gojq.Code, varValues []any, opts output.Options, sc *streamCfg, hasOutput *bool) int {
	iter, filterCode := resolveReader(r, code, sc)
	vals, rc := collectReaderValues(iter, label)
	if rc != 0 {
		return rc
	}
	return executeFilter(filterCode, vals, varValues, opts, hasOutput)
}

func collectReaderValues(next valueIterator, label string) ([]any, int) {
	var vals []any
	for {
		v, ok, err := next.Next()
		if err != nil {
			fmt.Fprintf(os.Stderr, "tq: %s: %v\n", label, err)
			return nil, exitUsage
		}
		if !ok {
			break
		}
		vals = append(vals, v)
	}
	return vals, 0
}

// openFileReader opens a file or stdin ("-") and returns a reader and cleanup func.
func openFileReader(filename string) (io.Reader, func(), int) {
	if filename == "-" {
		return os.Stdin, func() {}, 0
	}
	// #nosec G304 -- tq intentionally opens user-provided CLI file paths.
	fh, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: %v\n", err)
		return nil, nil, exitUsage
	}
	return fh, func() { _ = fh.Close() }, 0
}

func fileLabel(filename string) string {
	if filename == "-" {
		return "stdin"
	}
	return filename
}

// executeFilter runs the compiled filter on a single input and writes results.
func executeFilter(code *gojq.Code, inp any, varValues []any, opts output.Options, hasOutput *bool) int {
	iter := code.Run(inp, varValues...)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			fmt.Fprintf(os.Stderr, "tq: %v\n", err)
			return exitRuntime
		}
		*hasOutput = true
		if err := output.Write(os.Stdout, v, opts); err != nil {
			fmt.Fprintf(os.Stderr, "tq: %v\n", err)
			return exitUsage
		}
	}
	return exitOK
}

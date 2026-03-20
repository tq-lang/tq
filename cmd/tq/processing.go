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
		return slurpFiles(files, code, varValues, opts, sc, hasOutput)
	}
	return streamFiles(files, code, varValues, opts, sc, hasOutput, cfg, threshold, filterExpr, argVars, argjsonVars)
}

func slurpFiles(files []string, code *gojq.Code, varValues []any, opts output.Options, sc *streamCfg, hasOutput *bool) int {
	all, rc := collectAllFileValues(files, code, sc)
	if rc != 0 {
		return rc
	}
	return executeFilter(code, all, varValues, opts, hasOutput)
}

func collectAllFileValues(files []string, code *gojq.Code, sc *streamCfg) ([]any, int) {
	var all []any
	for _, f := range files {
		vals, rc := slurpFileValues(f, code, sc)
		if rc != 0 {
			return nil, rc
		}
		all = append(all, vals...)
	}
	return all, 0
}

// streamContext groups parameters for streaming file processing.
type streamContext struct {
	code      *gojq.Code
	varValues []any
	opts      output.Options
	sc        *streamCfg
	hasOutput *bool
	cfg       *config
	threshold int64
	filterExpr string
	argVars    []keyValue
	argjsonVars []keyValue
}

func streamFiles(files []string, code *gojq.Code, varValues []any, opts output.Options, sc *streamCfg, hasOutput *bool, cfg *config, threshold int64, filterExpr string, argVars, argjsonVars []keyValue) int {
	ctx := streamContext{code, varValues, opts, sc, hasOutput, cfg, threshold, filterExpr, argVars, argjsonVars}
	return ctx.processAll(files)
}

func (ctx *streamContext) processAll(files []string) int {
	acc := 0
	for _, f := range files {
		rc := ctx.processFile(f)
		if rc == exitUsage {
			return rc
		}
		acc = accumulateRC(acc, rc)
	}
	return acc
}

func (ctx *streamContext) processFile(f string) int {
	return processOneFile(f, ctx.code, ctx.varValues, ctx.opts, ctx.hasOutput, ctx.sc, ctx.cfg, ctx.threshold, ctx.filterExpr, ctx.argVars, ctx.argjsonVars)
}

func accumulateRC(current, latest int) int {
	if latest != 0 {
		return latest
	}
	return current
}

func processOneFile(f string, code *gojq.Code, varValues []any, opts output.Options, hasOutput *bool, sc *streamCfg, cfg *config, threshold int64, filterExpr string, argVars, argjsonVars []keyValue) int {
	fileSC, src := resolveFileStream(f, sc, cfg, threshold, filterExpr, argVars, argjsonVars)
	if src != 0 {
		return src
	}
	return filterFile(f, code, varValues, opts, hasOutput, fileSC)
}

// resolveFileStream returns the stream config for a file, auto-detecting if needed.
func resolveFileStream(f string, sc *streamCfg, cfg *config, threshold int64, filterExpr string, argVars, argjsonVars []keyValue) (*streamCfg, int) {
	if sc != nil || cfg.noStream || !shouldAutoStream(f, threshold) {
		return sc, 0
	}
	return autoStream(f, cfg, threshold, filterExpr, argVars, argjsonVars)
}

func autoStream(f string, cfg *config, threshold int64, filterExpr string, argVars, argjsonVars []keyValue) (*streamCfg, int) {
	fileSC, src := buildStreamCfg(filterExpr, argVars, argjsonVars)
	if src != 0 {
		return nil, src
	}
	logAutoStream(cfg, f, threshold, filterExpr)
	return fileSC, 0
}

func logAutoStream(cfg *config, f string, threshold int64, filterExpr string) {
	if cfg.quiet {
		return
	}
	fmt.Fprintf(os.Stderr, "tq: info: streaming enabled for %s (file > %s)\n", fileLabel(f), formatSize(threshold))
	warnNonStreamable(filterExpr)
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
	result := 0
	for {
		v, rc, done := readNextValue(next, label)
		if done {
			return accumulateRC(result, rc)
		}
		result = accumulateRC(result, executeFilter(code, v, varValues, opts, hasOutput))
	}
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
		v, rc, done := readNextValue(next, label)
		if done {
			return vals, rc
		}
		vals = append(vals, v)
	}
}

func readNextValue(next valueIterator, label string) (any, int, bool) {
	v, ok, err := next.Next()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: %s: %v\n", label, err)
		return nil, exitUsage, true
	}
	if !ok {
		return nil, 0, true
	}
	return v, 0, false
}

// openFileReader opens a file or stdin ("-") and returns a reader and cleanup func.
func openFileReader(filename string) (io.Reader, func(), int) {
	if filename == "-" {
		return os.Stdin, func() {}, 0
	}
	return openNamedFile(filename)
}

func openNamedFile(filename string) (io.Reader, func(), int) {
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
	return drainFilterIter(iter, opts, hasOutput)
}

func drainFilterIter(iter gojq.Iter, opts output.Options, hasOutput *bool) int {
	for {
		v, ok := iter.Next()
		if !ok {
			return exitOK
		}
		if rc := emitValue(v, opts, hasOutput); rc != exitOK {
			return rc
		}
	}
}

func emitValue(v any, opts output.Options, hasOutput *bool) int {
	if err, isErr := v.(error); isErr {
		return reportError(err)
	}
	*hasOutput = true
	return writeOutput(v, opts)
}

func reportError(err error) int {
	fmt.Fprintf(os.Stderr, "tq: %v\n", err)
	return exitRuntime
}

func writeOutput(v any, opts output.Options) int {
	if err := output.Write(os.Stdout, v, opts); err != nil {
		fmt.Fprintf(os.Stderr, "tq: %v\n", err)
		return exitUsage
	}
	return exitOK
}

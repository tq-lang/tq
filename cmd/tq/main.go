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
	flag "github.com/spf13/pflag"
	"github.com/toon-format/toon-go"

	"github.com/tq-lang/tq/internal/detect"
	"github.com/tq-lang/tq/internal/input"
	"github.com/tq-lang/tq/internal/output"
)

// Exit codes matching jq conventions.
const (
	exitOK       = 0
	exitUsage    = 2 // bad flags, I/O errors
	exitCompile  = 3 // filter parse/compile error
	exitNoOutput = 4 // --exit-status with no output
	exitRuntime  = 5 // jq filter runtime error
)

var version = "dev"

func main() {
	os.Exit(run())
}

// config holds parsed CLI flags.
type config struct {
	jsonOutput bool
	toonOutput bool
	rawOutput  bool
	compact    bool
	slurp      bool
	nullInput  bool
	joinOutput bool
	tab        bool
	indent     int
	exitStatus bool
	stream     bool
	delimiter  string
	fromFile   string
	version    bool
	argPairs   []string
	argjsonPairs []string
}

func parseFlags() (*config, []string) {
	cfg := &config{}

	flag.BoolVar(&cfg.jsonOutput, "json", false, "output JSON instead of TOON")
	flag.BoolVar(&cfg.toonOutput, "toon", false, "output TOON (default)")
	flag.BoolVarP(&cfg.rawOutput, "raw-output", "r", false, "output raw strings")
	flag.BoolVarP(&cfg.compact, "compact-output", "c", false, "compact output")
	flag.BoolVarP(&cfg.slurp, "slurp", "s", false, "read entire input into array")
	flag.BoolVarP(&cfg.nullInput, "null-input", "n", false, "run filter without reading input")
	flag.BoolVarP(&cfg.joinOutput, "join-output", "j", false, "no newline between outputs")
	flag.BoolVar(&cfg.tab, "tab", false, "use tab for indentation")
	flag.IntVar(&cfg.indent, "indent", 0, "number of spaces for indentation")
	flag.BoolVarP(&cfg.exitStatus, "exit-status", "e", false, "set exit code based on output")
	flag.BoolVar(&cfg.stream, "stream", false, "output path-value pairs for streaming")
	flag.StringVar(&cfg.delimiter, "delimiter", "", "TOON output delimiter: comma, tab, pipe")
	flag.StringVarP(&cfg.fromFile, "from-file", "f", "", "read filter from file")
	flag.BoolVar(&cfg.version, "version", false, "print version")
	flag.StringArrayVar(&cfg.argPairs, "arg", nil, "set variable: --arg name value")
	flag.StringArrayVar(&cfg.argjsonPairs, "argjson", nil, "set JSON variable: --argjson name value")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: tq [flags] <filter> [file...]

tq is a command-line TOON/JSON processor. Like jq, but for TOON.

Examples:
  echo '{"name":"Alice"}' | tq '.name'          # field access
  echo '{"a":1}' | tq --json '.'                # convert to JSON
  cat data.toon | tq '.users[] | .name'          # iterate array
  tq -n '1 + 1'                                  # null input
  tq '.key' file1.json file2.toon                # read from files
  echo '{"a":1}' | tq '.' -                      # explicit stdin

Flags:
`)
		flag.PrintDefaults()
	}

	flag.Parse()
	return cfg, flag.Args()
}

// resolveFilter determines the jq filter expression and remaining file args.
func resolveFilter(cfg *config, args []string) (filterExpr string, fileArgs []string, rc int) {
	filterExpr = "."
	fileArgs = args

	if cfg.fromFile != "" {
		data, err := os.ReadFile(cfg.fromFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tq: %v\n", err)
			return "", nil, exitUsage
		}
		filterExpr = string(data)
	} else if len(fileArgs) > 0 {
		filterExpr = fileArgs[0]
		fileArgs = fileArgs[1:]
	}

	return filterExpr, fileArgs, exitOK
}

// compileFilter parses and compiles a jq filter with bound variables.
func compileFilter(filterExpr string, args, argsJSON []keyValue) (*gojq.Code, []any, int) {
	query, err := gojq.Parse(filterExpr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: parse error: %v\n", err)
		return nil, nil, exitCompile
	}

	var compileOpts []gojq.CompilerOption
	var varNames []string
	var varValues []any
	for _, a := range args {
		varNames = append(varNames, "$"+a.name)
		varValues = append(varValues, a.value)
	}
	for _, a := range argsJSON {
		varNames = append(varNames, "$"+a.name)
		varValues = append(varValues, a.value)
	}
	if len(varNames) > 0 {
		compileOpts = append(compileOpts, gojq.WithVariables(varNames))
	}

	code, err := gojq.Compile(query, compileOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: compile error: %v\n", err)
		return nil, nil, exitCompile
	}
	return code, varValues, exitOK
}

// resolveDelimiter maps a delimiter flag string to a toon.Delimiter.
func resolveDelimiter(s string) (toon.Delimiter, int) {
	switch strings.ToLower(s) {
	case "tab":
		return toon.DelimiterTab, exitOK
	case "pipe":
		return toon.DelimiterPipe, exitOK
	case "comma", "":
		return toon.DelimiterComma, exitOK
	default:
		fmt.Fprintf(os.Stderr, "tq: unknown delimiter %q (use comma, tab, or pipe)\n", s)
		return 0, exitUsage
	}
}

// streamCfg holds pre-compiled filter state for --stream mode.
// JSON inputs use TokenReader (O(depth) memory); TOON inputs fall back
// to the tostream-wrapped filter since TokenReader is JSON-only.
type streamCfg struct {
	code     *gojq.Code // plain filter (used for JSON TokenReader output)
	toonCode *gojq.Code // tostream | (filter) (used for TOON fallback)
}

func run() int {
	cfg, args := parseFlags()

	if cfg.version {
		fmt.Println("tq " + version)
		return exitOK
	}

	if cfg.jsonOutput && cfg.toonOutput {
		fmt.Fprintf(os.Stderr, "tq: --json and --toon are mutually exclusive\n")
		return exitUsage
	}

	argVars, rc := parseVarPairs(cfg.argPairs, "arg", false)
	if rc != 0 {
		return rc
	}
	argjsonVars, rc := parseVarPairs(cfg.argjsonPairs, "argjson", true)
	if rc != 0 {
		return rc
	}

	filterExpr, fileArgs, rc := resolveFilter(cfg, args)
	if rc != 0 {
		return rc
	}

	code, varValues, rc := compileFilter(filterExpr, argVars, argjsonVars)
	if rc != 0 {
		return rc
	}

	// When --stream is set, pre-compile a wrapped filter for TOON inputs.
	// JSON inputs use the TokenReader (O(depth) memory), but TOON inputs
	// still need the tostream | (...) approach since TokenReader is JSON-only.
	var sc *streamCfg
	if cfg.stream {
		toonExpr := "tostream | (" + filterExpr + ")"
		toonCode, _, trc := compileFilter(toonExpr, argVars, argjsonVars)
		if trc != 0 {
			return trc
		}
		sc = &streamCfg{code: code, toonCode: toonCode}
	}

	delim, rc := resolveDelimiter(cfg.delimiter)
	if rc != 0 {
		return rc
	}

	opts := output.Options{
		JSON:      cfg.jsonOutput,
		Raw:       cfg.rawOutput,
		Compact:   cfg.compact,
		Tab:       cfg.tab,
		Indent:    cfg.indent,
		Join:      cfg.joinOutput,
		Delimiter: delim,
	}

	hasOutput := false
	var exitCode int

	if cfg.nullInput {
		exitCode = executeFilter(code, nil, varValues, opts, &hasOutput)
	} else if len(fileArgs) > 0 {
		exitCode = processFiles(fileArgs, code, varValues, opts, cfg.slurp, sc, &hasOutput)
	} else {
		exitCode = processStream(os.Stdin, code, varValues, opts, cfg.slurp, sc, &hasOutput)
	}

	if cfg.exitStatus && !hasOutput && exitCode == exitOK {
		return exitNoOutput
	}

	return exitCode
}

// processStream reads values from a stream and executes the filter on each.
// With slurp, all values are collected into an array before filtering.
func processStream(r io.Reader, code *gojq.Code, varValues []any, opts output.Options, slurp bool, sc *streamCfg, hasOutput *bool) int {
	if slurp {
		return slurpAll(r, "stdin", code, varValues, opts, sc, hasOutput)
	}
	return filterAll(r, "stdin", code, varValues, opts, hasOutput, sc)
}

// processFiles reads each file (or "-" for stdin) as a streaming source.
func processFiles(files []string, code *gojq.Code, varValues []any, opts output.Options, slurp bool, sc *streamCfg, hasOutput *bool) int {
	if slurp {
		var all []any
		for _, f := range files {
			vals, rc := slurpFileValues(f, sc)
			if rc != 0 {
				return rc
			}
			all = append(all, vals...)
		}
		return executeFilter(code, all, varValues, opts, hasOutput)
	}

	exitCode := 0
	for _, f := range files {
		rc := filterFile(f, code, varValues, opts, hasOutput, sc)
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
// In stream mode, JSON inputs produce [path,value] pairs via TokenReader;
// TOON inputs produce full docs (the caller applies toonCode via slurpAll).
func slurpFileValues(filename string, sc *streamCfg) ([]any, int) {
	r, cleanup, rc := openFileReader(filename)
	if rc != 0 {
		return nil, rc
	}
	defer cleanup()
	return collectValues(r, fileLabel(filename), sc)
}

// filterAll reads values from r and runs the filter on each.
// When sc is non-nil (--stream) and input is JSON, uses TokenReader for O(depth) memory.
// For TOON stream mode, falls back to the pre-compiled toonCode with tostream wrapping.
func filterAll(r io.Reader, label string, code *gojq.Code, varValues []any, opts output.Options, hasOutput *bool, sc *streamCfg) int {
	if sc == nil {
		return filterLoop(input.NewReader(r), label, code, varValues, opts, hasOutput)
	}
	return filterAllStream(r, label, code, varValues, opts, hasOutput, sc)
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

// filterAllStream uses format detection to choose between TokenReader (JSON)
// and tostream-wrapped filter (TOON) for --stream mode.
func filterAllStream(r io.Reader, label string, code *gojq.Code, varValues []any, opts output.Options, hasOutput *bool, sc *streamCfg) int {
	br := bufio.NewReader(r)

	if !isConfirmedJSON(br) {
		// TOON path: read full values via TOON reader, apply tostream-wrapped filter.
		return filterLoop(input.NewTOONReader(br), label, sc.toonCode, varValues, opts, hasOutput)
	}

	// Confirmed JSON — stream via TokenReader for O(depth) memory.
	return filterLoop(input.NewTokenReader(br), label, code, varValues, opts, hasOutput)
}

// slurpAll collects all values from r into an array, then runs the filter once.
// In stream mode, JSON inputs are collected as [path,value] pairs via TokenReader;
// TOON inputs are collected as full docs with the tostream-wrapped filter applied.
func slurpAll(r io.Reader, label string, code *gojq.Code, varValues []any, opts output.Options, sc *streamCfg, hasOutput *bool) int {
	vals, rc := collectValues(r, label, sc)
	if rc != 0 {
		return rc
	}
	filterCode := code
	if sc != nil && !isSliceOfPairs(vals) {
		// TOON slurp: vals are full documents, use tostream-wrapped filter.
		filterCode = sc.toonCode
	}
	return executeFilter(filterCode, vals, varValues, opts, hasOutput)
}

// isSliceOfPairs returns true if vals contains [path,value] stream pairs
// (i.e. JSON stream mode produced them). Returns false for full TOON docs.
func isSliceOfPairs(vals []any) bool {
	if len(vals) == 0 {
		return false
	}
	_, ok := vals[0].([]any)
	return ok
}

// collectValues reads values from r into a slice. When sc is non-nil and the
// input is confirmed JSON, it uses TokenReader to emit [path,value] pairs.
// Otherwise it reads full values via Reader.
func collectValues(r io.Reader, label string, sc *streamCfg) ([]any, int) {
	if sc != nil {
		return collectStreamValues(r, label)
	}
	return collectReaderValues(input.NewReader(r), label)
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

func collectStreamValues(r io.Reader, label string) ([]any, int) {
	br := bufio.NewReader(r)

	if !isConfirmedJSON(br) {
		// TOON: collect full values; caller will use toonCode.
		return collectReaderValues(input.NewTOONReader(br), label)
	}

	// Confirmed JSON: collect [path,value] pairs.
	return collectReaderValues(input.NewTokenReader(br), label)
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
	format := detect.DetectReader(br)
	if format == detect.TOON {
		return false
	}

	peeked, _ := br.Peek(br.Buffered())
	if len(peeked) == 0 {
		return true // empty input — TokenReader handles EOF gracefully
	}

	testDec := json.NewDecoder(bytes.NewReader(peeked))
	if _, err := testDec.Token(); err != nil {
		return false
	}

	// Check that the byte right after the decoded token is valid in a JSON
	// context. This catches "true_value:" where Token() happily returns true
	// but the trailing '_' proves it's not actually JSON.
	offset := int(testDec.InputOffset())
	rest := peeked[offset:]
	for _, b := range rest {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case ',', ':', '[', ']', '{', '}', '"',
			'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
			'-', 't', 'f', 'n':
			return true
		default:
			return false
		}
	}
	// All remaining peeked bytes were whitespace (or there were none after
	// the token). This is valid JSON.
	return true
}

// openFileReader opens a file or stdin ("-") and returns a reader and cleanup func.
func openFileReader(filename string) (io.Reader, func(), int) {
	if filename == "-" {
		return os.Stdin, func() {}, 0
	}
	fh, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: %v\n", err)
		return nil, nil, exitUsage
	}
	return fh, func() { fh.Close() }, 0
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

// keyValue holds a --arg or --argjson pair.
type keyValue struct {
	name  string
	value any
}

// parseVarPairs parses --arg/--argjson string array into key-value pairs.
// Each flag usage provides one token, so we expect pairs: name, value, name, value, ...
func parseVarPairs(pairs []string, flagName string, parseJSON bool) ([]keyValue, int) {
	if len(pairs)%2 != 0 {
		fmt.Fprintf(os.Stderr, "tq: --%s requires pairs of name and value\n", flagName)
		return nil, exitUsage
	}
	var result []keyValue
	for i := 0; i < len(pairs); i += 2 {
		name := pairs[i]
		rawValue := pairs[i+1]
		var value any = rawValue
		if parseJSON {
			if err := json.Unmarshal([]byte(rawValue), &value); err != nil {
				fmt.Fprintf(os.Stderr, "tq: --%s value for %q is not valid JSON: %v\n", flagName, name, err)
				return nil, exitUsage
			}
		}
		result = append(result, keyValue{name: name, value: value})
	}
	return result, 0
}

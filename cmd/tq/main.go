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
	jsonOutput      bool
	toonOutput      bool
	rawOutput       bool
	compact         bool
	slurp           bool
	nullInput       bool
	joinOutput      bool
	tab             bool
	indent          int
	exitStatus      bool
	stream          bool
	noStream        bool
	streamThreshold string
	quiet           bool
	delimiter       string
	fromFile        string
	version         bool
	argPairs        []string
	argjsonPairs    []string
}

func parseFlags() (*config, []string) {
	cfg := &config{}

	flag.BoolVar(&cfg.jsonOutput, "json", false, "")
	flag.BoolVar(&cfg.toonOutput, "toon", false, "")
	flag.BoolVarP(&cfg.rawOutput, "raw-output", "r", false, "")
	flag.BoolVarP(&cfg.compact, "compact-output", "c", false, "")
	flag.BoolVarP(&cfg.joinOutput, "join-output", "j", false, "")
	flag.BoolVar(&cfg.tab, "tab", false, "")
	flag.IntVar(&cfg.indent, "indent", 0, "")
	flag.StringVar(&cfg.delimiter, "delimiter", "", "")
	flag.BoolVarP(&cfg.slurp, "slurp", "s", false, "")
	flag.BoolVarP(&cfg.nullInput, "null-input", "n", false, "")
	flag.StringVarP(&cfg.fromFile, "from-file", "f", "", "")
	flag.StringArrayVar(&cfg.argPairs, "arg", nil, "")
	flag.StringArrayVar(&cfg.argjsonPairs, "argjson", nil, "")
	flag.BoolVar(&cfg.stream, "stream", false, "")
	flag.BoolVar(&cfg.noStream, "no-stream", false, "")
	flag.StringVar(&cfg.streamThreshold, "stream-threshold", "", "")
	flag.BoolVarP(&cfg.exitStatus, "exit-status", "e", false, "")
	flag.BoolVarP(&cfg.quiet, "quiet", "q", false, "")
	flag.BoolVar(&cfg.version, "version", false, "")

	flag.Usage = printUsage

	flag.Parse()
	return cfg, flag.Args()
}

func printUsage() {
	fmt.Fprint(os.Stderr, `Usage: tq [flags] <filter> [file...]

tq is a command-line TOON/JSON processor. Like jq, but for TOON.

Examples:
  echo '{"name":"Alice"}' | tq '.name'           # field access
  echo '{"a":1}' | tq --json '.'                 # convert to JSON
  cat data.toon | tq '.users[] | .name'           # iterate array
  tq -n '1 + 1'                                   # null input
  tq '.key' file1.json file2.toon                 # multiple files
  echo '[1,2,3]' | tq -s 'add'                    # slurp + reduce
  tq -f filter.jq data.json                       # filter from file
  tq --stream --json -c '.' large.toon            # stream large files

Output flags:
      --json                   output JSON instead of TOON
      --toon                   output TOON (default)
  -r, --raw-output             output raw strings without quotes
  -c, --compact-output         compact single-line output
  -j, --join-output            no newline between output values
      --tab                    indent with tabs
      --indent N               indent with N spaces (default 2)
      --delimiter TYPE         TOON array delimiter: comma, tab, pipe

Input flags:
  -s, --slurp                  read all inputs into an array
  -n, --null-input             run filter with null input
  -f, --from-file PATH         read filter from file
      --arg NAME VALUE         bind $NAME to string VALUE
      --argjson NAME VALUE     bind $NAME to parsed JSON VALUE

Streaming flags:
      --stream                 emit [path, value] pairs (O(depth) memory)
      --no-stream              disable auto-streaming for large files
      --stream-threshold SIZE  auto-stream file size (default 256MB)

Other flags:
  -e, --exit-status            exit 4 if filter produces no output
  -q, --quiet                  suppress info and warning messages
      --version                print version and exit
  -h, --help                   show this help

Environment:
  TQ_STREAM_THRESHOLD          auto-stream threshold (e.g. 512MB, 1GB)

Documentation: https://github.com/tq-lang/tq
`)
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

// streamCfg holds pre-compiled filter state for streaming mode.
// Both JSON and TOON use native TokenReaders with O(depth) memory.
type streamCfg struct {
	code *gojq.Code // filter applied to [path, value] pairs
}

// buildStreamCfg creates a streamCfg from the user's filter expression.
func buildStreamCfg(filterExpr string, argVars, argjsonVars []keyValue) (*streamCfg, int) {
	code, _, rc := compileFilter(filterExpr, argVars, argjsonVars)
	if rc != 0 {
		return nil, rc
	}
	return &streamCfg{code: code}, 0
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

	// Build streaming config when --stream is explicitly set.
	var sc *streamCfg
	if cfg.stream {
		var src int
		sc, src = buildStreamCfg(filterExpr, argVars, argjsonVars)
		if src != 0 {
			return src
		}
		if !cfg.quiet {
			warnNonStreamable(filterExpr)
		}
	}

	threshold := resolveStreamThreshold(cfg)

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
		exitCode = processFiles(fileArgs, code, varValues, opts, cfg.slurp, sc, &hasOutput, cfg, threshold, filterExpr, argVars, argjsonVars)
	} else if cfg.slurp {
		exitCode = slurpAll(os.Stdin, "stdin", code, varValues, opts, sc, &hasOutput)
	} else {
		exitCode = filterAll(os.Stdin, "stdin", code, varValues, opts, &hasOutput, sc)
	}

	if cfg.exitStatus && !hasOutput && exitCode == exitOK {
		return exitNoOutput
	}

	return exitCode
}

// resolveReader returns the appropriate value iterator and filter code.
// When sc is nil (no --stream), uses the standard Reader with code.
// When sc is set, detects JSON vs TOON and picks the right native tokenizer.
func resolveReader(r io.Reader, code *gojq.Code, sc *streamCfg) (valueIterator, *gojq.Code) {
	if sc == nil {
		return input.NewReader(r), code
	}
	br := bufio.NewReader(r)
	if !isConfirmedJSON(br) {
		return input.NewTOONTokenReader(br), sc.code
	}
	return input.NewTokenReader(br), sc.code
}

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

// shouldAutoStream returns true if filename refers to a file >= threshold bytes.
func shouldAutoStream(filename string, threshold int64) bool {
	if filename == "-" {
		return false
	}
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return info.Size() >= threshold
}

// resolveStreamThreshold determines the auto-stream threshold.
// Priority: --stream-threshold flag > TQ_STREAM_THRESHOLD env > 256MB default.
func resolveStreamThreshold(cfg *config) int64 {
	if cfg.streamThreshold != "" {
		if v, err := parseSize(cfg.streamThreshold); err == nil {
			return v
		}
		fmt.Fprintf(os.Stderr, "tq: warning: invalid --stream-threshold %q, using default\n", cfg.streamThreshold)
	}
	if env := os.Getenv("TQ_STREAM_THRESHOLD"); env != "" {
		if v, err := parseSize(env); err == nil {
			return v
		}
	}
	return 256 * 1024 * 1024 // 256MB
}

// parseSize parses a human-readable size string like "256MB", "1GB".
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)

	multipliers := []struct {
		suffix string
		mult   int64
	}{
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
		{"B", 1},
	}

	for _, m := range multipliers {
		if strings.HasSuffix(s, m.suffix) {
			numStr := strings.TrimSpace(s[:len(s)-len(m.suffix)])
			var n int64
			if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil {
				return 0, fmt.Errorf("invalid size: %s", s)
			}
			return n * m.mult, nil
		}
	}

	// Plain number = bytes.
	var n int64
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, fmt.Errorf("invalid size: %s", s)
	}
	return n, nil
}

// formatSize returns a human-readable size string.
func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%dGB", bytes/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%dMB", bytes/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%dKB", bytes/1024)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// warnNonStreamable prints a warning for filter builtins that may not work
// as expected in streaming mode where input is [path,value] pairs.
func warnNonStreamable(filterExpr string) {
	builtins := []string{
		"sort_by", "group_by", "unique_by", "min_by", "max_by",
		"sort", "unique", "reverse", "transpose", "flatten",
		"combinations", "walk",
	}
	for _, name := range builtins {
		if matchesBuiltin(filterExpr, name) {
			fmt.Fprintf(os.Stderr,
				"tq: warning: '%s' may not work as expected in streaming mode "+
					"(input is [path,value] pairs, not the full document)\n", name)
		}
	}
}

// matchesBuiltin checks if filterExpr contains name as a whole word.
func matchesBuiltin(filterExpr, name string) bool {
	idx := 0
	for {
		pos := strings.Index(filterExpr[idx:], name)
		if pos == -1 {
			return false
		}
		pos += idx
		before := pos - 1
		after := pos + len(name)
		beforeOK := before < 0 || !isIdentChar(filterExpr[before])
		afterOK := after >= len(filterExpr) || !isIdentChar(filterExpr[after])
		if beforeOK && afterOK {
			return true
		}
		idx = pos + len(name)
	}
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
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

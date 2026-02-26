package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/itchyny/gojq"
	flag "github.com/spf13/pflag"
	"github.com/toon-format/toon-go"

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

	// When --stream is set, wrap the filter to decompose input into path-value pairs.
	// The filter is parenthesized so compound expressions (e.g. "select(…) | …")
	// work correctly. Top-level definitions like "def f: …; f" may not compose.
	if cfg.stream {
		filterExpr = "tostream | (" + filterExpr + ")"
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
		exitCode = processFiles(fileArgs, code, varValues, opts, cfg.slurp, &hasOutput)
	} else {
		exitCode = processStream(os.Stdin, code, varValues, opts, cfg.slurp, &hasOutput)
	}

	if cfg.exitStatus && !hasOutput && exitCode == exitOK {
		return exitNoOutput
	}

	return exitCode
}

// processStream reads values from a stream and executes the filter on each.
// With slurp, all values are collected into an array before filtering.
func processStream(r io.Reader, code *gojq.Code, varValues []any, opts output.Options, slurp bool, hasOutput *bool) int {
	if slurp {
		vals, rc := slurpAll(r, "stdin")
		if rc != 0 {
			return rc
		}
		return executeFilter(code, vals, varValues, opts, hasOutput)
	}
	return filterAll(r, "stdin", code, varValues, opts, hasOutput)
}

// processFiles reads each file (or "-" for stdin) as a streaming source.
func processFiles(files []string, code *gojq.Code, varValues []any, opts output.Options, slurp bool, hasOutput *bool) int {
	if slurp {
		var all []any
		for _, f := range files {
			vals, rc := slurpFile(f)
			if rc != 0 {
				return rc
			}
			all = append(all, vals...)
		}
		return executeFilter(code, all, varValues, opts, hasOutput)
	}

	exitCode := 0
	for _, f := range files {
		rc := filterFile(f, code, varValues, opts, hasOutput)
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
func filterFile(filename string, code *gojq.Code, varValues []any, opts output.Options, hasOutput *bool) int {
	r, cleanup, rc := openFileReader(filename)
	if rc != 0 {
		return rc
	}
	defer cleanup()
	return filterAll(r, fileLabel(filename), code, varValues, opts, hasOutput)
}

// slurpFile reads all values from a single file into a slice.
func slurpFile(filename string) ([]any, int) {
	r, cleanup, rc := openFileReader(filename)
	if rc != 0 {
		return nil, rc
	}
	defer cleanup()
	return slurpAll(r, fileLabel(filename))
}

// filterAll reads values from r and runs the filter on each.
func filterAll(r io.Reader, label string, code *gojq.Code, varValues []any, opts output.Options, hasOutput *bool) int {
	sr := input.NewReader(r)
	exitCode := 0
	for {
		v, ok, err := sr.Next()
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

// slurpAll reads all values from r into a slice.
func slurpAll(r io.Reader, label string) ([]any, int) {
	sr := input.NewReader(r)
	var vals []any
	for {
		v, ok, err := sr.Next()
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

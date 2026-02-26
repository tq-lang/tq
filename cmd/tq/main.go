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

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	var (
		jsonOutput   bool
		toonOutput   bool
		rawOutput    bool
		compact      bool
		slurp        bool
		nullInput    bool
		joinOutput   bool
		tab          bool
		indent       int
		exitStatus   bool
		delimiter    string
		fromFile     string
		showVersion  bool
		argPairs     []string
		argjsonPairs []string
	)

	flag.BoolVar(&jsonOutput, "json", false, "output JSON instead of TOON")
	flag.BoolVar(&toonOutput, "toon", false, "output TOON (default)")
	flag.BoolVarP(&rawOutput, "raw-output", "r", false, "output raw strings")
	flag.BoolVarP(&compact, "compact-output", "c", false, "compact output")
	flag.BoolVarP(&slurp, "slurp", "s", false, "read entire input into array")
	flag.BoolVarP(&nullInput, "null-input", "n", false, "run filter without reading input")
	flag.BoolVarP(&joinOutput, "join-output", "j", false, "no newline between outputs")
	flag.BoolVar(&tab, "tab", false, "use tab for indentation")
	flag.IntVar(&indent, "indent", 0, "number of spaces for indentation")
	flag.BoolVarP(&exitStatus, "exit-status", "e", false, "set exit code based on output")
	flag.StringVar(&delimiter, "delimiter", "", "TOON output delimiter: comma, tab, pipe")
	flag.StringVarP(&fromFile, "from-file", "f", "", "read filter from file")
	flag.BoolVar(&showVersion, "version", false, "print version")
	flag.StringArrayVar(&argPairs, "arg", nil, "set variable: --arg name value")
	flag.StringArrayVar(&argjsonPairs, "argjson", nil, "set JSON variable: --argjson name value")

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

	if showVersion {
		fmt.Println("tq " + version)
		return 0
	}

	// Parse --arg pairs (expects even number: name value name value ...)
	args, code := parseVarPairs(argPairs, "arg", false)
	if code != 0 {
		return code
	}
	argsJSON, code := parseVarPairs(argjsonPairs, "argjson", true)
	if code != 0 {
		return code
	}

	// --toon is the default; --json overrides it
	if jsonOutput && toonOutput {
		fmt.Fprintf(os.Stderr, "tq: --json and --toon are mutually exclusive\n")
		return 2
	}

	// Determine the filter expression
	filterExpr := "."
	fileArgs := flag.Args()

	if fromFile != "" {
		data, err := os.ReadFile(fromFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tq: %v\n", err)
			return 2
		}
		filterExpr = string(data)
	} else if len(fileArgs) > 0 {
		filterExpr = fileArgs[0]
		fileArgs = fileArgs[1:]
	}

	// Parse the jq filter
	query, err := gojq.Parse(filterExpr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: parse error: %v\n", err)
		return 3
	}

	// Compile with variables
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

	compiledCode, err := gojq.Compile(query, compileOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: compile error: %v\n", err)
		return 3
	}

	// Resolve TOON delimiter
	var toonDelimiter toon.Delimiter
	switch strings.ToLower(delimiter) {
	case "tab":
		toonDelimiter = toon.DelimiterTab
	case "pipe":
		toonDelimiter = toon.DelimiterPipe
	case "comma", "":
		toonDelimiter = toon.DelimiterComma
	default:
		fmt.Fprintf(os.Stderr, "tq: unknown delimiter %q (use comma, tab, or pipe)\n", delimiter)
		return 2
	}

	opts := output.Options{
		JSON:      jsonOutput,
		Raw:       rawOutput,
		Compact:   compact,
		Tab:       tab,
		Indent:    indent,
		Join:      joinOutput,
		Delimiter: toonDelimiter,
	}

	// Read input
	var inputs []any
	if nullInput {
		inputs = []any{nil}
	} else if len(fileArgs) > 0 {
		for _, f := range fileArgs {
			if f == "-" {
				v, readErr := readStdin()
				if readErr != nil {
					fmt.Fprintf(os.Stderr, "tq: reading stdin: %v\n", readErr)
					return 2
				}
				inputs = append(inputs, v)
				continue
			}
			data, readErr := os.ReadFile(f)
			if readErr != nil {
				fmt.Fprintf(os.Stderr, "tq: %v\n", readErr)
				return 2
			}
			v, parseErr := input.Parse(data)
			if parseErr != nil {
				fmt.Fprintf(os.Stderr, "tq: %s: %v\n", f, parseErr)
				return 2
			}
			inputs = append(inputs, v)
		}
	} else {
		v, readErr := readStdin()
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "tq: reading stdin: %v\n", readErr)
			return 2
		}
		inputs = append(inputs, v)
	}

	if slurp {
		inputs = []any{inputs}
	}

	// Execute filter and write output
	hasOutput := false
	exitCode := 0
	for _, inp := range inputs {
		iter := compiledCode.Run(inp, varValues...)
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if err, isErr := v.(error); isErr {
				fmt.Fprintf(os.Stderr, "tq: %v\n", err)
				exitCode = 5
				break
			}
			hasOutput = true
			if err := output.Write(os.Stdout, v, opts); err != nil {
				fmt.Fprintf(os.Stderr, "tq: %v\n", err)
				return 2
			}
		}
	}

	if exitStatus {
		if !hasOutput {
			return 4
		}
	}

	return exitCode
}

func readStdin() (any, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}
	return input.Parse(data)
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
		return nil, 2
	}
	var result []keyValue
	for i := 0; i < len(pairs); i += 2 {
		name := pairs[i]
		rawValue := pairs[i+1]
		var value any = rawValue
		if parseJSON {
			if err := json.Unmarshal([]byte(rawValue), &value); err != nil {
				fmt.Fprintf(os.Stderr, "tq: --%s value for %q is not valid JSON: %v\n", flagName, name, err)
				return nil, 2
			}
		}
		result = append(result, keyValue{name: name, value: value})
	}
	return result, 0
}

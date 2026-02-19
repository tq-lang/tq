package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/itchyny/gojq"
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
		jsonOutput  bool
		rawOutput   bool
		compact     bool
		slurp       bool
		nullInput   bool
		joinOutput  bool
		tab         bool
		indent      int
		exitStatus  bool
		delimiter   string
		fromFile    string
		showVersion bool
		args        []keyValue
		argsJSON    []keyValue
	)

	flag.BoolVar(&jsonOutput, "json", false, "output JSON instead of TOON")
	flag.BoolVar(&rawOutput, "r", false, "output raw strings")
	flag.BoolVar(&rawOutput, "raw-output", false, "output raw strings")
	flag.BoolVar(&compact, "c", false, "compact output")
	flag.BoolVar(&compact, "compact-output", false, "compact output")
	flag.BoolVar(&slurp, "s", false, "read entire input into array")
	flag.BoolVar(&slurp, "slurp", false, "read entire input into array")
	flag.BoolVar(&nullInput, "n", false, "run filter without reading input")
	flag.BoolVar(&nullInput, "null-input", false, "run filter without reading input")
	flag.BoolVar(&joinOutput, "j", false, "no newline between outputs")
	flag.BoolVar(&joinOutput, "join-output", false, "no newline between outputs")
	flag.BoolVar(&tab, "tab", false, "use tab for indentation")
	flag.IntVar(&indent, "indent", 0, "number of spaces for indentation")
	flag.BoolVar(&exitStatus, "e", false, "set exit status based on output")
	flag.BoolVar(&exitStatus, "exit-status", false, "set exit status based on output")
	flag.StringVar(&delimiter, "delimiter", "", "TOON output delimiter: comma, tab, pipe")
	flag.StringVar(&fromFile, "f", "", "read filter from file")
	flag.StringVar(&fromFile, "from-file", "", "read filter from file")
	flag.BoolVar(&showVersion, "version", false, "print version")
	flag.Var(&argFlag{target: &args}, "arg", "set variable: --arg name value")
	flag.Var(&argFlag{target: &argsJSON}, "argjson", "set JSON variable: --argjson name value")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tq [flags] <filter> [file...]\n\n")
		fmt.Fprintf(os.Stderr, "tq is a command-line TOON/JSON processor. Like jq, but for TOON.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if showVersion {
		fmt.Println("tq " + version)
		return 0
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

	code, err := gojq.Compile(query, compileOpts...)
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
			data, err := os.ReadFile(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "tq: %v\n", err)
				return 2
			}
			v, err := input.Parse(data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "tq: %s: %v\n", f, err)
				return 2
			}
			inputs = append(inputs, v)
		}
	} else {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tq: reading stdin: %v\n", err)
			return 2
		}
		v, err := input.Parse(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tq: %v\n", err)
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
		iter := code.Run(inp, varValues...)
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

// keyValue holds a --arg or --argjson pair.
type keyValue struct {
	name  string
	value any
}

// argFlag implements flag.Value for --arg/--argjson pairs.
type argFlag struct {
	target *[]keyValue
}

func (f *argFlag) String() string { return "" }

func (f *argFlag) Set(s string) error {
	// flag package calls Set once per flag occurrence.
	// We expect two consecutive values: name then value.
	// Store name first, then on second call store value.
	if len(*f.target) > 0 {
		last := &(*f.target)[len(*f.target)-1]
		if last.value == nil {
			last.value = s
			return nil
		}
	}
	*f.target = append(*f.target, keyValue{name: s})
	return nil
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/toon-format/toon-go"
)

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

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
func resolveFilter(cfg *config, args []string) (string, []string, int) {
	if cfg.fromFile != "" {
		return resolveFilterFromFile(cfg.fromFile, args)
	}
	return resolveFilterFromArgs(args)
}

func resolveFilterFromFile(path string, args []string) (string, []string, int) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: %v\n", err)
		return "", nil, exitUsage
	}
	return string(data), args, exitOK
}

func resolveFilterFromArgs(args []string) (string, []string, int) {
	if len(args) > 0 {
		return args[0], args[1:], exitOK
	}
	return ".", args, exitOK
}

// compileFilter parses and compiles a jq filter with bound variables.
func compileFilter(filterExpr string, args, argsJSON []keyValue) (*gojq.Code, []any, int) {
	query, err := gojq.Parse(filterExpr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: parse error: %v\n", err)
		return nil, nil, exitCompile
	}
	return compileQuery(query, args, argsJSON)
}

func compileQuery(query *gojq.Query, args, argsJSON []keyValue) (*gojq.Code, []any, int) {
	varNames, varValues := collectVars(args, argsJSON)
	code, rc := doCompile(query, varNames)
	if rc != 0 {
		return nil, nil, rc
	}
	return code, varValues, exitOK
}

func doCompile(query *gojq.Query, varNames []string) (*gojq.Code, int) {
	opts := varCompilerOpts(varNames)
	code, err := gojq.Compile(query, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: compile error: %v\n", err)
		return nil, exitCompile
	}
	return code, exitOK
}

func varCompilerOpts(varNames []string) []gojq.CompilerOption {
	if len(varNames) == 0 {
		return nil
	}
	return []gojq.CompilerOption{gojq.WithVariables(varNames)}
}

func collectVars(args, argsJSON []keyValue) ([]string, []any) {
	all := append(args, argsJSON...)
	return extractNamesValues(all)
}

func extractNamesValues(kvs []keyValue) ([]string, []any) {
	names := make([]string, 0, len(kvs))
	values := make([]any, 0, len(kvs))
	for _, a := range kvs {
		names = append(names, "$"+a.name)
		values = append(values, a.value)
	}
	return names, values
}

var delimiterNames = map[string]toon.Delimiter{
	"tab":   toon.DelimiterTab,
	"pipe":  toon.DelimiterPipe,
	"comma": toon.DelimiterComma,
	"":      toon.DelimiterComma,
}

// resolveDelimiter maps a delimiter flag string to a toon.Delimiter.
func resolveDelimiter(s string) (toon.Delimiter, int) {
	if d, ok := delimiterNames[strings.ToLower(s)]; ok {
		return d, exitOK
	}
	fmt.Fprintf(os.Stderr, "tq: unknown delimiter %q (use comma, tab, or pipe)\n", s)
	return 0, exitUsage
}

// keyValue holds a --arg or --argjson pair.
type keyValue struct {
	name  string
	value any
}

// parseVarPairs parses --arg/--argjson string array into key-value pairs.
// Each flag usage provides one token, so we expect pairs: name, value, name, value, ...
func parseVarPairs(pairs []string, flagName string, isJSON bool) ([]keyValue, int) {
	if len(pairs)%2 != 0 {
		fmt.Fprintf(os.Stderr, "tq: --%s requires pairs of name and value\n", flagName)
		return nil, exitUsage
	}
	return buildVarPairs(pairs, flagName, isJSON)
}

func buildVarPairs(pairs []string, flagName string, isJSON bool) ([]keyValue, int) {
	result := make([]keyValue, 0, len(pairs)/2)
	return appendVarPairs(result, pairs, flagName, isJSON)
}

func appendVarPairs(result []keyValue, pairs []string, flagName string, isJSON bool) ([]keyValue, int) {
	for i := 0; i < len(pairs); i += 2 {
		kv, rc := parseOneVarPair(pairs[i], pairs[i+1], flagName, isJSON)
		if rc != 0 {
			return nil, rc
		}
		result = append(result, kv)
	}
	return result, 0
}

func parseOneVarPair(name, rawValue, flagName string, isJSON bool) (keyValue, int) {
	var value any = rawValue
	if isJSON {
		if err := json.Unmarshal([]byte(rawValue), &value); err != nil {
			fmt.Fprintf(os.Stderr, "tq: --%s value for %q is not valid JSON: %v\n", flagName, name, err)
			return keyValue{}, exitUsage
		}
	}
	return keyValue{name: name, value: value}, 0
}

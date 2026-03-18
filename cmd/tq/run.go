package main

import (
	"fmt"
	"os"

	"github.com/tq-lang/tq/internal/output"
)

func run() int {
	cfg, args := parseFlags()

	if cfg.version {
		if commit != "unknown" || date != "unknown" {
			fmt.Printf("tq %s (%s, %s)\n", version, commit, date)
		} else {
			fmt.Println("tq " + version)
		}
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

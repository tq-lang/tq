package main

import (
	"fmt"
	"os"

	"github.com/itchyny/gojq"

	"github.com/tq-lang/tq/internal/output"
)

func run() int {
	cfg, args := parseFlags()

	if cfg.version {
		printVersion()
		return exitOK
	}

	if cfg.jsonOutput && cfg.toonOutput {
		fmt.Fprintf(os.Stderr, "tq: --json and --toon are mutually exclusive\n")
		return exitUsage
	}

	env, rc := buildRunEnv(cfg, args)
	if rc != 0 {
		return rc
	}

	exitCode := execute(cfg, env)

	if cfg.exitStatus && !env.hasOutput && exitCode == exitOK {
		return exitNoOutput
	}
	return exitCode
}

func printVersion() {
	if commit != "unknown" && date != "unknown" {
		_, _ = fmt.Fprintf(os.Stdout, "tq %s (%s, %s)\n", version, commit, date)
	} else {
		_, _ = fmt.Fprintln(os.Stdout, "tq "+version)
	}
}

// runEnv holds resolved runtime state needed for filter execution.
type runEnv struct {
	filterExpr  string
	fileArgs    []string
	argVars     []keyValue
	argjsonVars []keyValue
	code        *gojq.Code
	varValues   []any
	sc          *streamCfg
	threshold   int64
	opts        output.Options
	hasOutput   bool
}

func buildRunEnv(cfg *config, args []string) (*runEnv, int) {
	argVars, rc := parseVarPairs(cfg.argPairs, "arg", false)
	if rc != 0 {
		return nil, rc
	}
	argjsonVars, rc := parseVarPairs(cfg.argjsonPairs, "argjson", true)
	if rc != 0 {
		return nil, rc
	}

	filterExpr, fileArgs, rc := resolveFilter(cfg, args)
	if rc != 0 {
		return nil, rc
	}

	code, varValues, rc := compileFilter(filterExpr, argVars, argjsonVars)
	if rc != 0 {
		return nil, rc
	}

	var sc *streamCfg
	if cfg.stream {
		var src int
		sc, src = buildStreamCfg(filterExpr, argVars, argjsonVars)
		if src != 0 {
			return nil, src
		}
		if !cfg.quiet {
			warnNonStreamable(filterExpr)
		}
	}

	delim, rc := resolveDelimiter(cfg.delimiter)
	if rc != 0 {
		return nil, rc
	}

	return &runEnv{
		filterExpr:  filterExpr,
		fileArgs:    fileArgs,
		argVars:     argVars,
		argjsonVars: argjsonVars,
		code:        code,
		varValues:   varValues,
		sc:          sc,
		threshold:   resolveStreamThreshold(cfg),
		opts: output.Options{
			JSON:      cfg.jsonOutput,
			Raw:       cfg.rawOutput,
			Compact:   cfg.compact,
			Tab:       cfg.tab,
			Indent:    cfg.indent,
			Join:      cfg.joinOutput,
			Delimiter: delim,
		},
	}, 0
}

func execute(cfg *config, env *runEnv) int {
	if cfg.nullInput {
		return executeFilter(env.code, nil, env.varValues, env.opts, &env.hasOutput)
	}
	if len(env.fileArgs) > 0 {
		return processFiles(env.fileArgs, env.code, env.varValues, env.opts, cfg.slurp, env.sc, &env.hasOutput, cfg, env.threshold, env.filterExpr, env.argVars, env.argjsonVars)
	}
	if cfg.slurp {
		return slurpAll(os.Stdin, "stdin", env.code, env.varValues, env.opts, env.sc, &env.hasOutput)
	}
	return filterAll(os.Stdin, "stdin", env.code, env.varValues, env.opts, &env.hasOutput, env.sc)
}

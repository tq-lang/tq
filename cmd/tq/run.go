package main

import (
	"fmt"
	"os"

	"github.com/itchyny/gojq"
	"github.com/toon-format/toon-go"

	"github.com/tq-lang/tq/internal/output"
)

func run() int {
	cfg, args := parseFlags()
	if rc, done := handleEarlyFlags(cfg); done {
		return rc
	}
	env, rc := buildRunEnv(cfg, args)
	if rc != 0 {
		return rc
	}
	return runWithEnv(cfg, env)
}

func handleEarlyFlags(cfg *config) (int, bool) {
	if cfg.version {
		printVersion()
		return exitOK, true
	}
	if cfg.jsonOutput && cfg.toonOutput {
		fmt.Fprintf(os.Stderr, "tq: --json and --toon are mutually exclusive\n")
		return exitUsage, true
	}
	return 0, false
}

func runWithEnv(cfg *config, env *runEnv) int {
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
	argVars, argjsonVars, rc := parseAllVarPairs(cfg)
	if rc != 0 {
		return nil, rc
	}
	filterExpr, fileArgs, rc := resolveFilter(cfg, args)
	if rc != 0 {
		return nil, rc
	}
	return assembleRunEnv(cfg, filterExpr, fileArgs, argVars, argjsonVars)
}

func parseAllVarPairs(cfg *config) ([]keyValue, []keyValue, int) {
	argVars, rc := parseVarPairs(cfg.argPairs, "arg", false)
	if rc != 0 {
		return nil, nil, rc
	}
	argjsonVars, rc := parseVarPairs(cfg.argjsonPairs, "argjson", true)
	if rc != 0 {
		return nil, nil, rc
	}
	return argVars, argjsonVars, 0
}

func assembleRunEnv(cfg *config, filterExpr string, fileArgs []string, argVars, argjsonVars []keyValue) (*runEnv, int) {
	env := newRunEnv(cfg, filterExpr, fileArgs, argVars, argjsonVars)
	if rc := env.compile(cfg, filterExpr, argVars, argjsonVars); rc != 0 {
		return nil, rc
	}
	return env, 0
}

func newRunEnv(cfg *config, filterExpr string, fileArgs []string, argVars, argjsonVars []keyValue) *runEnv {
	return &runEnv{
		filterExpr: filterExpr, fileArgs: fileArgs,
		argVars: argVars, argjsonVars: argjsonVars,
		threshold: resolveStreamThreshold(cfg),
	}
}

func (env *runEnv) compile(cfg *config, filterExpr string, argVars, argjsonVars []keyValue) int {
	code, varValues, rc := compileFilter(filterExpr, argVars, argjsonVars)
	if rc != 0 {
		return rc
	}
	env.code = code
	env.varValues = varValues
	return env.resolveStreamAndOpts(cfg, filterExpr, argVars, argjsonVars)
}

func (env *runEnv) resolveStreamAndOpts(cfg *config, filterExpr string, argVars, argjsonVars []keyValue) int {
	if rc := env.setStream(cfg, filterExpr, argVars, argjsonVars); rc != 0 {
		return rc
	}
	return env.setOutputOpts(cfg)
}

func (env *runEnv) setStream(cfg *config, filterExpr string, argVars, argjsonVars []keyValue) int {
	sc, rc := resolveStreamCfg(cfg, filterExpr, argVars, argjsonVars)
	if rc != 0 {
		return rc
	}
	env.sc = sc
	return 0
}

func (env *runEnv) setOutputOpts(cfg *config) int {
	opts, rc := buildOutputOpts(cfg)
	if rc != 0 {
		return rc
	}
	env.opts = opts
	return 0
}

func resolveStreamCfg(cfg *config, filterExpr string, argVars, argjsonVars []keyValue) (*streamCfg, int) {
	if !cfg.stream {
		return nil, 0
	}
	return buildAndWarnStream(cfg, filterExpr, argVars, argjsonVars)
}

func buildAndWarnStream(cfg *config, filterExpr string, argVars, argjsonVars []keyValue) (*streamCfg, int) {
	sc, src := buildStreamCfg(filterExpr, argVars, argjsonVars)
	if src != 0 {
		return nil, src
	}
	if !cfg.quiet {
		warnNonStreamable(filterExpr)
	}
	return sc, 0
}

func buildOutputOpts(cfg *config) (output.Options, int) {
	delim, rc := resolveDelimiter(cfg.delimiter)
	if rc != 0 {
		return output.Options{}, rc
	}
	return newOutputOpts(cfg, delim), 0
}

func newOutputOpts(cfg *config, delim toon.Delimiter) output.Options {
	return output.Options{
		JSON: cfg.jsonOutput, Raw: cfg.rawOutput,
		Compact: cfg.compact, Tab: cfg.tab,
		Indent: cfg.indent, Join: cfg.joinOutput,
		Delimiter: delim,
	}
}

func execute(cfg *config, env *runEnv) int {
	if cfg.nullInput {
		return executeFilter(env.code, nil, env.varValues, env.opts, &env.hasOutput)
	}
	if len(env.fileArgs) > 0 {
		return executeFiles(cfg, env)
	}
	return executeStdin(cfg, env)
}

func executeFiles(cfg *config, env *runEnv) int {
	return processFiles(env.fileArgs, env.code, env.varValues, env.opts, cfg.slurp, env.sc, &env.hasOutput, cfg, env.threshold, env.filterExpr, env.argVars, env.argjsonVars)
}

func executeStdin(cfg *config, env *runEnv) int {
	if cfg.slurp {
		return slurpAll(os.Stdin, "stdin", env.code, env.varValues, env.opts, env.sc, &env.hasOutput)
	}
	return filterAll(os.Stdin, "stdin", env.code, env.varValues, env.opts, &env.hasOutput, env.sc)
}

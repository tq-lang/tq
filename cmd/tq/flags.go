package main

import (
	"fmt"
	"os"
	"strings"

	flag "github.com/spf13/pflag"
)

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

type varArgExtractor struct {
	remaining    []string
	argPairs     []string
	argjsonPairs []string
}

// extractVarArgs scans args for --arg/--argjson NAME VALUE (jq-style two-positional-arg syntax),
// collects the pairs, and returns the remaining args for pflag.
func extractVarArgs(args []string) (remaining, argPairs, argjsonPairs []string, err error) {
	var ext varArgExtractor
	if err := ext.extract(args); err != nil {
		return nil, nil, nil, err
	}
	return ext.remaining, ext.argPairs, ext.argjsonPairs, nil
}

func (e *varArgExtractor) extract(args []string) error {
	for i := 0; i < len(args); i++ {
		n, stop, err := e.step(args, i)
		if err != nil || stop {
			return err
		}
		i += n
	}
	return nil
}

func (e *varArgExtractor) step(args []string, i int) (int, bool, error) {
	if args[i] == "--" {
		e.remaining = append(e.remaining, args[i:]...)
		return 0, true, nil
	}
	n, err := e.processArg(args, i)
	return n, false, err
}

func (e *varArgExtractor) processArg(args []string, i int) (int, error) {
	a := args[i]
	if !isVarFlag(a) {
		e.remaining = append(e.remaining, a)
		return 0, nil
	}
	return e.consumeVarPair(args, i, a)
}

func isVarFlag(a string) bool {
	return a == "--arg" || a == "--argjson"
}

func (e *varArgExtractor) consumeVarPair(args []string, i int, a string) (int, error) {
	if err := checkVarPairArgs(args, i, a); err != nil {
		return 0, err
	}
	e.appendVarPair(a, args[i+1], args[i+2])
	return 2, nil
}

func checkVarPairArgs(args []string, i int, a string) error {
	if i+2 >= len(args) {
		return fmt.Errorf("tq: --%s requires NAME and VALUE arguments", strings.TrimPrefix(a, "--"))
	}
	return nil
}

func (e *varArgExtractor) appendVarPair(flagName, name, value string) {
	if flagName == "--arg" {
		e.argPairs = append(e.argPairs, name, value)
	} else {
		e.argjsonPairs = append(e.argjsonPairs, name, value)
	}
}

func parseFlags() (*config, []string) {
	cfg := &config{}
	registerOutputFlags(cfg)
	registerInputFlags(cfg)
	registerStreamFlags(cfg)
	registerOtherFlags(cfg)
	flag.Usage = printUsage
	return parseFlagArgs(cfg)
}

func registerOutputFlags(cfg *config) {
	registerFormatFlags(cfg)
	flag.BoolVar(&cfg.tab, "tab", false, "")
	flag.IntVar(&cfg.indent, "indent", 0, "")
	flag.StringVar(&cfg.delimiter, "delimiter", "", "")
}

func registerFormatFlags(cfg *config) {
	flag.BoolVar(&cfg.jsonOutput, "json", false, "")
	flag.BoolVar(&cfg.toonOutput, "toon", false, "")
	flag.BoolVarP(&cfg.rawOutput, "raw-output", "r", false, "")
	flag.BoolVarP(&cfg.compact, "compact-output", "c", false, "")
	flag.BoolVarP(&cfg.joinOutput, "join-output", "j", false, "")
}

func registerInputFlags(cfg *config) {
	flag.BoolVarP(&cfg.slurp, "slurp", "s", false, "")
	flag.BoolVarP(&cfg.nullInput, "null-input", "n", false, "")
	flag.StringVarP(&cfg.fromFile, "from-file", "f", "", "")
}

func registerStreamFlags(cfg *config) {
	flag.BoolVar(&cfg.stream, "stream", false, "")
	flag.BoolVar(&cfg.noStream, "no-stream", false, "")
	flag.StringVar(&cfg.streamThreshold, "stream-threshold", "", "")
}

func registerOtherFlags(cfg *config) {
	flag.BoolVarP(&cfg.exitStatus, "exit-status", "e", false, "")
	flag.BoolVarP(&cfg.quiet, "quiet", "q", false, "")
	flag.BoolVar(&cfg.version, "version", false, "")
}

func parseFlagArgs(cfg *config) (*config, []string) {
	remaining := extractAndSetVarArgs(cfg)
	_ = flag.CommandLine.Parse(remaining)
	return cfg, flag.Args()
}

func extractAndSetVarArgs(cfg *config) []string {
	remaining, argPairs, argjsonPairs, err := extractVarArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitUsage)
	}
	cfg.argPairs = argPairs
	cfg.argjsonPairs = argjsonPairs
	return remaining
}

const usageText = `Usage: tq [flags] <filter> [file...]

tq is a command-line TOON/JSON processor. Like jq, but for TOON.

Examples:
  echo 'name: Alice' | tq '.name'                 # field access
  echo 'a: 1' | tq --json '.'                     # convert to JSON
  cat data.toon | tq '.users[] | .name'            # iterate array
  tq -n '1 + 1'                                    # null input
  tq '.key' file1.json file2.toon                  # multiple files
  echo '1 2 3' | tq -s 'add'                       # slurp + reduce
  tq -f filter.jq data.json                        # filter from file
  tq --stream --json -c '.' large.toon             # stream large files

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
`

func printUsage() {
	_, _ = fmt.Fprint(os.Stdout, usageText)
}

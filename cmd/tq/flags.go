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

// extractVarArgs scans args for --arg/--argjson NAME VALUE (jq-style two-positional-arg syntax),
// collects the pairs, and returns the remaining args for pflag.
func extractVarArgs(args []string) (remaining, argPairs, argjsonPairs []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			remaining = append(remaining, args[i:]...)
			break
		}
		if a == "--arg" || a == "--argjson" {
			if i+2 >= len(args) {
				return nil, nil, nil, fmt.Errorf("tq: --%s requires NAME and VALUE arguments", strings.TrimPrefix(a, "--"))
			}
			name := args[i+1]
			value := args[i+2]
			if a == "--arg" {
				argPairs = append(argPairs, name, value)
			} else {
				argjsonPairs = append(argjsonPairs, name, value)
			}
			i += 2
			continue
		}
		remaining = append(remaining, a)
	}
	return remaining, argPairs, argjsonPairs, nil
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
	flag.BoolVar(&cfg.stream, "stream", false, "")
	flag.BoolVar(&cfg.noStream, "no-stream", false, "")
	flag.StringVar(&cfg.streamThreshold, "stream-threshold", "", "")
	flag.BoolVarP(&cfg.exitStatus, "exit-status", "e", false, "")
	flag.BoolVarP(&cfg.quiet, "quiet", "q", false, "")
	flag.BoolVar(&cfg.version, "version", false, "")

	flag.Usage = printUsage

	// Extract --arg/--argjson before pflag parsing (jq-style two-arg syntax).
	remaining, argPairs, argjsonPairs, err := extractVarArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitUsage)
	}
	cfg.argPairs = argPairs
	cfg.argjsonPairs = argjsonPairs

	_ = flag.CommandLine.Parse(remaining)
	return cfg, flag.Args()
}

func printUsage() {
	_, _ = fmt.Fprint(os.Stdout, `Usage: tq [flags] <filter> [file...]

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
`)
}

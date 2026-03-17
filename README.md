# tq

**tq** is a command-line processor for [TOON](https://github.com/toon-format/toon) data — the same way [jq](https://github.com/jqlang/jq) works for JSON.

TOON (Token Oriented Object Notation) and JSON share the **exact same data model**: strings, numbers, booleans, null, arrays, and objects. TOON simply uses a more compact, token-efficient serialization. Because the data model is identical, jq's query language maps 1:1 — so if you know jq, you already know tq.

## Quick Start

```bash
# Query TOON data
echo 'name: Alice
age: 30' | tq '.name'
# Alice

# Query JSON data (auto-detected)
echo '{"name":"Alice","age":30}' | tq '.name'
# Alice

# Convert JSON to TOON
echo '{"users":[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]}' | tq '.'
# users[2]{id,name}:
#   1,Alice
#   2,Bob

# Convert TOON to JSON
echo 'name: Alice
age: 30' | tq --json '.'
# {"age":30,"name":"Alice"}

# Complex filters work just like jq
echo '{"users":[{"id":1,"name":"Alice","active":true},{"id":2,"name":"Bob","active":false}]}' \
  | tq '.users[] | select(.active) | .name'
# Alice
```

## Installation

### Homebrew

```bash
brew install tq-lang/tap/tq
```

### From source

```bash
go install github.com/tq-lang/tq/cmd/tq@latest
```

### Build from repo

```bash
make build                    # builds ./tq with version "dev"
make build VERSION=1.2.3      # builds ./tq with custom version
```

### From releases

Download a prebuilt binary from the [Releases](https://github.com/tq-lang/tq/releases) page.
See [docs/release-verification.md](docs/release-verification.md) for SBOM and provenance verification.

## Usage

```
tq [flags] <filter> [file...]
```

tq reads TOON or JSON from stdin (or files), applies a jq filter, and writes the result.

### Output format

| Input  | Default output | With `--json` |
|--------|---------------|---------------|
| TOON   | TOON          | JSON          |
| JSON   | TOON          | JSON          |

Input format is auto-detected. Output is TOON by default; use `--json` for JSON output.

Use `-` as a file argument to read from stdin explicitly (e.g. `tq '.key' -`).

### Flags

| Flag | Description |
|------|-------------|
| `--json` | Output JSON instead of TOON |
| `--toon` | Output TOON (default, explicit for scripts) |
| `-r`, `--raw-output` | Output raw strings (no quotes) |
| `-c`, `--compact-output` | Compact output |
| `-s`, `--slurp` | Read all inputs into an array |
| `-n`, `--null-input` | Run filter without reading input |
| `-e`, `--exit-status` | Set exit code based on output |
| `-j`, `--join-output` | No newline between outputs |
| `--tab` | Use tab indentation |
| `--indent N` | Set indentation width |
| `--delimiter` | TOON output delimiter: `comma`, `tab`, `pipe` |
| `--stream` | Output path-value pairs for streaming |
| `--arg NAME --arg VALUE` | Pass a string variable to the filter |
| `--argjson NAME --argjson VALUE` | Pass a JSON variable to the filter |
| `-f`, `--from-file` | Read filter from file |
| `--version` | Print version |
| `-h`, `--help` | Show help with examples |

Short flags can be combined: `-rc` is equivalent to `-r -c`.

### Filter language

tq uses the [jq filter language](https://jqlang.github.io/jq/manual/). All jq filters, builtins, and operators work in tq:

```bash
# Identity
tq '.'

# Field access
tq '.name'
tq '.user.address.city'

# Array operations
tq '.[0]'
tq '.[]'
tq '.users[0:3]'

# Pipes and composition
tq '.users[] | .name'
tq '.users | map(select(.active)) | length'

# Conditionals
tq '.users[] | if .active then .name else empty end'

# String operations
tq '.name | split(" ") | .[0]'

# Object construction
tq '{name: .user.name, email: .user.email}'

# And everything else jq supports...
```

### Streaming

tq supports streaming input, matching jq's behavior for multi-document streams:

```bash
# Process a stream of multiple JSON values from stdin
echo '{"a":1}
{"b":2}
{"c":3}' | tq '.a // .b // .c'
# 1
# 2
# 3

# Stream of TOON documents (separated by blank lines)
printf 'name: Alice\nage: 30\n\nname: Bob\nage: 25\n' | tq '.name'
# Alice
# Bob

# Decompose a document into path-value pairs with --stream
echo '{"a":1,"b":[2,3]}' | tq --stream --json '.'
# [["a"],1]
# [["b",0],2]
# [["b",1],3]
# ...

# Filter streamed path-value pairs
echo '{"a":1,"b":2}' | tq --stream --json 'select(.[0][0] == "a")'
# [["a"],1]

# Slurp a stream of values into an array
echo '{"a":1}
{"b":2}' | tq -s 'length'
# 2
```

## Why tq?

TOON is designed for LLM workflows where every token counts. tq lets you work with TOON data using the query language you already know from jq — no new syntax to learn.

Since both formats share the same data model, tq also works as a **format converter**:

```bash
# JSON → TOON
cat data.json | tq '.'

# TOON → JSON
cat data.toon | tq --json '.'
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT](LICENSE)

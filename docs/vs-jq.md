# tq vs jq — Comparison and Migration Guide

`tq` accepts the same filter language as `jq` (via gojq), accepts the same flags, and returns the same exit codes. The key addition is TOON output: a compact, human-readable format that reduces token count significantly — useful for LLM workflows and shell pipelines where JSON punctuation adds noise without value. See the [Known Differences from jq](#known-differences-from-jq) section for the small set of features that behave differently.

---

## 1. Same Filter Language

Every jq filter works unchanged in `tq`. Field access, pipes, `select`, `map` — all identical.

Field access and pipes:

```tq
echo '{"name":"Alice","age":30}' | tq '.name'
# output
Alice
```

`select` to filter values:

```tq
echo '{"users":[{"name":"Alice","role":"admin"},{"name":"Bob","role":"user"}]}' | tq '[.users[] | select(.role == "admin") | .name]'
# output
[1]: Alice
```

`map` to transform arrays:

```tq
echo '[1,2,3,4,5]' | tq 'map(. * 2)'
# output
[5]: 2,4,6,8,10
```

Pipe with `to_entries`:

```tq
echo '{"z":"last","a":"first","m":"mid"}' | tq 'to_entries | map(.key)'
# output
[3]: a,m,z
```

Any jq tutorial or filter you already have will work with `tq` without modification (see [Known Differences from jq](#known-differences-from-jq) for the small set of exceptions).

---

## 2. Default Output: TOON vs JSON

The only user-visible difference from jq is the default output format.

Simple object:

```tq
echo '{"name":"Alice","age":30}' | tq '.'
# output
age: 30
name: Alice
```

Same command with `--json`:

```tq
echo '{"name":"Alice","age":30}' | tq --json '.'
# output
{
  "age": 30,
  "name": "Alice"
}
```

Array of objects — TOON uses table notation:

```tq
echo '{"users":[{"name":"Alice","score":95},{"name":"Bob","score":87}]}' | tq '.users'
# output
[2]{name,score}:
  Alice,95
  Bob,87
```

Same with `--json`:

```tq
echo '{"users":[{"name":"Alice","score":95},{"name":"Bob","score":87}]}' | tq --json '.users'
# output
[
  {
    "name": "Alice",
    "score": 95
  },
  {
    "name": "Bob",
    "score": 87
  }
]
```

TOON is more compact and easier to scan. That matters when reviewing shell output or feeding data to an LLM — fewer tokens, no punctuation noise.

---

## 3. Format Auto-Detection

`tq` reads both JSON and TOON without any flags. `jq` only reads JSON.

JSON input:

```tq
echo '{"name":"Alice","age":30}' | tq '.name'
# output
Alice
```

TOON input — same filter, same result:

```tq
printf 'name: Alice\nage: 30\n' | tq '.name'
# output
Alice
```

TOON table input:

```tq
printf 'users[3]{name,score}:\n  Alice,85\n  Bob,92\n  Carol,78' | tq '.users[] | select(.score > 86) | .name'
# output
Bob
```

The format is detected from the first non-whitespace character. TOON starts with an identifier; JSON starts with `{`, `[`, `"`, a digit, or a keyword. No flag needed.

---

## 4. Streaming

Use `--stream` to process large JSON files without loading the whole document into memory.

**JSON input** uses a token-level reader and runs in O(depth) memory — the document size does not affect peak memory usage. JSON streaming emits one `[path, value]` pair per leaf:

```tq
echo '{"name":"Alice","scores":[95,87,92]}' | tq --stream --json -c 'select(length == 2)'
# output
[["name"],"Alice"]
[["scores",0],95]
[["scores",1],87]
[["scores",2],92]
```

Filter to a specific path:

```tq
echo '{"users":[{"name":"Alice"},{"name":"Bob"}]}' | tq --stream --json -c 'select(.[0][0] == "users" and length == 2)'
# output
[["users",0,"name"],"Alice"]
[["users",1,"name"],"Bob"]
```

**TOON input** with `--stream` loads the full document into memory and applies `tostream` internally — there is no O(depth) guarantee for TOON. If you need memory-efficient streaming, convert TOON to JSON first or use a JSON source:

```tq
printf 'name: Alice\nage: 30\n' | tq --stream --json -c '.'
# output
[["age"],30]
[["name"],"Alice"]
[["name"]]
```

Use `--stream` when processing large JSON files; for TOON inputs it still produces the streaming format but without the memory benefit.

---

## 5. Migration Guide

**Step 1:** Replace `jq` with `tq` in your command line. That is all that is required for interactive use.

**All jq flags carry over unchanged:**

| Flag | Meaning |
|------|---------|
| `-r` | Raw string output |
| `-c` | Compact output (with `--json`) |
| `-s` | Slurp: read all inputs into one array |
| `-n` | Null input: run filter without reading stdin |
| `-e` | Exit 4 when filter produces no output |
| `-j` | Join output: no newline between values |
| `-f file` | Read filter from file |
| `--arg name value` | Bind a string variable (see note below) |
| `--argjson name value` | Bind a JSON variable |
| `--stream` | Output path-value pairs for streaming |

**tq-specific flags:**

| Flag | Meaning |
|------|---------|
| `--json` | Output JSON instead of TOON |
| `--toon` | Output TOON explicitly (default) |
| `--tab` | Use a tab character for JSON indentation |
| `--indent N` | Use N spaces for JSON indentation |
| `--delimiter comma\|tab\|pipe` | TOON column delimiter (default: comma) |
| `--version` | Print version and exit |

Raw string output:

```tq
echo '{"name":"Alice"}' | tq -r '.name'
# output
Alice
```

Slurp a stream and aggregate:

```tq
printf '{"n":1}\n{"n":2}\n{"n":3}\n' | tq -s 'map(.n) | add'
# output
6
```

Null input:

```tq
tq -n '1 + 1'
# output
2
```

String variable — note the syntax difference from jq: tq uses a repeated flag (`--arg name --arg value`) because flags accept one argument each, whereas jq uses `--arg name value`:

```tq
tq -n --arg name --arg Alice '$name'
# output
Alice
```

The equivalent jq command is `jq -n --arg name Alice '$name'` and returns `"Alice"` (quoted JSON string). `tq` returns `Alice` (unquoted TOON).

**Exit codes are identical to jq:**

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage or I/O error |
| 3 | Filter parse or compile error |
| 4 | `--exit-status` with no output |
| 5 | Runtime error |

Exit 4 when no output matches:

```tq
echo '{"a":1}' | tq -e 'select(.a > 5)'
# output (exit: 4)
```

**The only difference:** default output is TOON, not JSON. Scripts that pipe `tq` output to another JSON tool need `--json`.

---

## 6. When to Use `--json`

Use `--json` whenever the next tool in the pipeline expects JSON.

Piping to another JSON processor:

```tq
echo '{"name":"Alice","age":30,"city":"Paris"}' | tq --json -c '{name,city}'
# output
{"city":"Paris","name":"Alice"}
```

Compact JSON for an API payload:

```tq
echo '{"version":"1.2.3","name":"myapp"}' | tq --json -c '{name,version}'
# output
{"name":"myapp","version":"1.2.3"}
```

Pretty JSON when you need valid, indented JSON output:

```tq
printf 'name: Alice\nage: 30\n' | tq --json '.'
# output
{
  "age": 30,
  "name": "Alice"
}
```

For interactive inspection and shell scripts that only read the output as text, the TOON default is sufficient — and more readable.

---

## 7. TOON Output Details

| Value type | TOON output |
|------------|-------------|
| String | Unquoted (quoted only when ambiguous, e.g. `"75001"`) |
| Number | Plain (`42`, `3.14`) |
| Boolean | `true` / `false` |
| Null | `null` |
| Object | `key: value` pairs, alphabetically sorted |
| Flat array | `[N]: v1,v2,...` |
| Uniform array of objects | `[N]{col1,col2}:\n  v1,v2\n  ...` |
| Nested object | Indented `key:\n  subkey: value` |

Scalar types:

```tq
echo '{"name":"Alice","active":true,"score":42,"missing":null}' | tq '.'
# output
active: true
missing: null
name: Alice
score: 42
```

Nested object:

```tq
echo '{"user":{"name":"Alice","address":{"city":"Paris","zip":"75001"}}}' | tq '.'
# output
user:
  address:
    city: Paris
    zip: "75001"
  name: Alice
```

Table notation — the header `[N]{col1,col2}:` states the row count and column names. Each subsequent line is one row:

```tq
echo '{"users":[{"name":"Alice","score":95},{"name":"Bob","score":87}]}' | tq '.'
# output
users[2]{name,score}:
  Alice,95
  Bob,87
```

Simple array (no uniform object structure):

```tq
echo '[1,2,3]' | tq '.'
# output
[3]: 1,2,3
```

---

## Known Differences from jq

`tq` uses gojq internally. The filter language is nearly identical to jq, but the following features are absent or behave differently:

**`keys_unsorted` is not available.** Go maps are always iterated in sorted order; `keys` and `keys_unsorted` return the same result. Use `keys`:

```tq
echo '{"z":1,"a":2,"m":3}' | tq 'keys'
# output
[3]: a,m,z
```

**`--sort-keys` / `-S` is not needed.** Object keys are always emitted in sorted order in both TOON and JSON output. The flag is accepted for compatibility but has no additional effect.

**Regular expressions use Go's RE2 engine.** RE2 does not support backreferences or lookahead/lookbehind assertions. Patterns that rely on these features will produce a runtime error:

```tq
echo '"abcabc"' | tq 'test("(abc)\\1")'
# output error (exit: 5)
tq: invalid regular expression "(abc)\\1": error parsing regexp: invalid escape sequence: `\1`
```

Standard patterns without backreferences work as expected:

```tq
echo '"foobar"' | tq 'test("foo.+")'
# output
true
```

**`input_line_number` is not supported.** It is not implemented in gojq and will produce a compile error.

**`$__loc__` is not supported.** It is not implemented in gojq and will produce a compile error.

**`nan` and `infinite` as JSON input values are rejected.** jq accepts `NaN` and `Infinity` as literal tokens in JSON input; tq (via gojq) does not — they will produce a parse error. Values computed internally with `nan` are represented as `null` in both tools.

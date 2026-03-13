# tq Error Reference

This document covers every error tq can produce, with exact error text, explanations, and fixes.

---

## 1. Exit Codes

| Code | Meaning                                          |
|------|--------------------------------------------------|
| 0    | Success                                          |
| 2    | Usage or I/O error (bad flag, file not found)    |
| 3    | Filter parse or compile error                    |
| 4    | `--exit-status` with no output produced          |
| 5    | Runtime error during filter execution            |

---

## 2. Parse Errors (exit 3)

Parse errors happen before any input is read. tq cannot compile the filter expression.

### Unexpected end of input

An unclosed bracket, parenthesis, string, or incomplete expression causes the parser to reach EOF while still expecting more tokens.

```tq
echo '{"a":1}' | tq '.[invalid['
# output error (exit: 3)
tq: parse error: unexpected EOF
```

**Fix:** close the bracket — `.[\"invalid\"]` for string key access, or `.[]` to iterate.

```tq
echo '{"a":1}' | tq '.["a"]'
# output
1
```

### Unmatched parenthesis

```tq
echo '{"a":1}' | tq '(.a + .b'
# output error (exit: 3)
tq: parse error: unexpected EOF
```

**Fix:** add the closing parenthesis: `(.a + .b)`.

### Incomplete `if-then-else`

```tq
echo '{"active":true}' | tq 'if .active then 1'
# output error (exit: 3)
tq: parse error: unexpected EOF
```

**Fix:** complete the expression with `else` and `end`:

```tq
echo '{"active":true}' | tq 'if .active then 1 else 0 end'
# output
1
```

### Unterminated string literal

```tq
echo '{"a":1}' | tq '"unterminated'
# output error (exit: 3)
tq: parse error: unterminated string literal
```

**Fix:** close the string with a matching `"`.

### Unexpected token

A stray or misplaced token that cannot appear at that position in the grammar.

```tq
echo '{"a":1}' | tq 'else .b end'
# output error (exit: 3)
tq: parse error: unexpected token "else"
```

```tq
echo '{"a":1}' | tq '.a; .b'
# output error (exit: 3)
tq: parse error: unexpected token ";"
```

**Fix:** use `,` to produce multiple outputs, or `|` to pipe: `.a, .b` or `.a | .b`.

### Undefined function

```tq
echo '{"a":1}' | tq 'blah(.a)'
# output error (exit: 3)
tq: compile error: function not defined: blah/1
```

**Fix:** check the [jq manual](https://jqlang.github.io/jq/manual/) for built-in names, or define the function with `def`.

### Undefined variable

```tq
echo '{"a":1}' | tq '$undefined'
# output error (exit: 3)
tq: compile error: variable not defined: $undefined
```

**Fix:** pass the variable with `--arg` or `--argjson`, or bind it inline with `as`.

---

## 3. Runtime Errors (exit 5)

Runtime errors occur while evaluating the filter against actual input values.

### Cannot iterate over a non-iterable

`.[]` on null, a boolean, or a number.

```tq
echo '{"items":null}' | tq '.items[]'
# output error (exit: 5)
tq: cannot iterate over: null
```

**Fix:** use the optional operator `?` to suppress the error, or provide a default:

```tq
echo '{"items":null}' | tq '.items[]?'
# output
```

```tq
echo '{"items":null}' | tq '(.items // []) | .[]'
# output
```

```tq
echo '{"active":true}' | tq '.active[]'
# output error (exit: 5)
tq: cannot iterate over: boolean (true)
```

### Expected an object

Field access (`.field`) on a value that is not an object.

```tq
echo '42' | tq '.field'
# output error (exit: 5)
tq: expected an object but got: number (42)
```

```tq
echo '"hello"' | tq '.field'
# output error (exit: 5)
tq: expected an object but got: string ("hello")
```

```tq
echo '[1,2,3]' | tq '.name'
# output error (exit: 5)
tq: expected an object but got: array ([1,2,3])
```

**Fix:** check the type before accessing a field, or iterate into the right level:

```tq
echo '{"items":[{"name":"a"},{"name":"b"}]}' | tq '.items | .[] | .name'
# output
a
b
```

### Expected an array

Numeric index access (`.[N]`) on a value that is not an array.

```tq
echo '{"a":1}' | tq '.[0]'
# output error (exit: 5)
tq: expected an array but got: object ({"a":1})
```

```tq
echo '42' | tq '.[0]'
# output error (exit: 5)
tq: expected an array but got: number (42)
```

**Fix:** check the type first with `type`, or suppress the error with the `?` operator:

```tq
echo '{"a":1}' | tq '.[0]?'
# output
```

### Cannot add incompatible types

Adding values of mismatched types.

```tq
echo '{"count":5}' | tq '"Result: " + .count'
# output error (exit: 5)
tq: cannot add: string ("Result: ") and number (5)
```

**Fix:** convert to string first with `tostring`:

```tq
echo '{"count":5}' | tq '"Result: " + (.count | tostring)'
# output
"Result: 5"
```

```tq
echo '{"score":9.5}' | tq '.score + " points"'
# output error (exit: 5)
tq: cannot add: number (9.5) and string (" points")
```

**Fix:** `(.score | tostring) + " points"`.

### Cannot divide by zero

```tq
echo '10' | tq '. / 0'
# output error (exit: 5)
tq: cannot divide number (10) by: number (0)
```

**Fix:** guard with a condition: `if . == 0 then 0 else 10 / . end`.

### Built-in applied to wrong type

```tq
echo '"hello"' | tq 'keys'
# output error (exit: 5)
tq: keys cannot be applied to: string ("hello")
```

```tq
echo '"hello"' | tq 'ascii_downcase | . + 1'
# output error (exit: 5)
tq: cannot add: string ("hello") and number (1)
```

```tq
echo '42' | tq 'has("key")'
# output error (exit: 5)
tq: has("key") cannot be applied to: number (42)
```

**Fix:** `keys` and `has` work on objects and arrays. Use `type` to branch: `if type == "object" then keys else [] end`.

### Negative limit

```tq
tq -n 'limit(-1; range(10))'
# output error (exit: 5)
tq: error: limit doesn't support negative count
```

**Fix:** pass a non-negative count to `limit`.

### Format string not defined

```tq
echo '"hello"' | tq '@unknown'
# output error (exit: 5)
tq: format not defined: @unknown
```

**Fix:** use a supported format string: `@base64`, `@uri`, `@html`, `@csv`, `@tsv`, `@json`, `@text`, `@sh`.

---

## 4. Usage Errors (exit 2)

Usage errors are detected before the filter runs.

### File not found

```tq
tq '.' missing.json
# output error (exit: 2)
tq: open missing.json: no such file or directory
```

### Filter file not found

```tq
tq -f missing.jq
# output error (exit: 2)
tq: open missing.jq: no such file or directory
```

**Fix:** create the file or correct the path. Use `tq '…'` to pass the filter inline instead.

### Unknown delimiter

```tq
echo 'a: 1' | tq --delimiter=semicolon '.'
# output error (exit: 2)
tq: unknown delimiter "semicolon" (use comma, tab, or pipe)
```

**Fix:** use one of the supported values: `--delimiter=comma`, `--delimiter=tab`, or `--delimiter=pipe`.

### Mutually exclusive flags

```tq
echo '{"a":1}' | tq --json --toon '.'
# output error (exit: 2)
tq: --json and --toon are mutually exclusive
```

### Missing flag value (`--arg`)

`--arg` requires a name and a value as two separate flag invocations.

```tq
echo '{}' | tq --arg name '$name'
# output error (exit: 2)
tq: --arg requires pairs of name and value
```

**Fix:** provide both name and value: `--arg name --arg Alice`.

```tq
echo '{}' | tq --arg name --arg Alice '"Hello, " + $name'
# output
"Hello, Alice"
```

### Invalid JSON value for `--argjson`

```tq
echo '{}' | tq --argjson val --argjson 'notjson' '$val'
# output error (exit: 2)
tq: --argjson value for "val" is not valid JSON: invalid character 'o' in literal null (expecting 'u')
```

**Fix:** supply valid JSON: `--argjson val --argjson '42'` or `--argjson val --argjson '{"x":1}'`.

```tq
echo '{}' | tq --argjson val --argjson '42' '$val + 1'
# output
43
```

---

## 5. No Output (exit 4)

Exit code 4 is only returned when `--exit-status` (`-e`) is set and the filter produces no output.

### `select()` that matches nothing

```tq
echo '[1,2,3]' | tq -e '.[] | select(. > 10)'
# output error (exit: 4)
```

### `empty` filter

```tq
tq -n -e 'empty'
# output error (exit: 4)
```

**Note:** a filter that produces `null` or `false` still sets `hasOutput = true`; exit 4 means zero values were produced, not that the value was falsy.

---

## 6. Common Gotchas

These are not errors — they are surprising behaviors that can look like bugs.

### Null propagation: `.missing` returns null, not an error

Accessing a key that does not exist returns `null` rather than raising an error.

```tq
echo '{"a":1}' | tq '.missing'
# output
null
```

Chaining further through null also returns null:

```tq
echo '{}' | tq '.user.name'
# output
null
```

Use `//` to supply a default, or `has("key")` to check presence explicitly:

```tq
echo '{}' | tq '.user.name // "unknown"'
# output
unknown
```

### TOON output does not quote strings

In TOON (the default output format), string values are printed without quotes. Use `--json` or `-r` when the quotes matter.

```tq
echo '{"name":"Alice"}' | tq '.name'
# output
Alice
```

```tq
echo '{"name":"Alice"}' | tq --json '.name'
# output
"Alice"
```

### Object keys are sorted alphabetically in TOON output

```tq
echo '{"z":1,"a":2,"m":3}' | tq '.'
# output
a: 2
m: 3
z: 1
```

This matches jq's behavior. If insertion order matters, use `.keys_unsorted`.

### `//` only catches `null` and `false`, not zero or empty string

The alternative operator `//` is not a general "missing or empty" guard. It only activates when the left side produces `null` or `false`.

```tq
echo '{"retries":0}' | tq '.retries // 3'
# output
0
```

```tq
echo '{"label":""}' | tq '.label // "none"'
# output
""
```

Both `0` and `""` pass through unchanged because they are not `null` or `false`.

### `.[] | select()` produces scalar outputs; `map(select())` produces an array

```tq
echo '[1,2,3,4,5]' | tq '.[] | select(. > 3)'
# output
4
5
```

```tq
echo '[1,2,3,4,5]' | tq 'map(select(. > 3))'
# output
[2]: 4,5
```

Use `.[] | select()` when you want to feed results into a further pipe. Use `map(select())` when you need to keep an array.

### `--stream` changes the shape of input to the filter

With `--stream`, tq does not pass the full document to the filter. Instead the filter receives a stream of `[path, value]` pairs — one per leaf value plus a terminator. A filter that works without `--stream` will often fail or produce unexpected results with it.

```tq
echo '{"a":1}' | tq '.a'
# output
1
```

```tq
echo '{"a":1}' | tq --stream '.a'
# output error (exit: 5)
tq: expected an object but got: array ([["a"],1])
```

When using `--stream`, write the filter to handle `[path, value]` pairs, for example with `select(length == 2)` to skip the truncation markers, then extract `.` as needed.

### `try`-`catch` turns runtime errors into strings

`try expr catch .` suppresses exit-5 errors and gives you the error message as a string:

```tq
echo 'null' | tq 'try (.a[]) catch .'
# output
"cannot iterate over: null"
```

This is useful for defensive pipelines, but be careful — the output is now a string, not the original type.

---

## 7. Quick Fix Reference

| Error message | Cause | Fix |
|---|---|---|
| `tq: parse error: unexpected EOF` | Unclosed bracket, paren, string, or incomplete expression | Close the open construct |
| `tq: parse error: unterminated string literal` | Missing closing `"` in filter | Add the closing `"` |
| `tq: parse error: unexpected token ";"` | Semicolon used as separator | Use `,` (multi-output) or `\|` (pipe) |
| `tq: compile error: function not defined: foo/1` | Calling a nonexistent built-in | Check spelling or define the function with `def` |
| `tq: compile error: variable not defined: $x` | Using `$x` without binding it | Add `--arg x --arg value` or bind with `as` |
| `tq: cannot iterate over: null` | `.field[]` where `.field` is null | Use `[]?` or `(.field // []) \| .[]` |
| `tq: cannot iterate over: boolean (true)` | `.[]` on a boolean | Check type first; iterate the correct field |
| `tq: expected an object but got: number (42)` | `.field` access on non-object | Check input type; use correct path |
| `tq: expected an array but got: X` | Using `.[N]` on a non-array | Check type with `type`, or use `.[N]?` to suppress |
| `tq: cannot add: string ("x") and number (1)` | String + number without conversion | Wrap number with `tostring` |
| `tq: cannot divide number (10) by: number (0)` | Division by zero | Guard with `if . == 0` |
| `tq: keys cannot be applied to: string ("x")` | `keys` called on non-object/array | Check `type` before calling `keys` |
| `tq: format not defined: @unknown` | Unknown `@format` string | Use `@base64`, `@uri`, `@html`, `@csv`, `@tsv`, `@json`, `@sh` |
| `tq: open file.json: no such file or directory` | Input file missing | Check path; use `-` for stdin |
| `tq: open filter.jq: no such file or directory` | `-f` filter file missing | Fix path or pass filter inline |
| `tq: unknown delimiter "x" (use comma, tab, or pipe)` | Invalid `--delimiter` value | Use `comma`, `tab`, or `pipe` |
| `tq: --arg requires pairs of name and value` | Odd number of `--arg` tokens | Each variable needs a name and a value |
| `tq: --argjson value for "x" is not valid JSON: …` | Value is not valid JSON | Pass valid JSON: `--argjson x --argjson '42'` |
| `tq: --json and --toon are mutually exclusive` | Both output flags set | Use one or neither |
| *(exit 4, no output)* | `--exit-status` with no filter output | Fix the filter, or drop `-e` if empty output is fine |

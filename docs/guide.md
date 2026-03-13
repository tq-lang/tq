# tq Progressive Tutorial

`tq` is a jq-compatible command-line processor for TOON and JSON. Input format is
auto-detected. Output is TOON by default.

This tutorial builds from simple queries to advanced transformations. Every example
is a real command with verified output.

---

## 1. Your First Query

The simplest tq session: pipe data in, get data out.

Pipe JSON in — tq auto-detects the format and outputs TOON:

```tq
echo '{"name":"Alice","age":30}' | tq '.'
# output
age: 30
name: Alice
```

TOON is a human-readable key-value format. Keys are sorted alphabetically.
Pipe TOON directly and extract a single field:

```tq
printf 'name: Alice\nage: 30' | tq '.name'
# output
Alice
```

The identity filter `.` passes the document through unchanged:

```tq
printf 'name: Alice\nage: 30' | tq '.'
# output
age: 30
name: Alice
```

---

## 2. Objects and Fields

Access a field with `.fieldname`:

```tq
printf 'name: Alice\nage: 30\ncity: Paris' | tq '.city'
# output
Paris
```

Chain field access to navigate nested objects:

```tq
printf 'user:\n  name: Alice\n  city: Paris' | tq '.user.name'
# output
Alice
```

Deep nesting works the same way:

```tq
printf 'a:\n  b:\n    c: deep' | tq '.a.b.c'
# output
deep
```

Extract multiple fields in one filter — each produces a separate output line:

```tq
printf 'name: Alice\nage: 30\ncity: Paris' | tq '.name, .city'
# output
Alice
Paris
```

Construct a new object by picking and renaming fields:

```tq
printf 'first: Alice\nlast: Smith\nage: 30' | tq '{name: .first, years: .age}'
# output
name: Alice
years: 30
```

Shorthand construction: when the output key matches the field name, write it once:

```tq
printf 'name: Alice\nage: 30' | tq '{name, age}'
# output
age: 30
name: Alice
```

---

## 3. Arrays

Access an element by zero-based index:

```tq
echo '{"colors":["red","green","blue"]}' | tq '.colors[0]'
# output
red
```

Negative indices count from the end:

```tq
echo '{"colors":["red","green","blue"]}' | tq '.colors[-1]'
# output
blue
```

Iterate every element with `.[]`:

```tq
echo '{"scores":[85,90,72]}' | tq '.scores[]'
# output
85
90
72
```

`map(f)` applies a filter to every element of an array and returns an array:

```tq
echo '{"nums":[1,2,3]}' | tq '.nums | map(. * 10)'
# output
[3]: 10,20,30
```

Slice a range (start inclusive, end exclusive):

```tq
echo '{"nums":[10,20,30,40,50]}' | tq '.nums[1:4]'
# output
[3]: 20,30,40
```

Get the length of an array:

```tq
echo '{"tags":["go","toon","cli"]}' | tq '.tags | length'
# output
3
```

Arrays of objects render as TOON table notation. Columns are sorted alphabetically:

```tq
echo '{"users":[{"name":"Alice","age":30},{"name":"Bob","age":25}]}' | tq '.users'
# output
[2]{age,name}:
  30,Alice
  25,Bob
```

Access a specific column from a table by iterating and selecting:

```tq
echo '{"users":[{"name":"Alice","age":30},{"name":"Bob","age":25},{"name":"Carol","age":35}]}' | tq '.users[].name'
# output
Alice
Bob
Carol
```

TOON table notation is also valid input. tq parses it back into objects:

```tq
printf 'users[2]{name,age}:\n  Alice,30\n  Bob,25' | tq '.users[1].name'
# output
Bob
```

---

## 4. Pipes — Chaining Filters

The `|` operator passes the output of one filter as the input to the next.

Iterate an array of objects and extract a field:

```tq
echo '{"users":[{"name":"Alice","active":true},{"name":"Bob","active":false}]}' | tq '.users[] | .name'
# output
Alice
Bob
```

Add a `select` stage to filter the stream:

```tq
echo '{"users":[{"name":"Alice","active":true},{"name":"Bob","active":false}]}' | tq '.users[] | select(.active) | .name'
# output
Alice
```

Build pipelines step by step. First, see what `.users[]` produces:

```tq
echo '{"users":[{"name":"Alice","active":true},{"name":"Bob","active":false}]}' | tq '.users[]'
# output
active: true
name: Alice
active: false
name: Bob
```

Then add `select` to keep only active users:

```tq
echo '{"users":[{"name":"Alice","active":true},{"name":"Bob","active":false}]}' | tq '.users[] | select(.active)'
# output
active: true
name: Alice
```

A more complete pipeline — filter, transform, and collect results:

```tq
echo '{"orders":[{"id":1,"status":"shipped","total":150},{"id":2,"status":"pending","total":75},{"id":3,"status":"shipped","total":200}]}' | tq '[.orders[] | select(.status == "shipped")] | map(.total) | add'
# output
350
```

---

## 5. Selecting and Filtering

`select(condition)` passes a value through only when the condition is true.

Keep users over 18:

```tq
echo '{"users":[{"name":"Alice","age":30},{"name":"Bob","age":17},{"name":"Carol","age":25}]}' | tq '.users[] | select(.age > 18) | .name'
# output
Alice
Carol
```

Filter by substring match:

```tq
echo '{"users":[{"name":"Alice","age":30},{"name":"Bob","age":17},{"name":"Carol","age":25}]}' | tq '.users[] | select(.name | contains("Ali")) | .name'
# output
Alice
```

Filter by regex with `test`:

```tq
echo '{"users":[{"name":"Alice","age":30},{"name":"Bob","age":17},{"name":"Anna","age":22}]}' | tq '[.users[] | select(.name | test("^A"))] | map(.name)'
# output
[2]: Alice,Anna
```

Wrap the filtered stream in `[...]` to collect results into an array:

```tq
echo '{"users":[{"name":"Alice","age":30},{"name":"Bob","age":17},{"name":"Carol","age":25}]}' | tq '[.users[] | select(.age > 18)]'
# output
[2]{age,name}:
  30,Alice
  25,Carol
```

Combine conditions with `and` and `or`:

```tq
echo '{"users":[{"name":"Alice","active":true,"age":30},{"name":"Bob","active":false,"age":25},{"name":"Carol","active":true,"age":17}]}' | tq '.users[] | select(.active and .age >= 18) | .name'
# output
Alice
```

```tq
echo '{"users":[{"name":"Alice","role":"admin"},{"name":"Bob","role":"viewer"},{"name":"Carol","role":"editor"}]}' | tq '.users[] | select(.role == "admin" or .role == "editor") | .name'
# output
Alice
Carol
```

Check whether a key exists with `has`:

```tq
printf 'name: Alice\nage: 30' | tq 'has("age")'
# output
true
```

```tq
printf 'name: Alice\nage: 30' | tq 'has("email")'
# output
false
```

`in` tests whether a value is a key of an object:

```tq
tq -n '"b" | in({"a":1,"b":2,"c":3})'
# output
true
```

`[.arr[] | select(...)]` and `.arr | map(select(...))` are equivalent ways to filter an array:

```tq
echo '{"arr":[1,2,3,4,5]}' | tq '[.arr[] | select(. > 2)]'
# output
[3]: 3,4,5
```

```tq
echo '{"arr":[1,2,3,4,5]}' | tq '.arr | map(select(. > 2))'
# output
[3]: 3,4,5
```

Use `type` to inspect the type of any value:

```tq
printf 'name: Alice\nage: 30' | tq '.name | type'
# output
string
```

```tq
echo '{"arr":[1,2,3]}' | tq '.arr | type'
# output
array
```

---

## 6. Building New Objects

Add a field to an existing object with `+`:

```tq
printf 'name: Alice\nage: 30' | tq '. + {status: "active"}'
# output
age: 30
name: Alice
status: active
```

Remove a field with `del`:

```tq
printf 'name: Alice\nage: 30\npassword: secret' | tq 'del(.password)'
# output
age: 30
name: Alice
```

Remove multiple fields in one `del`:

```tq
printf 'name: Alice\nage: 30\npassword: secret\ntoken: abc' | tq 'del(.password, .token)'
# output
age: 30
name: Alice
```

Rename a field by constructing a new object:

```tq
printf 'first: Alice\nlast: Smith' | tq '{name: .first, surname: .last}'
# output
name: Alice
surname: Smith
```

Update a field in place with `|=`:

```tq
printf 'name: Alice\nage: 30' | tq '.age |= . + 1'
# output
age: 31
name: Alice
```

Update a field across every element in an array:

```tq
echo '{"users":[{"name":"Alice","score":85},{"name":"Bob","score":70}]}' | tq '.users | map(.score |= . + 5)'
# output
[2]{name,score}:
  Alice,90
  Bob,75
```

Filter object entries by value with `with_entries`:

```tq
printf 'a: 1\nb: 2\nc: 3' | tq 'with_entries(select(.value > 1))'
# output
b: 2
c: 3
```

---

## 7. String Operations

Concatenate strings with `+`:

```tq
printf 'first: Alice\nlast: Smith' | tq '"Hello, " + .first'
# output
"Hello, Alice"
```

String interpolation with `\(.expr)` — use `-r` to strip the surrounding quotes:

```tq
printf 'first: Alice\nlast: Smith' | tq -r '"\(.first) \(.last)"'
# output
Alice Smith
```

Split a string into an array:

```tq
printf 'tags: go,cli,toon' | tq '.tags | split(",")'
# output
[3]: go,cli,toon
```

Join an array of strings:

```tq
echo '{"words":["hello","world"]}' | tq '.words | join(" ")'
# output
hello world
```

Parse a delimited string and reshape it:

```tq
echo '{"csv":"Alice,30,engineering"}' | tq '.csv | split(",") | {name: .[0], age: .[1], dept: .[2]}'
# output
age: "30"
dept: engineering
name: Alice
```

Test whether a string matches a regex:

```tq
printf 'email: alice@example.com' | tq '.email | test("@")'
# output
true
```

Check prefix and suffix:

```tq
printf 'name: Alice' | tq '.name | startswith("Al")'
# output
true
```

```tq
printf 'filename: report.pdf' | tq '.filename | endswith(".pdf")'
# output
true
```

Lowercase and uppercase:

```tq
printf 'name: ALICE' | tq '.name | ascii_downcase'
# output
alice
```

```tq
printf 'status: active' | tq '.status | ascii_upcase'
# output
ACTIVE
```

Replace all occurrences with `gsub`:

```tq
printf 'text: Hello World' | tq '.text | gsub("o"; "0")'
# output
Hell0 W0rld
```

Convert between types:

```tq
printf 'count: 42' | tq '.count | tostring'
# output
"42"
```

```tq
echo '{"price":"42.50"}' | tq '.price | tonumber'
# output
42.5
```

---

## 8. Variables and Shell Integration

`--arg` injects a shell string as a jq variable. tq uses the flag twice — once
for the name and once for the value: `--arg name --arg value`. This differs from
jq's single-flag form `--arg name value`.

Pass a string variable and use it in a filter:

```tq
tq -n --arg greeting --arg Hello '$greeting'
# output
Hello
```

Use a variable to parameterise a filter at runtime:

```tq
echo '{"users":[{"name":"Alice","dept":"eng"},{"name":"Bob","dept":"sales"},{"name":"Carol","dept":"eng"}]}' | tq --arg dept --arg eng '[.users[] | select(.dept == $dept) | .name]'
# output
[2]: Alice,Carol
```

`--argjson name value` parses the value as JSON, so numbers, booleans, and
objects remain their correct types:

```tq
tq -n --argjson limit --argjson 5 '[range($limit)]'
# output
[5]: 0,1,2,3,4
```

```tq
tq -n --argjson config --argjson '{"limit":10,"debug":false}' '$config.limit'
# output
10
```

`-n` (null input) runs the filter without reading any input — useful for
pure computation:

```tq
tq -n '2 * 21'
# output
42
```

Generate a sequence of squares:

```tq
tq -n '[range(1;6)] | map(. * .)'
# output
[5]: 1,4,9,16,25
```

---

## 9. Multiple Documents

tq processes multiple documents in sequence. TOON documents are separated by
blank lines; JSON documents are separated by whitespace.

Each TOON document is filtered independently:

```tq
printf 'name: Alice\nage: 30\n\nname: Bob\nage: 25' | tq '.name'
# output
Alice
Bob
```

`select` works across the stream:

```tq
printf 'name: Alice\nage: 30\n\nname: Bob\nage: 25' | tq 'select(.age > 26) | .name'
# output
Alice
```

JSON multi-document streams work the same way:

```tq
printf '{"name":"Alice"}\n{"name":"Bob"}' | tq '.name'
# output
Alice
Bob
```

`-s` (slurp) reads all documents into a single array before filtering:

```tq
printf 'name: Alice\nage: 30\n\nname: Bob\nage: 25' | tq -s 'length'
# output
2
```

Collect and transform across a slurped stream:

```tq
printf 'name: Alice\nage: 30\n\nname: Bob\nage: 25\n\nname: Carol\nage: 35' | tq -s 'map(select(.age >= 30)) | map(.name)'
# output
[2]: Alice,Carol
```

Rank a JSON stream by slurping and sorting:

```tq
printf '{"name":"Alice","score":95}\n{"name":"Bob","score":87}\n{"name":"Carol","score":72}' | tq -s 'sort_by(.score) | reverse | map(.name)'
# output
[3]: Alice,Bob,Carol
```

Read from a file containing multiple TOON documents:

```tq
cat <<'EOF' > server.toon
host: db1.internal
port: 5432
role: primary

host: db2.internal
port: 5432
role: replica

host: db3.internal
port: 5432
role: replica
EOF
tq 'select(.role == "replica") | .host' server.toon
# output
db2.internal
db3.internal
```

---

## 10. Conditionals and Defaults

`if-then-else-end` chooses between two branches:

```tq
printf 'score: 85' | tq 'if .score >= 60 then "pass" else "fail" end'
# output
pass
```

Chain multiple conditions with `elif`:

```tq
printf 'score: 82' | tq 'if .score >= 90 then "A" elif .score >= 80 then "B" elif .score >= 70 then "C" else "F" end'
# output
B
```

The alternative operator `//` returns the right side when the left is `null` or
`false`:

```tq
printf 'name: Alice' | tq '.nickname // "no nickname"'
# output
no nickname
```

```tq
printf 'name: Alice\nnickname: Ali' | tq '.nickname // "no nickname"'
# output
Ali
```

`//` treats both `null` and `false` as absent, so it also applies when a field is explicitly false:

```tq
tq -n 'false // "default"'
# output
default
```

The optional operator `.field?` suppresses errors for missing or wrong-type input
instead of raising a runtime error:

```tq
echo '42' | tq '.name'
# output error (exit: 5)
tq: expected an object but got: number (42)
```

```tq
echo '42' | tq '.name?'
# output

```

```tq
echo '{"user":{"name":"Alice"}}' | tq '.user.email? // "no email"'
# output
no email
```

`try-catch` handles errors gracefully:

```tq
echo '{"val":"oops"}' | tq 'try (.val | tonumber) catch "not a number"'
# output
not a number
```

`empty` produces no output — useful to suppress unwanted results:

```tq
echo '{"items":[{"name":"a","active":true},{"name":"b","active":false},{"name":"c","active":true}]}' | tq '.items[] | if .active then .name else empty end'
# output
a
c
```

---

## 11. Aggregation and Transformation

Sort an array of scalars:

```tq
echo '{"scores":[85,42,90,55,78]}' | tq '.scores | sort'
# output
[5]: 42,55,78,85,90
```

Sort descending:

```tq
echo '{"scores":[85,42,90,55,78]}' | tq '.scores | sort | reverse'
# output
[5]: 90,85,78,55,42
```

Sort an array of objects by a field:

```tq
echo '{"users":[{"name":"Alice","age":30},{"name":"Bob","age":17},{"name":"Carol","age":25}]}' | tq '.users | sort_by(.age) | map(.name)'
# output
[3]: Bob,Carol,Alice
```

Find the maximum or minimum by a field:

```tq
echo '{"users":[{"name":"Alice","age":30},{"name":"Bob","age":17},{"name":"Carol","age":25}]}' | tq '.users | max_by(.age) | .name'
# output
Alice
```

```tq
echo '{"users":[{"name":"Alice","age":30},{"name":"Bob","age":17},{"name":"Carol","age":25}]}' | tq '.users | min_by(.age) | .name'
# output
Bob
```

Remove duplicates from an array:

```tq
echo '{"tags":["go","cli","go","toon","cli","go"]}' | tq '.tags | unique'
# output
[3]: cli,go,toon
```

Deduplicate objects by a field:

```tq
echo '{"events":[{"user":"alice","type":"login"},{"user":"bob","type":"login"},{"user":"alice","type":"click"}]}' | tq '.events | unique_by(.user) | map(.user)'
# output
[2]: alice,bob
```

Sum an array with `add`:

```tq
echo '{"scores":[85,42,90,55,78]}' | tq '.scores | add'
# output
350
```

Accumulate with `reduce`:

```tq
echo '{"nums":[1,2,3,4,5]}' | tq '.nums | reduce .[] as $n (0; . + $n)'
# output
15
```

`group_by` partitions an array into groups sharing a field value:

```tq
echo '{"users":[{"name":"Alice","dept":"eng"},{"name":"Bob","dept":"sales"},{"name":"Carol","dept":"eng"},{"name":"Dave","dept":"sales"}]}' | tq '.users | group_by(.dept) | map({dept: .[0].dept, count: length})'
# output
[2]{count,dept}:
  2,eng
  2,sales
```

Aggregate totals across groups:

```tq
echo '{"sales":[{"rep":"Alice","amount":100},{"rep":"Bob","amount":200},{"rep":"Alice","amount":150}]}' | tq '.sales | group_by(.rep) | map({rep: .[0].rep, total: (map(.amount) | add)})'
# output
[2]{rep,total}:
  Alice,250
  Bob,200
```

`to_entries` converts an object into an array of `{key, value}` pairs:

```tq
printf 'name: Alice\nage: 30' | tq 'to_entries'
# output
[2]{key,value}:
  age,30
  name,Alice
```

`from_entries` converts them back:

```tq
echo '{"pairs":[{"key":"x","value":10},{"key":"y","value":20}]}' | tq '.pairs | from_entries'
# output
x: 10
y: 20
```

`with_entries` applies a filter to every key-value pair:

```tq
printf 'name: Alice\nage: 30' | tq 'with_entries(.key |= ascii_upcase)'
# output
AGE: 30
NAME: Alice
```

`keys` returns an object's keys sorted alphabetically:

```tq
printf 'name: Alice\nage: 30\ncity: Paris' | tq 'keys'
# output
[3]: age,city,name
```

`flatten` removes nesting from arrays:

```tq
echo '{"matrix":[[1,2],[3,[4,5]]]}' | tq '.matrix | flatten'
# output
[5]: 1,2,3,4,5
```

---

## 12. Output Formats

By default tq outputs TOON. The flags below change that.

`-r` outputs raw strings without surrounding quotes:

```tq
printf 'name: Alice' | tq -r '.name'
# output
Alice
```

`--json` switches to JSON output; `-c` makes it compact:

```tq
printf 'name: Alice\nage: 30' | tq --json -c '.'
# output
{"age":30,"name":"Alice"}
```

Pretty JSON with the default two-space indent:

```tq
printf 'name: Alice\nage: 30' | tq --json '.'
# output
{
  "age": 30,
  "name": "Alice"
}
```

Tab indentation:

```tq
printf 'name: Alice\nage: 30' | tq --json --tab '.'
# output
{
	"age": 30,
	"name": "Alice"
}
```

Custom indent width (e.g. 4 spaces):

```tq
printf 'name: Alice\nage: 30' | tq --json --indent 4 '.'
# output
{
    "age": 30,
    "name": "Alice"
}
```

TOON scalar arrays use comma as the default delimiter:

```tq
echo '{"tags":["go","toon","cli"]}' | tq '.tags'
# output
[3]: go,toon,cli
```

Switch to pipe delimiter:

```tq
echo '{"users":[{"name":"Alice","age":30},{"name":"Bob","age":25}]}' | tq --delimiter pipe '.users'
# output
[2|]{age|name}:
  30|Alice
  25|Bob
```

Switch to tab delimiter:

```tq
echo '{"users":[{"name":"Alice","age":30},{"name":"Bob","age":25}]}' | tq --delimiter tab '.users'
# output
[2	]{age	name}:
  30	Alice
  25	Bob
```

`-j` (join output) suppresses the newline between results — useful when piping
raw output to another tool:

```tq
printf 'name: Alice\nage: 30' | tq -r -j '.name, " is ", (.age | tostring)'
# output
Alice is 30
```

Format conversion — JSON to TOON (just omit `--json`):

```tq
echo '{"name":"Alice","age":30}' | tq '.'
# output
age: 30
name: Alice
```

---

## What's Next

- The [cheatsheet](cheatsheet.md) is a compact reference for everyday commands.
- Run `tq --help` for a full flag listing.
- Any jq filter works in tq — the [jq manual](https://jqlang.github.io/jq/manual/)
  is the authoritative reference for the filter language.

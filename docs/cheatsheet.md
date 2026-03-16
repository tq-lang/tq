# tq Cheatsheet

`tq` is a jq-compatible CLI for TOON and JSON data. Input format is auto-detected. Output is TOON by default.

## 1. Basics

Extract a field from TOON input:

```tq
printf 'name: Alice\nage: 30' | tq '.name'
# output
Alice
```

Extract a nested field:

```tq
printf 'user:\n  address:\n    city: Paris' | tq '.user.address.city'
# output
Paris
```

Identity — pass the document through unchanged:

```tq
printf 'name: Alice\nage: 30' | tq '.'
# output
age: 30
name: Alice
```

Construct a new object from selected fields:

```tq
printf 'first: Alice\nlast: Smith\nage: 30' | tq '{name: .first, age: .age}'
# output
age: 30
name: Alice
```

Extract multiple fields with a pipe:

```tq
printf 'name: Alice\nrole: admin\nteam: eng' | tq '.name, .role'
# output
Alice
admin
```

Access deeply nested data:

```tq
printf 'a:\n  b:\n    c: deep' | tq '.a.b.c'
# output
deep
```

## 2. Arrays

Access by index (zero-based):

```tq
echo '[10,20,30]' | tq '.[1]'
# output
20
```

Iterate all elements:

```tq
echo '["red","green","blue"]' | tq '.[]'
# output
red
green
blue
```

Slice a range:

```tq
echo '[1,2,3,4,5]' | tq '.[1:3]'
# output
[2]: 2,3
```

Get array length:

```tq
echo '["go","cli","json","toon"]' | tq 'length'
# output
4
```

First element:

```tq
echo '["alpha","beta","gamma"]' | tq '.[0]'
# output
alpha
```

Last element:

```tq
echo '["alpha","beta","gamma"]' | tq '.[-1]'
# output
gamma
```

Iterate an array of objects (TOON table input):

```tq
printf 'users[2]{name,age}:\n  Alice,30\n  Bob,25' | tq '.users[] | .name'
# output
Alice
Bob
```

## 3. Filtering & Selection

Select elements matching a condition:

```tq
printf 'users[3]{name,active}:\n  Alice,true\n  Bob,false\n  Carol,true' | tq '.users[] | select(.active) | .name'
# output
Alice
Carol
```

Map over an array:

```tq
printf 'users[2]{name,age}:\n  Alice,30\n  Bob,25' | tq '.users | map(.name)'
# output
[2]: Alice,Bob
```

Map combined with select:

```tq
printf 'users[3]{name,score}:\n  Alice,85\n  Bob,92\n  Carol,78' | tq '[.users[] | select(.score > 80) | .name]'
# output
[2]: Alice,Bob
```

Check if a key exists with `has`:

```tq
printf 'name: Alice\nage: 30' | tq 'has("email")'
# output
false
```

Check type:

```tq
printf 'name: Alice\nage: 30' | tq 'type'
# output
object
```

Provide a default when a value is null or missing:

```tq
printf 'items[3]{name,value}:\n  foo,10\n  bar,null\n  baz,20' | tq '.items[] | .value // "N/A"'
# output
10
N/A
20
```

Recursive descent (`..`) — walk every value at any depth:

```tq
echo '{"a":{"b":"deep","c":1},"d":"top"}' | tq '[.. | strings]'
# output
[2]: deep,top
```

## 4. Transformation

Get sorted object keys:

```tq
printf 'name: Alice\nage: 30\ncity: Paris' | tq 'keys'
# output
[3]: age,city,name
```

Convert an object to key-value entries:

```tq
printf 'name: Alice\nage: 30' | tq 'to_entries'
# output
[2]{key,value}:
  age,30
  name,Alice
```

Convert key-value entries back to an object:

```tq
printf 'name: Alice\nage: 30' | tq 'to_entries | from_entries'
# output
age: 30
name: Alice
```

Add a field to an object:

```tq
printf 'name: Alice\nage: 30' | tq '. + {city: "Paris"}'
# output
age: 30
city: Paris
name: Alice
```

Remove a field with `del`:

```tq
printf 'name: Alice\nage: 30\ntemp: remove' | tq 'del(.temp)'
# output
age: 30
name: Alice
```

Sort an array of objects:

```tq
printf 'users[3]{name,score}:\n  Alice,85\n  Bob,92\n  Carol,78' | tq '.users | sort_by(.score)'
# output
[3]{name,score}:
  Carol,78
  Alice,85
  Bob,92
```

Sort descending:

```tq
printf 'users[3]{name,score}:\n  Alice,85\n  Bob,92\n  Carol,78' | tq '.users | sort_by(.score) | reverse'
# output
[3]{name,score}:
  Bob,92
  Alice,85
  Carol,78
```

Group by a field:

```tq
printf 'users[4]{name,dept}:\n  Alice,eng\n  Bob,eng\n  Carol,sales\n  Dave,eng' | tq '.users | group_by(.dept) | map(length)'
# output
[2]: 3,1
```

Flatten nested arrays:

```tq
echo '[[1,2],[3,[4,5]]]' | tq 'flatten'
# output
[5]: 1,2,3,4,5
```

Recursively transform every value with `walk()`:

```tq
echo '{"a":1,"b":{"c":2}}' | tq 'walk(if type == "number" then . * 10 else . end)'
# output
a: 10
b:
  c: 20
```

## 5. String Operations

Split a string:

```tq
printf 'full: Alice Smith' | tq '.full | split(" ")'
# output
[2]: Alice,Smith
```

Join an array of strings:

```tq
echo '["Alice","Smith"]' | tq 'join(" ")'
# output
Alice Smith
```

String interpolation:

```tq
printf 'name: Alice\nrole: admin' | tq -r '"Hello, \(.name) (\(.role))"'
# output
Hello, Alice (admin)
```

Test a regex pattern:

```tq
printf 'email: alice@example.com' | tq '.email | test("@example\\.com$")'
# output
true
```

Trim a prefix:

```tq
printf 'path: /home/alice' | tq '.path | ltrimstr("/home/")'
# output
alice
```

Trim a suffix:

```tq
printf 'filename: report.csv' | tq '.filename | rtrimstr(".csv")'
# output
report
```

Lowercase and uppercase:

```tq
printf 'name: Alice' | tq '.name | ascii_downcase'
# output
alice
```

```tq
printf 'name: Alice' | tq '.name | ascii_upcase'
# output
ALICE
```

## 6. Format Strings

Encode as CSV (use `-r` to get a plain string):

```tq
echo '{"a":"x","b":"y"}' | tq -r '[.a, .b] | @csv'
# output
"x","y"
```

Encode as TSV:

```tq
echo '{"a":"x","b":"y"}' | tq -r '[.a, .b] | @tsv'
# output
x	y
```

Base64 encode:

```tq
echo '"hello"' | tq '@base64'
# output
aGVsbG8=
```

Base64 decode (use `-r` to get plain text):

```tq
echo '"aGVsbG8="' | tq -r '@base64d'
# output
hello
```

URL-encode:

```tq
echo '"hello world"' | tq -r '@uri'
# output
hello%20world
```

HTML-escape:

```tq
echo '"<b>hi</b>"' | tq -r '@html'
# output
&lt;b&gt;hi&lt;/b&gt;
```

Re-encode a value as a JSON string:

```tq
echo '{"a":1}' | tq '@json'
# output
"{\"a\":1}"
```

Shell-quote a string:

```tq
echo '"hello world"' | tq -r '@sh'
# output
'hello world'
```

## 7. Math & Logic

Arithmetic:

```tq
tq -n '(3 + 4) * 2'
# output
14
```

Division:

```tq
tq -n '10 / 4'
# output
2.5
```

Modulo:

```tq
tq -n '10 % 3'
# output
1
```

Conditional with `if-then-else`:

```tq
printf 'score: 75' | tq 'if .score >= 90 then "A" elif .score >= 75 then "B" else "C" end'
# output
B
```

Find maximum by field:

```tq
printf 'users[3]{name,score}:\n  Alice,85\n  Bob,92\n  Carol,78' | tq '.users | max_by(.score) | .name'
# output
Bob
```

Find minimum by field:

```tq
printf 'users[3]{name,score}:\n  Alice,85\n  Bob,92\n  Carol,78' | tq '.users | min_by(.score) | .name'
# output
Carol
```

Sum with `reduce`:

```tq
echo '[1,2,3,4,5]' | tq 'reduce .[] as $x (0; . + $x)'
# output
15
```

## 8. Multiple Documents

Process a stream of TOON documents (separated by blank lines):

```tq
printf 'name: Alice\nage: 30\n\nname: Bob\nage: 25' | tq '.name'
# output
Alice
Bob
```

Filter across a document stream:

```tq
printf 'name: Alice\nage: 30\n\nname: Bob\nage: 25' | tq 'select(.age > 26) | .name'
# output
Alice
```

Slurp all documents into a single array:

```tq
printf 'name: Alice\nage: 30\n\nname: Bob\nage: 25' | tq -s 'length'
# output
2
```

Collect names from a slurped stream:

```tq
printf 'name: Alice\nage: 30\n\nname: Bob\nage: 25' | tq -s 'map(.name)'
# output
[2]: Alice,Bob
```

## 9. Output Formats

Raw string output with `-r` (strips quotes):

```tq
printf 'users[2]{name,age}:\n  Alice,30\n  Bob,25' | tq -r '.users[].name'
# output
Alice
Bob
```

Compact JSON output:

```tq
printf 'name: Alice\nage: 30' | tq --json -c '.'
# output
{"age":30,"name":"Alice"}
```

Pretty JSON output:

```tq
printf 'name: Alice\nage: 30' | tq --json '.'
# output
{
  "age": 30,
  "name": "Alice"
}
```

Join output without newlines between results:

```tq
printf 'users[2]{name,age}:\n  Alice,30\n  Bob,25' | tq -r -j '.users[].name'
# output
AliceBob
```

Pipe delimiter for TOON table output:

```tq
printf 'users[2]{name,age}:\n  Alice,30\n  Bob,25' | tq --delimiter pipe '.users'
# output
[2|]{age|name}:
  30|Alice
  25|Bob
```

Tab delimiter:

```tq
printf 'users[2]{name,age}:\n  Alice,30\n  Bob,25' | tq --delimiter tab '.users'
# output
[2	]{age	name}:
  30	Alice
  25	Bob
```

## 10. Variables & Null Input

Pass a string variable with `--arg`:

```tq
echo 'null' | tq --arg name --arg Alice '$name'
# output
Alice
```

Use a variable in a filter:

```tq
printf 'users[2]{name,role}:\n  Alice,admin\n  Bob,user' | tq --arg role --arg admin '.users[] | select(.role == $role) | .name'
# output
Alice
```

Pass a structured JSON variable with `--argjson`:

```tq
echo 'null' | tq --argjson threshold --argjson 80 'if $threshold > 50 then "above" else "below" end'
# output
above
```

Run a computation without any input (`-n`):

```tq
tq -n '1 + 1'
# output
2
```

Generate a sequence:

```tq
tq -n '[range(5)]'
# output
[5]: 0,1,2,3,4
```

Access environment variables via `$ENV` or `env` (returns an object of exported variables; tq exposes only what the OS makes available to the process):

```tq
tq -n '$ENV | keys | length'
# output
0
```

Define a custom function with `def`:

```tq
tq -n 'def double: . * 2; [1,2,3] | map(double)'
# output
[3]: 2,4,6
```

First, last, and limited outputs from a generator:

```tq
echo '[1,2,3,4,5]' | tq 'first(.[])'
# output
1
```

```tq
echo '[1,2,3,4,5]' | tq 'last(.[])'
# output
5
```

```tq
echo '[1,2,3,4,5]' | tq '[limit(3; .[])]'
# output
[3]: 1,2,3
```

## 11. File Input

Read from a JSON file:

```tq
cat <<'EOF' > data.json
{"name":"Alice","age":30}
EOF
tq '.name' data.json
# output
Alice
```

Read from a TOON file:

```tq
cat <<'EOF' > data.toon
name: Bob
age: 25
EOF
tq '.name' data.toon
# output
Bob
```

Read from multiple files (filter applied to each):

```tq
cat <<'EOF' > a.toon
name: Alice
EOF
cat <<'EOF' > b.toon
name: Bob
EOF
tq '.name' a.toon b.toon
# output
Alice
Bob
```

Read filter from a file with `-f`:

```tq
cat <<'EOF' > filter.jq
.name
EOF
echo '{"name":"Alice"}' | tq -f filter.jq
# output
Alice
```

Use `-` as an explicit stdin argument:

```tq
echo '{"key":"value"}' | tq '.key' -
# output
value
```

## 12. Streaming (`--stream`)

Decompose a JSON document into path-value pairs:

```tq
echo '{"name":"Alice","age":30}' | tq --stream --json -c '.'
# output
[["name"],"Alice"]
[["age"],30]
[["age"]]
```

Decompose a TOON document (same path-value format):

```tq
printf 'name: Alice\nage: 30' | tq --stream --json -c '.'
# output
[["name"],"Alice"]
[["age"],30]
[["age"]]
```

Filter streamed output by path:

```tq
echo '{"name":"Alice","age":30}' | tq --stream --json -c 'select(.[0][0] == "name")'
# output
[["name"],"Alice"]
```

Stream a nested document (paths include array indices):

```tq
echo '{"users":[{"name":"Alice"},{"name":"Bob"}]}' | tq --stream --json -c 'select(length == 2)'
# output
[["users",0,"name"],"Alice"]
[["users",1,"name"],"Bob"]
```

## 13. Format Conversion

TOON to JSON:

```tq
printf 'name: Alice\nage: 30' | tq --json -c '.'
# output
{"age":30,"name":"Alice"}
```

JSON to TOON (auto-detected, TOON output by default):

```tq
echo '{"name":"Alice","age":30}' | tq '.'
# output
age: 30
name: Alice
```

JSON array of objects to TOON table notation:

```tq
echo '{"users":[{"name":"Alice","age":30},{"name":"Bob","age":25}]}' | tq '.'
# output
users[2]{age,name}:
  30,Alice
  25,Bob
```

## 14. Error Handling & Exit Codes

| Exit code | Meaning |
|-----------|---------|
| 0 | Success |
| 2 | Usage or I/O error (bad flag, file not found) |
| 3 | Filter parse or compile error |
| 4 | `--exit-status` with no output produced |
| 5 | Runtime error during filter execution |

Invalid filter syntax (exit 3):

```tq
echo '{}' | tq '.[invalid['
# output error (exit: 3)
parse error
```

Runtime error — wrong type (exit 5):

```tq
echo '42' | tq '.foo'
# output error (exit: 5)
expected an object
```

File not found (exit 2):

```tq
tq '.' /nonexistent/file.json
# output error (exit: 2)
no such file
```

Exit 4 when filter produces no output (`-e`):

```tq
echo '{"a":1}' | tq -e 'select(.a > 5)'
# output (exit: 4)
```

## 15. Common Patterns / Recipes

Extract all strings from a nested structure:

```tq
echo '{"dept":"eng","team":{"lead":"Alice","members":["Bob","Carol"]}}' | tq '[.. | strings]'
# output
[4]: eng,Alice,Bob,Carol
```

Count occurrences of each value:

```tq
printf 'items[5]{category}:\n  fruit\n  veg\n  fruit\n  fruit\n  veg' | tq '[.items[].category] | group_by(.) | map({(.[0]): length}) | add'
# output
fruit: 3
veg: 2
```

Reshape objects — rename and pick fields:

```tq
printf 'users[2]{name,age,city}:\n  Alice,30,Paris\n  Bob,25,London' | tq '.users | map({name, location: .city})'
# output
[2]{location,name}:
  Paris,Alice
  London,Bob
```

Build CSV-like output:

```tq
printf 'users[2]{name,score}:\n  Alice,85\n  Bob,92' | tq -r '.users[] | "\(.name),\(.score)"'
# output
Alice,85
Bob,92
```

Sum a numeric field:

```tq
printf 'orders[3]{amount}:\n  100\n  250\n  75' | tq '[.orders[].amount] | add'
# output
425
```

Filter and transform in one pass:

```tq
printf 'products[3]{name,price,inStock}:\n  Apple,1.5,true\n  Banana,0.8,false\n  Cherry,3.0,true' | tq '[.products[] | select(.inStock) | {name, price}]'
# output
[2]{name,price}:
  Apple,1.5
  Cherry,3
```

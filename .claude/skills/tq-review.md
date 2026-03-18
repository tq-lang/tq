---
name: tq-review
description: "Review Go code for correctness, edge cases, and idiomatic Go. Use when the user says /tq-review, 'review this', 'review the diff', or before shipping a PR."
argument-hint: "[file, directory, or 'diff' to review staged/unstaged changes]"
disable-model-invocation: true
---

# Go Code Review

You are now in code review mode. Review Go source files for correctness, edge
cases, defensive coding, and idiomatic Go. Focus on bugs that pass tests but
break in production or under unexpected input.

## Scope

$ARGUMENTS

- If the user names specific files or directories, scope your review to those.
- If the user says "diff", review only the files changed in `git diff` and
  `git diff --cached` (staged + unstaged). This is the most common use case.
- If no argument is given, review files changed on the current branch vs main:
  `git diff main...HEAD --name-only -- '*.go'`
- For large scopes, use `AskUserQuestion` to narrow down.

## Workflow

1. **Read** — Read every file in scope thoroughly. For diffs, also read the
   surrounding context (the full function, callers, tests). Don't review code
   you haven't read.

2. **Analyse** — Apply the checklist below. For each finding, verify it's real
   by tracing data flow and checking whether tests cover the case.

3. **Report** — Present findings grouped by severity. For each finding, show:
   - The file and line(s)
   - What the issue is (be specific — show the problematic code)
   - Why it matters (what input or scenario triggers the bug)
   - A concrete fix (code snippet)

4. **Classify** — Mark each finding as:
   - **Blocking** — must fix before merging (bugs, data corruption, security)
   - **Should fix** — fix in this PR if easy, otherwise file an issue
   - **Nit** — style or minor improvement, author's discretion

## Checklist — What to Look For

Work through these categories for each file/diff.

### 1. Input validation at boundaries

Functions that accept external input (CLI args, file contents, env vars,
network data) must validate before processing.

**Common misses in this codebase:**
- `--arg`/`--argjson` values that look like flags (`--json`, `-n`) being
  silently consumed as variable names
- Flag values that are empty strings when they shouldn't be
- File paths that are `-` (stdin convention) but handled as regular files

```go
// Bad: silently treats --json as a variable name
name := args[i+1]  // could be "--json"

// Good: reject flag-like names
if strings.HasPrefix(name, "-") {
    return fmt.Errorf("variable name %q must not start with a dash", name)
}
```

### 2. Boolean and condition logic

Subtle bugs in `||` vs `&&`, negation, short-circuit evaluation, and
conditions that are coincidentally correct.

**Patterns to check:**
- `||` where `&&` was intended (or vice versa) — read the condition in plain
  English and verify the logic table
- Conditions that work today because two variables happen to have correlated
  values, but the correlation isn't guaranteed
- `!a || !b` vs `!(a && b)` — De Morgan's law errors
- Early returns that skip cleanup or state updates

```go
// Suspicious: shows extended version if EITHER is set
// But if only one is set, output looks odd: "tq 0.1.0 (abc1234, unknown)"
if commit != "unknown" || date != "unknown" {

// Defensive: require BOTH to be set
if commit != "unknown" && date != "unknown" {
```

### 3. Error handling

Go's explicit error handling is a strength, but only if errors are actually
checked and propagated correctly.

**Patterns to check:**
- Unchecked error returns (the errcheck linter catches most, but not all)
- Errors logged but not returned (swallowed)
- `err` variable shadowed by `:=` in an inner scope
- Error messages that don't include enough context to debug
- `defer` with error returns — the deferred error is silently dropped
- `fmt.Errorf` wrapping without `%w` (loses error chain)

```go
// Bad: shadows outer err, deferred close error lost
func process(path string) error {
    f, err := os.Open(path)
    if err != nil { return err }
    defer f.Close()  // error from Close() is dropped

    if data, err := io.ReadAll(f); err != nil {  // shadows outer err
        return err
    }
}
```

### 4. Slice and map edge cases

**Patterns to check:**
- Nil slice vs empty slice — `len(s) == 0` is true for both, but
  `json.Marshal(nil)` → `null` vs `json.Marshal([]int{})` → `[]`
- Appending to a sub-slice can mutate the original (shared backing array)
- Iterating and modifying the same slice/map
- Map access without comma-ok when zero value is meaningful
- `range` over nil slice/map is safe (no-op), but code after the loop may
  assume the loop body ran at least once

### 5. Concurrency and race conditions

**Patterns to check:**
- Shared state accessed without synchronization
- Goroutine leaks (launched but never joined or cancelled)
- Channel operations that can deadlock
- `sync.WaitGroup` Add/Done mismatch
- Reading from a closed channel (returns zero value — silent bug)

### 6. Resource lifecycle

**Patterns to check:**
- `os.Open` / `os.Create` without `defer f.Close()`
- `http.Response.Body` not closed
- `context.WithCancel` / `WithTimeout` without calling cancel
- Temp files created but not cleaned up on error paths
- `defer` in a loop (defers pile up, only run when function returns)

### 7. String and encoding

**Patterns to check:**
- `len(s)` gives bytes, not runes — use `utf8.RuneCountInString` for character
  count
- `s[i]` gives a byte, not a rune — use `range s` for rune iteration
- String concatenation in a loop — use `strings.Builder`
- `fmt.Sprintf` when `strconv` would be clearer and faster
- Raw string literals with backticks that contain backticks (can't be escaped)

### 8. API contract mismatches

**Patterns to check:**
- pflag/cobra quirks: `StringArray` vs `StringSlice` (comma splitting),
  `Changed()` for detecting explicit flag use
- `io.Reader` / `io.Writer` contracts: partial reads are valid, `Write` must
  return `len(p)` or an error
- `json.Decoder` vs `json.Unmarshal` — decoder handles streams, unmarshal
  expects complete input
- `os.Exit` bypasses deferred functions — use a `run() int` pattern instead

### 9. Test quality

**Patterns to check:**
- Tests that verify behavior coincidentally (test passes but for the wrong
  reason)
- Substring assertions that are too loose (`Contains("a")` matches anything)
- Missing error case tests — happy path is tested but error paths aren't
- Table-driven tests where a new case should be added for the code being changed
- Test helpers that call `t.Fatal` from a goroutine (must use `t.Error` +
  return instead)

### 10. Go-specific idioms

**Patterns to check:**
- Exported functions/types without doc comments (golint)
- Stuttering names: `package user` with `type UserService` → `type Service`
- `interface{}` instead of `any` (Go 1.18+)
- Unnecessary pointer receivers on small structs
- `init()` functions with side effects (makes testing hard)
- Constants that should be iota enums

## Output Format

Present your review as:

```markdown
## Review: <scope description>

### Blocking

#### 1. <title> — `file.go:NN`

<description of the issue>

```go
// Current
<problematic code>

// Suggested
<fixed code>
```

**Why:** <what breaks and when>

---

### Should Fix

#### 1. <title> — `file.go:NN`
...

### Nits

- `file.go:NN` — <one-liner description and fix>
```

## Calibration

- **Be precise.** Every finding must reference specific code with line numbers.
  Don't say "consider adding validation" — say where, what input, what check.
- **Prove it.** For each finding, explain the concrete scenario that triggers
  the bug. "If someone passes `--arg --json value`" is good. "This could
  theoretically cause issues" is not.
- **Don't pad.** If the code is solid, say so. A review with zero findings is
  a valid outcome. Don't manufacture nits to seem thorough.
- **Respect the author's style.** Don't flag things that are a matter of
  preference unless they cause real confusion. `if err != nil { return err }` on
  one line vs three is not worth mentioning.
- **Check tests before flagging.** If you think an edge case is unhandled, check
  whether a test already covers it. If so, mention the test exists and move on.
- **Separate suggestions from issues.** "This works but could be cleaner" is a
  nit. "This breaks when input contains dashes" is blocking. Don't mix them.

## Critical Rules

- **Read before reviewing.** Never comment on code you haven't read. Read the
  full function, its callers, and its tests.
- **Don't fix code.** Your job is to identify issues and suggest fixes. Don't
  edit files unless the user asks you to apply your suggestions.
- **Ask when uncertain.** If you're unsure whether something is a real issue or
  intentional, use `AskUserQuestion` rather than guessing.
- **No false positives.** A false positive wastes more of the author's time than
  a missed issue. If you're less than 80% confident, skip it or downgrade to a
  nit with a question mark.

# Phase 2: Fix

Apply fixes for each classified failure from the triage report. Use TDD: write
a failing test first (when applicable), then fix, then verify.

## Input

The triage report from Phase 1, with user approval to proceed.

## Fix Procedure by Classification

### Real Bugs

1. **Red** — Write a test that reproduces the bug:
   ```go
   func TestFoo_NilReader(t *testing.T) {
       // This should not panic
       result := processLine(nil, ...)
       // assert expected behavior
   }
   ```
2. **Green** — Fix the code to make the test pass.
3. **Verify** — Run the failing test: `go test -run <TestName> ./...`
4. **Commit** — `fix: <description>` with `Closes #N` if applicable.

### Flaky Tests

1. **Identify** — Find the source of non-determinism:
   - Map iteration order
   - Goroutine scheduling
   - Time-dependent assertions (`time.Sleep`, deadlines)
   - Temp file race conditions
   - Shared test state (missing `t.Parallel()` isolation)
2. **Fix** — Make the test deterministic:
   - Sort output before comparing
   - Use channels instead of sleep
   - Use `t.TempDir()` for isolation
   - Add proper synchronization
3. **Verify** — Run 10 times: `go test -count=10 -run <TestName> ./...`
4. **Commit** — `test: fix flaky <TestName>`

### Test Bugs

1. **Verify** — Confirm the code behavior is correct by reading the code
   and running it manually.
2. **Fix** — Update the test assertion to match correct behavior.
3. **Verify** — Run the test: `go test -run <TestName> ./...`
4. **Commit** — `test: fix assertion in <TestName>`

### Env Issues

1. **Identify** — What's different between CI and local:
   - OS (Ubuntu in CI vs macOS locally)
   - Go version
   - Available tools (golangci-lint version)
   - File permissions, temp directory paths
2. **Fix** — Update CI config (`.github/workflows/ci.yml`) or add build
   tags / skip conditions.
3. **Verify** — Check the fix makes sense for both environments.
4. **Commit** — `ci: fix <description>`

### Lint Issues

1. **Fix** — Fix the code. Never add exclusions (Boy Scout Rule).
2. **Verify** — `golangci-lint run`
3. **Commit** — `fix(lint): <description>`

## Final Verification

After all fixes, run the full quality gate:
```bash
golangci-lint run && go vet ./... && go build ./cmd/tq && go test -race -short -count=1 ./...
```

## Summary Report

```markdown
## Fix Summary

| # | Issue | Classification | Fix | Commit |
|---|---|---|---|---|
| 1 | TestFoo NPE | Real bug | Added nil guard in processLine | `abc1234` |
| 2 | TestBar ordering | Flaky | Sorted output before comparison | `def5678` |
| 3 | errcheck stream.go | Lint | Added `_ =` to discarded return | `ghi9012` |

**Quality gate:** PASSED
**Total commits:** 3
```

# Phase 1: Triage

Gather failure data, reproduce, classify, and report.

## Step 1: Gather Failure Data

If a PR number is given:
```bash
gh pr checks <N>
gh run list --branch <branch> --limit 3
```

If "ci" or no argument:
```bash
gh run list --limit 5
```

For each failed run, download logs:
```bash
gh run view <run-id> --log-failed
```

Parse the logs to extract:
- Which step failed (lint, vet, build, test)
- The specific error message
- The test name and package (if test failure)

## Step 2: Reproduce Locally

Run the full quality gate:
```bash
golangci-lint run
go vet ./...
go build ./cmd/tq
go test -race -short -count=1 ./...
```

If specific tests fail, re-run them 3 times to check for flakiness:
```bash
go test -race -count=3 -run <TestName> ./path/to/package
```

Record the result of each run (pass/fail + error output).

## Step 3: Classify Each Failure

Apply these criteria:

| Classification | Criteria | Action |
|---|---|---|
| **Real bug** | Fails consistently (3/3 runs), same error | Fix code in Phase 2 |
| **Flaky test** | Fails intermittently (1/3 or 2/3), timing or ordering dependent | Fix test in Phase 2 |
| **Test bug** | Test assertion is wrong, code behavior is correct | Fix test in Phase 2 |
| **Env issue** | Only fails in CI (permissions, OS-specific, missing tools) | Fix CI config in Phase 2 |
| **Lint issue** | golangci-lint failure, code compiles and tests pass | Fix code in Phase 2 |

For each failure, determine:
- **Root cause** — what's actually wrong (not just the symptom)
- **Confidence** — High (proven), Medium (likely), Low (needs investigation)
- **Affected files** — which source files need changes

## Step 4: Present Triage Report

```markdown
## Triage Report

| # | Test/Check | Classification | Confidence | Root Cause |
|---|---|---|---|---|
| 1 | TestFoo | Real bug | High | NPE when input is nil |
| 2 | TestBar | Flaky | Medium | Timing-dependent goroutine ordering |
| 3 | errcheck | Lint | High | Unchecked return in stream.go:42 |

### Details

#### 1. TestFoo — Real Bug (High confidence)
- **Error:** `panic: runtime error: invalid memory address`
- **Reproduction:** `go test -run TestFoo ./cmd/tq` — fails 3/3 times
- **Root cause:** `processLine` doesn't check for nil reader when stream is empty
- **Affected:** `cmd/tq/processing.go:45`, needs nil guard

#### 2. TestBar — Flaky (Medium confidence)
- **Error:** `expected "Alice" but got "Bob"` (sometimes passes)
- **Reproduction:** fails 1/3 times
- **Root cause:** map iteration order is non-deterministic
- **Affected:** `cmd/tq/main_test.go:120`, need sorted output comparison
```

Ask the user: "Found N issues. Want me to fix them? (Y/n)"

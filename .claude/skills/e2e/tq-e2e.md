---
name: tq-e2e
description: "Triage and fix test/CI failures: classify as flaky vs real, then fix. Use: /tq-e2e or /tq-e2e <PR number>"
---

# E2E Test Triage and Fix

Two-phase pipeline: triage test failures to classify them, then fix real bugs.

## Input

$ARGUMENTS — optional PR number or "ci" to check the latest CI run.

## Phase 1: Triage

### 1. Gather Failure Data

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

### 2. Reproduce Locally

Run the full quality gate to reproduce:
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

### 3. Classify Each Failure

For each failure, determine:

| Classification | Criteria | Action |
|---|---|---|
| **Real bug** | Fails consistently, same error each time | Fix in Phase 2 |
| **Flaky test** | Fails intermittently, different errors or timing-dependent | Fix the test, not the code |
| **Test bug** | Test assertion is wrong, code is correct | Fix the test |
| **Env issue** | Only fails in CI (permissions, missing tools, OS-specific) | Fix CI config |
| **Lint issue** | golangci-lint failure | Fix the code (Boy Scout Rule) |

### 4. Present Triage Report

```markdown
## Triage Report

| # | Test/Check | Classification | Confidence | Details |
|---|---|---|---|---|
| 1 | TestFoo | Real bug | High | NPE when input is nil |
| 2 | TestBar | Flaky | Medium | Passes 2/3 runs, timing-dependent |

### Details

#### 1. TestFoo -- Real Bug
- **Error:** `panic: runtime error: invalid memory address`
- **Root cause:** `processLine` doesn't check for nil reader
- **Reproduction:** `go test -run TestFoo ./cmd/tq` -- fails 3/3 times
```

Ask the user: "Found N issues. Want me to fix them?"

## Phase 2: Fix

For each classified failure (user-approved):

### Real Bugs
1. Write a failing test that reproduces the bug.
2. Fix the code to make the test pass.
3. Run the full quality gate.
4. Commit: `fix: <description>`

### Flaky Tests
1. Identify the source of non-determinism (timing, goroutine ordering, temp files).
2. Fix the test to be deterministic.
3. Run the test 10 times: `go test -count=10 -run <TestName> ./...`
4. Commit: `test: fix flaky <TestName>`

### Test Bugs
1. Verify the code behavior is correct.
2. Fix the test assertion.
3. Commit: `test: fix assertion in <TestName>`

### Lint Issues
1. Fix the code (never add exclusions — Boy Scout Rule).
2. Run `golangci-lint run` to verify.
3. Commit: `fix(lint): <description>`

## Final Verification

After all fixes:
```bash
golangci-lint run && go vet ./... && go build ./cmd/tq && go test -race -short -count=1 ./...
```

Report a summary table:

```markdown
## Fix Summary

| # | Issue | Fix | Commit |
|---|---|---|---|
| 1 | TestFoo NPE | Added nil check in processLine | abc1234 |
| 2 | TestBar flaky | Replaced sleep with channel sync | def5678 |

Quality gate: PASSED
```

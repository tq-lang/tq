---
name: tq-check
description: "Run the tq quality gate (lint, vet, test, build). Mirrors CI exactly. Use when the user says /tq-check, 'run checks', or before committing."
---

# tq Quality Gate

Run the same checks as CI (`.github/workflows/ci.yml`). Stop on first failure — never fix anything.

## Steps (run in order, stop on first failure)

### 1. Lint

```bash
golangci-lint run
```

This is the single biggest source of CI failures. It runs errcheck, govet, and many other linters configured in the repo.

### 2. Vet

```bash
go vet ./...
```

### 3. Build

```bash
go build ./cmd/tq
```

### 4. Test (race + short)

```bash
go test -race -short -count=1 ./...
```

This runs all tests including `TestDocs` (doc example tests) but skips the slow memory tests. The `-race` flag matches CI.

## Rules

- NEVER fix, edit, or modify any file — you are a read-only reporter
- NEVER re-run a failing step — stop on first failure
- Show the full error output for any failing step
- End with: `RESULT: ALL PASSED` or `RESULT: FAILURES DETECTED`
- If golangci-lint is not installed, report it and fail — it's required for this repo

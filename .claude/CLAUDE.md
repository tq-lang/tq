# tq

Command-line TOON/JSON processor. Like jq, but for TOON.

## Quality gate (must pass before committing)

Run `/tq-check` which runs, in order:

1. `golangci-lint run` — required, includes errcheck, gocritic, errorlint, staticcheck
2. `go vet ./...`
3. `go build ./cmd/tq`
4. `go test -race -short -count=1 ./...` — includes TestDocs (doc example tests)

This mirrors `.github/workflows/ci.yml` exactly.

## Boy Scout Rule

We take full ownership of the codebase. There is no "pre-existing" code — if you touch a file and see issues, fix them. Specifically:

- **Never exclude lint findings.** If a linter flags something, fix it. Do not add exclusions, `//nolint` comments, or raise thresholds to avoid fixing code.
- **Refactor complex functions.** gocyclo is set to 10. If a function exceeds it, break it into smaller functions with clear responsibilities.
- **Use `errors.Is`/`errors.As`** instead of `==` for error comparisons (errorlint).
- **Don't shadow imports.** If a parameter name conflicts with an import (e.g. `bytes`), rename the parameter.
- If a fix is too large for the current PR, open a GitHub issue with `gh issue create` — but never silently skip it.

## Conventions

- Conventional Commits: `feat`, `fix`, `refactor`, `build`, `ci`, `chore`, `docs`, `test`
- Discard return values explicitly with `_ =` or `_, _ =` (errcheck linter)
- Doc examples in `docs/*.md` are tested by `TestDocs` — keep code blocks in sync
- `--arg`/`--argjson` use jq-style syntax: `--arg NAME VALUE` (two positional args)

## Linters (`.golangci.yml`)

Enabled beyond defaults: gocritic (diagnostic + style + performance), errorlint, nilerr, reassign, wastedassign, forcetypeassert, exhaustive, errchkjson, perfsprint, forbidigo, nolintlint, bodyclose, asciicheck, bidichk, copyloopvar, durationcheck, errname, goconst, nakedret, usestdlibvars, whitespace, makezero, intrange, mirror, tparallel, dupl (75 tokens), maintidx (under 20), funlen (60 lines / 40 statements). `errcheck.check-type-assertions: true`. gocyclo max complexity: 10 (Go best practice — refactor if exceeded, never raise). Source files max 500 LOC, test files max 1000 LOC.

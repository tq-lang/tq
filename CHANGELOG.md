# Changelog

## 2026-03-17

### Features
- add SBOM + provenance attestations (#27) (6addb9e)
- grouped help, --quiet flag, env/docs in help output (#19) (2947185)
- native TOON streaming with auto-detection and filter warnings (#18) (528b8e5)

### Fixes
- auto-sync changelog on main and drop PR stale check (#29) (3da6c20)

### Refactor
- split main into focused modules (#25) (c3c2976)

### Build
- bump actions/checkout from 4 to 6 (#15) (41d47be)
- bump actions/setup-go from 5 to 6 (#13) (b857102)
- bump golangci/golangci-lint-action from 6 to 9 (#16) (a06d964)
- bump goreleaser/goreleaser-action from 6 to 7 (#14) (84c9fd7)

### CI
- run changelog verification only on pull requests (#26) (091067f)
- use GitHub App token for homebrew tap updates (#31) (b0d248d)

### Chores
- add CODEOWNERS (#28) (9397cac)
- version hooks and enforce changelog checks (#24) (47d16e2)

## 2026-03-16

### Features
- Homebrew tap for brew install tq-lang/tap/tq (#9) (05eb11d)

### CI
- add Dependabot for GitHub Actions and Go modules (#12) (1e5acd6)

## 2026-03-13

### Features
- integration-tested cheatsheet with 80+ runnable examples (8e3e8b4)

### Docs
- add guide, recipes, errors, and vs-jq with 220+ tested examples (5bf3b7b)

## 2026-02-27

### Features
- streaming JSON tokenizer for --stream mode (O(depth) memory) (033fa1f)

### Refactor
- consolidate stream dispatch into resolveReader (1d003e9)

### Docs
- note natural split points in input package doc (b42388b)
- remove roadmap section from README (0901ffb)

### Build
- add VERSION ldflags to Makefile and document build (5c6cd5d)

### CI
- add coverage for CLI integration tests (bd31a67)

## 2026-02-26

### Features
- add PNG versions of project logo assets (2c9f523)
- add project logo assets in SVG format (250a9f2)
- add streaming support for multi-document input and --stream flag (201b56d)

### Fixes
- check os.WriteFile error returns for errcheck lint (6984240)

### Refactor
- deduplicate read loops with filterAll/slurpAll helpers (1c7e392)
- extract config struct, helpers, and exit code constants (cfaa080)
- extract terminateLine and indentString in output package (a583979)
- rename StreamReader to Reader, remove dead Parse code (1226bfd)
- replace flag with pflag and improve CLI UX (#3) (a371e81)

### Style
- fix gofmt alignment in run() variable block (16c4078)

### Tests
- add table-driven test suite for all packages (#1) (4fb6a3c)
- strengthen assertions and enable verbose CI output (8c9e068)

## 2026-02-19

### Features
- implement tq CLI with full jq filter support (b63b08e)

### Fixes
- pin Go version to 1.24 for golangci-lint compatibility (693cd08)

### Docs
- add comprehensive README and Go build configuration (8ca0e16)

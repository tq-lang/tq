# Changelog

## 2026-02-27

### Docs
- note natural split points in input package doc (b42388b)

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

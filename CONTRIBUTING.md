# Contributing to tq

Thank you for your interest in contributing to tq! This document provides guidelines to make the process smooth for everyone.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/<your-username>/tq.git`
3. Create a branch: `git checkout -b my-feature`
4. Make your changes
5. Run checks: `make check`
6. Commit using [Conventional Commits](https://www.conventionalcommits.org/): `git commit -m "feat: add something"`
7. Push and open a pull request

## Development

### Prerequisites

- Go 1.23+
- [golangci-lint](https://golangci-lint.run/)

### Build

```bash
make build
```

### Test

```bash
make test
```

### Lint

```bash
make lint
```

### Run all checks

```bash
make check
```

### Documentation Tests

All code examples in `docs/*.md` are integration-tested — they run against the real `tq` binary on every `make test`. If you change tq's output format or CLI behaviour, update the affected examples and run `make test-docs` to verify them.

Use fenced ` ```tq ` blocks with `# output` and optional `# output error (exit: N)` markers:

````markdown
```tq
echo '{"name":"Alice"}' | tq '.name'
# output
Alice
```
````

Only `tq`, `echo`, `printf`, and `cat` heredocs are allowed — examples are executed by a Go mini-interpreter, not a shell. See `docs/cheatsheet.md` for the full format.

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/). Prefix your commit message with a type:

- `feat:` — new feature
- `fix:` — bug fix
- `docs:` — documentation only
- `test:` — adding or updating tests
- `refactor:` — code change that neither fixes a bug nor adds a feature
- `chore:` — maintenance tasks

## Pull Requests

- Keep PRs focused — one feature or fix per PR
- Include tests for new functionality
- Update documentation if behavior changes
- Ensure `make check` passes before submitting

## Reporting Issues

- Use the [bug report template](https://github.com/tq-lang/tq/issues/new?template=bug_report.md) for bugs
- Use the [feature request template](https://github.com/tq-lang/tq/issues/new?template=feature_request.md) for ideas

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md).

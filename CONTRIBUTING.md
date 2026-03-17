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

- Go 1.24+
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

### Changelog Workflow

`CHANGELOG.md` is enforced in CI on pull requests. Generate it locally before opening or updating a PR:

```bash
make generate-changelog
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

## Release Setup

Releases are automated via GoReleaser on tag push. The Homebrew formula is published to [`tq-lang/homebrew-tap`](https://github.com/tq-lang/homebrew-tap) automatically, but requires a one-time secret setup:

1. Go to https://github.com/settings/personal-access-tokens/new
2. Create a **fine-grained PAT** with:
   - **Resource owner**: `tq-lang`
   - **Repository access**: Only select `tq-lang/homebrew-tap`
   - **Permissions**: Contents → **Read and write**
3. Go to https://github.com/tq-lang/tq/settings/secrets/actions
4. Add a new repository secret:
   - **Name**: `HOMEBREW_TAP_TOKEN`
   - **Value**: the PAT from step 2

After this, any `v*` tag push will build binaries, create a GitHub release, and update the Homebrew formula.

Release hardening policy (action SHA pinning now, SBOM/provenance phased in) is documented in [docs/adr-002-release-supply-chain.md](docs/adr-002-release-supply-chain.md).
Consumer verification steps for release SBOMs and provenance attestations are documented in [docs/release-verification.md](docs/release-verification.md).

## Reporting Issues

- Use the [bug report template](https://github.com/tq-lang/tq/issues/new?template=bug_report.md) for bugs
- Use the [feature request template](https://github.com/tq-lang/tq/issues/new?template=feature_request.md) for ideas

## Reporting Security Issues

See [SECURITY.md](SECURITY.md). Please use private reporting for vulnerabilities.

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md).

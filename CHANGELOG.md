# Changelog

## [0.1.0-rc3](https://github.com/tq-lang/tq/releases/tag/v0.1.0-rc3) — 2026-03-17

### Build

- use unique SBOM artifact names ([19125aa](https://github.com/tq-lang/tq/commit/19125aaebbf8417d0fabde1bdf5307c2f5ae395c))
## [0.1.0-rc2](https://github.com/tq-lang/tq/releases/tag/v0.1.0-rc2) — 2026-03-17

### CI

- install syft for SBOM generation ([82f009b](https://github.com/tq-lang/tq/commit/82f009b412b416c11f98bc99c1a7287c167b0eda))
## [0.1.0-rc1](https://github.com/tq-lang/tq/releases/tag/v0.1.0-rc1) — 2026-03-17

### Build

- remove invalid GoReleaser release_notes_file setting ([017cc44](https://github.com/tq-lang/tq/commit/017cc4479f00e38503e09c8adcb5690472bb0b58))
- replace changelog shell script with git-cliff ([09dd0ce](https://github.com/tq-lang/tq/commit/09dd0ce10efdbde9b646d7620bc9d026655fdd8e))
- bump actions/setup-go from 5 to 6 ([#13](https://github.com/tq-lang/tq/pull/13)) ([b857102](https://github.com/tq-lang/tq/commit/b85710294337cc58a06b124804ced343976dab30))
- bump goreleaser/goreleaser-action from 6 to 7 ([#14](https://github.com/tq-lang/tq/pull/14)) ([84c9fd7](https://github.com/tq-lang/tq/commit/84c9fd71e16ed9cab038b238573a8a0ad20c62cf))
- bump golangci/golangci-lint-action from 6 to 9 ([#16](https://github.com/tq-lang/tq/pull/16)) ([a06d964](https://github.com/tq-lang/tq/commit/a06d964a5d09607ef8856a056dc8d128aebfae04))
- bump actions/checkout from 4 to 6 ([#15](https://github.com/tq-lang/tq/pull/15)) ([41d47be](https://github.com/tq-lang/tq/commit/41d47be311097a766c0f1d34cd976c51e39e79fe))
- add VERSION ldflags to Makefile and document build ([5c6cd5d](https://github.com/tq-lang/tq/commit/5c6cd5d5bdd50fb6acfbbbeef809c5648317f382))

### CI

- use GitHub App token for homebrew tap updates ([#32](https://github.com/tq-lang/tq/pull/32)) ([9437b36](https://github.com/tq-lang/tq/commit/9437b36d14adf61055f62b88b194a1dd905c58dc))
- use GitHub App token for homebrew tap updates ([#31](https://github.com/tq-lang/tq/pull/31)) ([b0d248d](https://github.com/tq-lang/tq/commit/b0d248d8ee2466c901228d42bc92f8eb75bd1158))
- run changelog verification only on pull requests ([#26](https://github.com/tq-lang/tq/pull/26)) ([091067f](https://github.com/tq-lang/tq/commit/091067fc9dda84abc6d4dfb3f571c36944ebd64e))
- add Dependabot for GitHub Actions and Go modules ([#12](https://github.com/tq-lang/tq/pull/12)) ([1e5acd6](https://github.com/tq-lang/tq/commit/1e5acd65e3fd87ee83dd74dbe7717adb1a4b833d))
- add coverage for CLI integration tests ([bd31a67](https://github.com/tq-lang/tq/commit/bd31a67d96f64c13bcd28a6bdc51532dee34824f))

### Chores

- add CODEOWNERS ([#28](https://github.com/tq-lang/tq/pull/28)) ([9397cac](https://github.com/tq-lang/tq/commit/9397caca19a1cccd6c7e54ba8b8024fd3b3b7a1c))
- version hooks and enforce changelog checks ([#24](https://github.com/tq-lang/tq/pull/24)) ([47d16e2](https://github.com/tq-lang/tq/commit/47d16e2931e72634415b5b2a047b9c6b38d55571))
- bootstrap project ([6480239](https://github.com/tq-lang/tq/commit/6480239e58b79d5988f7727dd5263c8bb2ff81e8))

### Docs

- add guide, recipes, errors, and vs-jq with 220+ tested examples ([5bf3b7b](https://github.com/tq-lang/tq/commit/5bf3b7b35e92268f73a7aa3d2ce18880bb727760))
- remove roadmap section from README ([0901ffb](https://github.com/tq-lang/tq/commit/0901ffbbbc4d32d6c440510a43114192ddf64f2e))
- note natural split points in input package doc ([b42388b](https://github.com/tq-lang/tq/commit/b42388b9e3bce4ee8e6d882de2a5ee37a7c13050))
- add comprehensive README and Go build configuration ([8ca0e16](https://github.com/tq-lang/tq/commit/8ca0e168c7b7a24491ea49098ff03823ed01abd6))

### Features

- add SBOM + provenance attestations ([#27](https://github.com/tq-lang/tq/pull/27)) ([6addb9e](https://github.com/tq-lang/tq/commit/6addb9e30a5179a8c2f31b7bce83b10d9dd6bad3))
- grouped help, --quiet flag, env/docs in help output ([#19](https://github.com/tq-lang/tq/pull/19)) ([2947185](https://github.com/tq-lang/tq/commit/29471858e190aacb37b66837ca795c7e3ad99388))
- native TOON streaming with auto-detection and filter warnings ([#18](https://github.com/tq-lang/tq/pull/18)) ([528b8e5](https://github.com/tq-lang/tq/commit/528b8e53683e0d88bb82ba01a3e03f08f0749598))
- Homebrew tap for brew install tq-lang/tap/tq ([#9](https://github.com/tq-lang/tq/pull/9)) ([05eb11d](https://github.com/tq-lang/tq/commit/05eb11d7587defe25fb13dde9706df688f87a5a6))
- integration-tested cheatsheet with 80+ runnable examples ([8e3e8b4](https://github.com/tq-lang/tq/commit/8e3e8b45acc09db99e36b70730b0d020a937b607))
- streaming JSON tokenizer for --stream mode (O(depth) memory) ([033fa1f](https://github.com/tq-lang/tq/commit/033fa1f628c2d61e854b2229c0dae222d072e05c))
- add streaming support for multi-document input and --stream flag ([201b56d](https://github.com/tq-lang/tq/commit/201b56d7c56e65f2551a84ec313f1c07e49ea8a2))
- add PNG versions of project logo assets ([2c9f523](https://github.com/tq-lang/tq/commit/2c9f5235bf1544f9cd61db611ca691f1122d94ea))
- add project logo assets in SVG format ([250a9f2](https://github.com/tq-lang/tq/commit/250a9f2ae59323e5b205c5b8f99a1228817ce761))
- implement tq CLI with full jq filter support ([b63b08e](https://github.com/tq-lang/tq/commit/b63b08e4d83fcb2acbc501eb742a8df2229345e0))

### Fixes

- auto-sync changelog on main and drop PR stale check ([#29](https://github.com/tq-lang/tq/pull/29)) ([3da6c20](https://github.com/tq-lang/tq/commit/3da6c20ccdd7da1d48b1cff95f6fab2e1f50d63f))
- check os.WriteFile error returns for errcheck lint ([6984240](https://github.com/tq-lang/tq/commit/6984240a516441c2a0fe953ab90447187ffdc42b))
- pin Go version to 1.24 for golangci-lint compatibility ([693cd08](https://github.com/tq-lang/tq/commit/693cd0898b16868536b68a464c5c0c0c36663dbd))

### Refactor

- split main into focused modules ([#25](https://github.com/tq-lang/tq/pull/25)) ([c3c2976](https://github.com/tq-lang/tq/commit/c3c297600f131cc4b08e47c3ff2514d122fcdc34))
- consolidate stream dispatch into resolveReader ([1d003e9](https://github.com/tq-lang/tq/commit/1d003e998ae69a004a5c831753a6134999438e11))
- rename StreamReader to Reader, remove dead Parse code ([1226bfd](https://github.com/tq-lang/tq/commit/1226bfd2c261d1285e1002ed13789b78bfa8a10a))
- extract terminateLine and indentString in output package ([a583979](https://github.com/tq-lang/tq/commit/a58397909f8505a019d15a68be838b7fbbaf1b1f))
- extract config struct, helpers, and exit code constants ([cfaa080](https://github.com/tq-lang/tq/commit/cfaa0804579a45be25be08045dc88fb5b7573bb2))
- deduplicate read loops with filterAll/slurpAll helpers ([1c7e392](https://github.com/tq-lang/tq/commit/1c7e392ffcd10d85c2c2762b23eb4533b00fb98f))
- replace flag with pflag and improve CLI UX ([#3](https://github.com/tq-lang/tq/pull/3)) ([a371e81](https://github.com/tq-lang/tq/commit/a371e814ff6d415d8981d9e8d1936faa6419ac36))

### Style

- fix gofmt alignment in run() variable block ([16c4078](https://github.com/tq-lang/tq/commit/16c4078ee7098d37c9975587f06850ebbd761103))

### Tests

- strengthen assertions and enable verbose CI output ([8c9e068](https://github.com/tq-lang/tq/commit/8c9e068fef14ad649994a5d953eedbef58fd4b9e))
- add table-driven test suite for all packages ([#1](https://github.com/tq-lang/tq/pull/1)) ([4fb6a3c](https://github.com/tq-lang/tq/commit/4fb6a3cf9990d8dee4ca1f0c7b189f8e116fd2b8))


#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"

git config core.hooksPath .githooks

chmod +x \
  "$REPO_ROOT/.githooks/pre-commit" \
  "$REPO_ROOT/.githooks/commit-msg" \
  "$REPO_ROOT/scripts/generate-changelog.sh"

echo "Configured git hooks path: .githooks"
echo "Enabled hooks: pre-commit, commit-msg"

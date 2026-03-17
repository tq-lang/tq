#!/usr/bin/env bash
# Generate CHANGELOG.md from git log using Conventional Commits format.
# Groups entries by date, then by commit type.
#
# Note: this script intentionally excludes HEAD when possible. It is run by
# pre-commit before the new commit exists, so the changelog naturally lags by
# one commit and is finalized on the next commit.

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
CHANGELOG="$REPO_ROOT/CHANGELOG.md"
LOG_REF="HEAD~1"

# In PR CI we run on a synthetic merge commit. Use the PR parent chain so
# changelog verification reflects what contributors generate locally.
if git rev-parse --verify HEAD^2 >/dev/null 2>&1; then
  LOG_REF="HEAD^2~1"
  if ! git rev-parse --verify "$LOG_REF" >/dev/null 2>&1; then
    LOG_REF="HEAD^2"
  fi
fi

if ! git rev-parse --verify "$LOG_REF" >/dev/null 2>&1; then
  LOG_REF="HEAD"
fi

# Map conventional commit types to section headers
type_label() {
  case "$1" in
    feat)     echo "Features" ;;
    fix)      echo "Fixes" ;;
    docs)     echo "Docs" ;;
    style)    echo "Style" ;;
    refactor) echo "Refactor" ;;
    perf)     echo "Performance" ;;
    test)     echo "Tests" ;;
    build)    echo "Build" ;;
    ci)       echo "CI" ;;
    chore)    echo "Chores" ;;
    revert)   echo "Reverts" ;;
    *)        echo "" ;;
  esac
}

# Section display order (lower = higher)
type_order() {
  case "$1" in
    Features)    echo 1 ;;
    Fixes)       echo 2 ;;
    Performance) echo 3 ;;
    Refactor)    echo 4 ;;
    Docs)        echo 5 ;;
    Style)       echo 6 ;;
    Tests)       echo 7 ;;
    Build)       echo 8 ;;
    CI)          echo 9 ;;
    Chores)      echo 10 ;;
    Reverts)     echo 11 ;;
    *)           echo 99 ;;
  esac
}

{
  echo "# Changelog"
  echo ""

  current_date=""

  # Read commits: date<TAB>hash<TAB>subject
  while IFS=$'\t' read -r date hash subject; do
    # Parse conventional commit: type(scope): description  or  type: description
    if [[ "$subject" =~ ^([a-zA-Z]+)(\(.+\))?!?:\ (.+)$ ]]; then
      cc_type="${BASH_REMATCH[1]}"
      description="${BASH_REMATCH[3]}"
      short_hash="${hash:0:7}"
      label="$(type_label "$cc_type")"
      [[ -z "$label" ]] && continue

      # Collect entries per date+section
      echo "${date}|${label}|$(type_order "$label")|${description}|${short_hash}"
    fi
  done < <(git log "$LOG_REF" --pretty=format:"%ad%x09%H%x09%s" --date=short) |
  sort -t'|' -k1,1r -k3,3n |
  while IFS='|' read -r date label _order description short_hash; do
    if [[ "$date" != "$current_date" ]]; then
      [[ -n "$current_date" ]] && echo ""
      echo "## $date"
      current_date="$date"
      current_section=""
    fi

    if [[ "$label" != "${current_section:-}" ]]; then
      echo ""
      echo "### $label"
      current_section="$label"
    fi

    echo "- ${description} (${short_hash})"
  done
} > "$CHANGELOG"

#!/bin/sh
# PreToolUse(Bash): block git hook-bypass via --no-verify.
#
# Protects the quality gate (pre-commit / pre-push hooks and, by extension,
# the /tq-check gate) from being skipped. Position-aware: single- and
# double-quoted segments are stripped first, so `--no-verify` appearing
# inside a commit message body ( -m "...--no-verify..." ) is NOT matched.
#
# Hooks must fail open: any parse/transport error exits 0 (never blocks).

cmd=$(jq -r '.tool_input.command // ""' 2>/dev/null) || exit 0
[ -z "$cmd" ] && exit 0

# Only relevant to git.
printf '%s' "$cmd" | grep -Eq '\bgit\b' || exit 0

# Remove quoted segments so message bodies can't trigger a false positive.
# Flatten newlines first so multi-line -m "..."/heredoc message bodies are
# stripped as a single quoted span (sed's [^"]* never crosses a newline).
stripped=$(printf '%s' "$cmd" | tr '\n' ' ' | sed -e "s/'[^']*'//g" -e 's/"[^"]*"//g')

if printf '%s' "$stripped" | grep -Eq -- '(^|[[:space:]])--no-verify([[:space:]]|=|$)'; then
  echo "[tq] BLOCKED: --no-verify bypasses the quality gate (pre-commit/pre-push hooks)." >&2
  echo "[tq] Run /tq-check, fix what it reports, then commit/push normally." >&2
  exit 2
fi

exit 0

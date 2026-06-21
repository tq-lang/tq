#!/bin/sh
# PostToolUse(Edit|Write): auto-format the edited Go file with gofmt.
#
# Fires only on *.go files (the rest exit immediately), so the cost is one
# gofmt invocation (~10-30ms) per Go edit. Non-blocking: always exits 0.

f=$(jq -r '.tool_input.file_path // ""' 2>/dev/null) || exit 0
case "$f" in
  *.go) [ -f "$f" ] && command -v gofmt >/dev/null 2>&1 && gofmt -w "$f" ;;
esac
exit 0

#!/bin/sh
# PreToolUse(Read): block reads of secret-bearing files.
#
# Defense-in-depth so credentials never land in the transcript. Example/
# sample/template env files are allowed. If a read is genuinely needed,
# view the file in your own shell. Fails open on parse error.

f=$(jq -r '.tool_input.file_path // ""' 2>/dev/null) || exit 0
[ -z "$f" ] && exit 0

case "$f" in
  *.env.example|*.env.sample|*.env.template|*.env.dist) exit 0 ;;
  *.env|*.env.*|*.pem|*.key|*.p12|*.pfx|*_rsa|*_dsa|*_ed25519|*/credentials|*/.netrc)
    echo "[tq] BLOCKED reading secret-bearing file: $f" >&2
    echo "[tq] If this is intentional, open it in your own shell instead." >&2
    exit 2 ;;
esac
exit 0

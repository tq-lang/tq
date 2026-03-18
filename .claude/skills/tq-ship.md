---
name: tq-ship
description: "Full ship workflow: check → commit → push → open PR. Use when the user says /tq-ship, 'ship it', 'commit and open PR', or wants to land changes."
---

# tq: Check → Commit → Push → Open PR

Two-phase approach: haiku subagents handle mechanical tasks, the parent agent (you) owns code fixes.

`$ARGUMENTS` may contain: commit message hint, issue number(s), or PR title.

## Step 1: Run tq Quality Gate (subagent)

Launch a subagent using the Agent tool with `model: "haiku"` and `subagent_type: "general-purpose"`.

Prompt:

~~~
Run the tq quality gate. Do NOT fix anything — just report results.

## Checks (in order, stop on first failure)

1. `golangci-lint run` — REQUIRED, do not skip. This is the #1 CI failure source (errcheck, govet, etc.)
2. `go vet ./...`
3. `go build ./cmd/tq`
4. `go test -race -short -count=1 ./...` — includes TestDocs (doc example tests that verify code blocks in docs/*.md)

## Rules

- NEVER fix, edit, or modify any file
- NEVER re-run a failing step — stop on first failure
- Show full error output for failures
- End with: `RESULT: ALL PASSED` or `RESULT: FAILURES DETECTED`
~~~

## Step 2: Fix failures (parent agent — you)

If `RESULT: ALL PASSED`, skip to Step 3.

If `RESULT: FAILURES DETECTED`:

1. Read the error output from the subagent
2. **You fix the code** — common fixes for this repo:
   - **errcheck**: add `_ =` or `_, _ =` for intentionally discarded return values
   - **TestDocs failures**: doc examples in `docs/*.md` are tested — update the code block or expected output
   - **Test failures**: read output carefully, fix with Edit tool
3. After fixing, re-run Step 1
4. Max 3 rounds — if still failing, ask the user

## Step 3: Commit → Push → Open PR (subagent)

Launch a subagent using the Agent tool with `model: "haiku"` and `subagent_type: "general-purpose"`.

Include `$ARGUMENTS` in the prompt if provided.

Prompt:

~~~
Create a git commit, push, and open a pull request for the tq project.

## Phase 1: Commit

1. Run `git status` to see all changes (never use -uall flag)
2. Run `git diff` to review staged and unstaged changes
3. Run `git log --oneline -5` to match commit style
4. Stage ONLY modified files by name — `git add path/to/file` for each
5. NEVER use `git add -A` or `git add .`
6. Draft commit message in Conventional Commits format:
   ```
   <type>(<scope>): <subject>

   [optional body]

   [footer: Closes #<issue> | Part of #<issue>]
   ```
   Types: feat, fix, refactor, build, ci, chore, docs, style, perf, test
7. Create commit using a HEREDOC:
   ```
   git commit -m "$(cat <<'EOF'
   <message>

   Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
   EOF
   )"
   ```
8. Run `git status` to verify clean working tree

## Phase 2: Push

1. Check upstream: `git rev-parse --abbrev-ref @{upstream} 2>/dev/null`
2. If no upstream: `git push -u origin HEAD`
3. If upstream exists: `git push`

## Phase 3: Open Pull Request

1. Check if PR already exists: `gh pr view --json url 2>/dev/null`
   - If exists: report its URL and skip to summary
2. Gather context:
   - `git log main..HEAD --oneline` — all commits on branch
   - Extract issue number from branch name (patterns: `fix/cli-audit-35` → `#35`, `feat/issue-21-desc` → `#21`)
3. Create PR:
   ```
   gh pr create --title "<type>(<scope>): <short description>" --body "$(cat <<'EOF'
   ## Summary
   <1-3 bullet points>

   ## Test plan
   - [ ] CI passes (lint, vet, test, build)
   - [ ] <specific verification steps>

   Closes #<issue>

   🤖 Generated with [Claude Code](https://claude.com/claude-code)
   EOF
   )"
   ```
4. Report the PR URL

## Rules

- Check `git status` before staging
- Do not amend unless explicitly asked
- After hook failure: create NEW commit, never amend
- Keep PR title under 70 chars
- Always link to GitHub issue with `Closes #N` if issue number is detectable
- Report final summary: commit hash, push status, PR URL
~~~

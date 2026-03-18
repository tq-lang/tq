---
description: "TDD dev-review loop: implement a feature/fix with automated review iterations. Use: /dev-cycle #<issue> or /dev-cycle <description>"
---

# Dev-Review Cycle

Orchestrate a development loop: a developer subagent implements, then a
reviewer subagent reviews. Loop until approved (max 3 iterations).

## Input

$ARGUMENTS — a GitHub issue number (`#42`), issue URL, or description.

## Step 0: Setup

1. If given an issue number, fetch it: `gh issue view <N>`.
2. Create a feature branch:
   ```
   # Extract type and short name from issue title
   # e.g. "feat: add --sort flag" -> feat/add-sort-flag-42
   git checkout main && git pull && git checkout -b <branch>
   ```
3. Read `.claude/CLAUDE.md` for project conventions.

## Step 1: Develop (subagent)

Launch a subagent using the Agent tool with `model: "opus"`.

Prompt:

~~~
Read and follow the developer persona in `.claude/agents/dev.md`.

## Your Task

<paste the full issue body here>

## Branch

You are on branch `<branch>`. Implement the feature/fix following TDD.
Commit after each logical step.

When done, report what you implemented and the quality gate result.
~~~

## Step 2: Review (subagent)

Launch a subagent using the Agent tool with `model: "opus"`.

Prompt:

~~~
Read and follow the reviewer persona in `.claude/agents/reviewer.md`.

## Scope

Review all changes on the current branch vs main:
`git diff main...HEAD`

Write your review following the output format in your persona doc.
End with a verdict: APPROVE or REQUEST CHANGES.
~~~

## Step 3: Evaluate

Read the reviewer's verdict.

**If APPROVE:**
- Report to the user: "Review passed. Run `/tq-ship` to commit and open a PR."
- Done.

**If REQUEST CHANGES:**
- Show the review findings to the user.
- Ask: "The reviewer found issues. Want me to fix them and re-review? (iteration N/3)"
- If yes, go back to Step 1 with the review findings added to the prompt.
- If max iterations reached, report findings and stop.

## Rules

- Never skip the review step.
- Always show review findings to the user, even on APPROVE (nits are useful).
- If the developer subagent fails the quality gate, that counts as an iteration.
- Don't create a PR — that's `/tq-ship`'s job.

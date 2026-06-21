---
description: "Spawn a reviewer subagent to review current branch changes. Use: /reviewer"
---

# Reviewer (one-shot)

Spawn a reviewer subagent to review all changes on the current branch vs main.

## Action

Launch a subagent using the Agent tool with `model: "opus"`.

Prompt:

~~~
Read and follow the reviewer persona in `.claude/agents/reviewer.md`.

## Scope

Review all changes on the current branch vs main:
`git diff main...HEAD`

Also check `git status` for any unstaged changes that should be included.

Write your review following the output format in your persona doc.
End with a verdict: APPROVE or REQUEST CHANGES.
~~~

Report the subagent's review to the user.

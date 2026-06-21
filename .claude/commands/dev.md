---
description: "Spawn a developer subagent to implement a task. Use: /dev #<issue> or /dev <description>"
---

# Developer (one-shot)

Spawn a developer subagent for a single implementation pass without the review
loop. Use `/dev-cycle` for the full loop with automated review.

## Input

$ARGUMENTS — a GitHub issue number, URL, or description.

If given an issue number, fetch it first: `gh issue view <N>`.

## Action

Launch a subagent using the Agent tool with `model: "opus"`.

Prompt:

~~~
Read and follow the developer persona in `.claude/agents/dev.md`.

## Your Task

<paste the full issue body or description here>

Implement the feature/fix following TDD. Commit after each logical step.
When done, report what you implemented and the quality gate result.
~~~

Report the subagent's results to the user.

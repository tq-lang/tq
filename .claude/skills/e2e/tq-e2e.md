---
name: tq-e2e
description: "Triage and fix test/CI failures: classify as flaky vs real, then fix. Use: /tq-e2e or /tq-e2e <PR number>"
---

# E2E Test Triage and Fix

Multi-phase pipeline for diagnosing and resolving test/CI failures.

## Input

$ARGUMENTS — optional PR number or "ci" to check the latest CI run.

## Phase 1: Triage

Read and execute the full procedure in `.claude/skills/e2e/triage.md`.

Pass the input arguments through. This phase:
- Downloads CI failure logs or reproduces locally
- Re-runs failing tests 3x to detect flakiness
- Classifies each failure (real bug, flaky, test bug, env, lint)
- Presents a triage report with confidence levels
- Asks user for approval before proceeding

**Do not proceed to Phase 2 until the user approves.**

## Phase 2: Fix

Read and execute the full procedure in `.claude/skills/e2e/fix.md`.

Pass the triage report from Phase 1. This phase:
- Applies TDD fixes for each classified failure
- Commits after each fix with appropriate conventional commit type
- Runs quality gate after all fixes
- Presents a summary table of fixes and commits

## Completion

After both phases, report:
- Total failures triaged
- Total fixes applied
- Quality gate result
- Suggest: "Run `/tq-ship` to commit and open a PR" (if on a branch)

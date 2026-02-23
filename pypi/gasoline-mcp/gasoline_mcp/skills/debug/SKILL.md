---
name: debug
description: Debug a broken website feature by collecting evidence across console errors, network requests, DOM state, and MCP command results.
---

# Gasoline Debug

Use this skill when a user reports "it is broken" and needs a root cause, not guesses.

## Inputs To Request

- Target URL
- Expected behavior
- Actual behavior
- Repro steps or user action sequence

## Workflow

1. Establish baseline:
Run observe calls for page status, tracked tab, console errors, and recent network failures.

2. Reproduce deliberately:
Use small interact actions and keep one correlation ID per high-level step.

3. Capture evidence at failure point:
Use observe for errors, network, and command_result. Use analyze for DOM or API validation only when needed.

4. Classify failure source:
Pick one primary source: frontend runtime, backend/API, auth/session, CSP/extension bridge, or timing/race.

5. Produce fix-oriented output:
Return root cause, confidence, exact evidence, minimal fix, and one verification step.

## Output Contract

- `root_cause`
- `evidence`
- `fix`
- `verify`

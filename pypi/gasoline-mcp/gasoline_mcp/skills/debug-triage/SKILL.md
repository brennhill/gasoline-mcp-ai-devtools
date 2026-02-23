---
name: debug-triage
description: Triage broken behavior with evidence-first debugging across logs, network, DOM state, and command results.
---

# Gasoline Debug Triage

Use this skill when a feature is broken and the user needs root cause with proof.

## Inputs To Request

- Target URL
- Expected behavior
- Actual behavior
- Minimal repro steps

## Workflow

1. Capture baseline:
Run `observe` for `page`, `tabs`, `errors`, and `network_waterfall`.

2. Reproduce in controlled steps:
Use `interact` actions one step at a time and keep correlation IDs.

3. Capture failure evidence:
Read `error_bundles` first, then targeted `logs`, `network_bodies`, and `command_result`.

4. Verify state assumptions:
Use `analyze(dom)` or `analyze(computed_styles)` only where evidence suggests mismatch.

5. Classify root cause:
Pick one primary class: UI runtime, API contract, auth/session, bridge/transport, timing/race, or third-party.

6. Propose smallest fix and verify:
Give one minimal change and one pass/fail validation step.

## Output Contract

- `root_cause`
- `evidence`
- `minimal_fix`
- `verification_step`

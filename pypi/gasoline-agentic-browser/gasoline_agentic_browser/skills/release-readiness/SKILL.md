---
name: release-readiness
description: Run release gates and produce a clear go/no-go assessment with blockers, mitigations, and confidence level.
---

# Gasoline Release Readiness

Use this skill before UAT and before shipping.

## Inputs To Request

- Release branch/commit
- Required gates
- UAT scenarios

## Workflow

1. Run quality gates:
Lint, unit tests, race tests, and security scans.

2. Run integration smoke:
Validate critical end-to-end flows.

3. Check operational readiness:
Confirm config paths, state paths, and startup/shutdown behavior.

4. Capture blockers:
List exact failing checks and impact.

5. Produce decision:
Return go/no-go with required actions.

## Output Contract

- `gate_results`
- `blockers`
- `mitigations`
- `decision`

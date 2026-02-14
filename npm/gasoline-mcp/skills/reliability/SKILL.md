---
name: reliability
description: Validate end-to-end reliability across LLM to MCP to server to extension, with focus on race conditions, reconnects, and state consistency.
---

# Gasoline Reliability

Use this skill for hardening and release confidence across the full control plane.

## Inputs To Request

- Critical flows to validate
- Target environment
- Time budget for soak checks

## Workflow

1. Run preflight:
Check health, tracked tab, and connection diagnostics.

2. Execute canary flows:
Cover observe, interact, analyze, and generate paths.

3. Stress sensitive transitions:
Test reconnect, tab switches, extension restarts, and stale query handling.

4. Validate invariants:
No silent drops, no unbounded queues, no stale command leakage.

5. Produce reliability report:
List failure modes, reproducibility, and mitigation.

## Output Contract

- `coverage_matrix`
- `failures`
- `mitigations`
- `release_risk`

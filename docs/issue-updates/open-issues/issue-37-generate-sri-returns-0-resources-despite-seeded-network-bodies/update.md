# Product Update: Issue #37

## Source
- Issue: https://github.com/brennhill/gasoline-mcp-ai-devtools/issues/37
- Title: generate(sri) returns 0 resources despite seeded network bodies
- Last Updated: 2026-02-13T16:28:03Z

## Change Summary
This update resolves or advances Issue #37 by defining a complete product behavior change, scoped to what users and AI agents should experience at runtime. The change is written to align with existing Gasoline workflows rather than introducing isolated behavior.

## Problem Statement
generate(sri) returns 0 resources despite seeded network bodies

## Product Intent
The product change should make this issue non-ambiguous for users by establishing deterministic behavior, clear user-visible outcomes, and compatibility with current tool semantics (observe, analyze, interact, configure, generate).

## Original Feature References
- docs/features/feature/noise-filtering/index.md (noise suppression and signal quality)
- docs/features/feature/observe/index.md (log/observe retrieval contracts)
- docs/features/file-upload/index.md (upload path and escalation behavior)
- docs/features/feature/interact-explore/index.md (interact upload action)
- docs/features/feature/observe/index.md (network/websocket observation)
- docs/features/feature/api-schema/index.md (API/schema contracts)

## User Experience Changes
1. The affected workflow should become predictable and discoverable in tool responses.
2. Failure modes should return explicit, actionable messages instead of silent or ambiguous outcomes.
3. Existing successful flows should remain backward compatible unless explicitly deprecated.

## Scope
- In scope: behavior needed to make Issue #37 functionally complete for end users.
- Out of scope: unrelated refactors, speculative platform additions not needed for this issue, and broad redesigns outside impacted flows.

## Rollout Expectations
- Ship behind existing capability flags when available.
- Preserve API shape stability unless schema changes are required for correctness.
- Document migration notes if any existing behavior changes for clients.

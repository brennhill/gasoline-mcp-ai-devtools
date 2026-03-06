# Product Update: Issue #63

## Source
- Issue: https://github.com/brennhill/gasoline-mcp-ai-devtools/issues/63
- Title: Navigate should return page content and metadata in one call
- Last Updated: 2026-02-16T13:43:28Z

## Change Summary
This update resolves or advances Issue #63 by defining a complete product behavior change, scoped to what users and AI agents should experience at runtime. The change is written to align with existing Gasoline workflows rather than introducing isolated behavior.

## Problem Statement
Navigate should return page content and metadata in one call

## Product Intent
The product change should make this issue non-ambiguous for users by establishing deterministic behavior, clear user-visible outcomes, and compatibility with current tool semantics (observe, analyze, interact, configure, generate).

## Original Feature References
- docs/features/feature/interact-explore/index.md (stateful interact actions)
- docs/features/feature/ring-buffer/index.md (state persistence and retention patterns)
- docs/features/cli-interface/index.md (installation and CLI UX contract)
- docs/features/npm-preinstall-fix/index.md (npm install/runtime behavior)
- docs/features/file-upload/index.md (upload path and escalation behavior)
- docs/features/feature/interact-explore/index.md (interact upload action)
- docs/features/feature/web-vitals/index.md (vitals and performance telemetry)
- docs/features/feature/perf-experimentation/index.md (performance analysis workflows)

## User Experience Changes
1. The affected workflow should become predictable and discoverable in tool responses.
2. Failure modes should return explicit, actionable messages instead of silent or ambiguous outcomes.
3. Existing successful flows should remain backward compatible unless explicitly deprecated.

## Scope
- In scope: behavior needed to make Issue #63 functionally complete for end users.
- Out of scope: unrelated refactors, speculative platform additions not needed for this issue, and broad redesigns outside impacted flows.

## Rollout Expectations
- Ship behind existing capability flags when available.
- Preserve API shape stability unless schema changes are required for correctness.
- Document migration notes if any existing behavior changes for clients.

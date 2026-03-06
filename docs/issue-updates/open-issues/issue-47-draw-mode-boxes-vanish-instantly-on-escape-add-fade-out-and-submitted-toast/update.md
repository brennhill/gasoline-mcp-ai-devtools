# Product Update: Issue #47

## Source
- Issue: https://github.com/brennhill/gasoline-mcp-ai-devtools/issues/47
- Title: Draw mode: boxes vanish instantly on Escape — add fade-out and 'submitted' toast
- Last Updated: 2026-02-14T18:56:16Z

## Change Summary
This update resolves or advances Issue #47 by defining a complete product behavior change, scoped to what users and AI agents should experience at runtime. The change is written to align with existing Gasoline workflows rather than introducing isolated behavior.

## Problem Statement
Draw mode: boxes vanish instantly on Escape — add fade-out and 'submitted' toast

## Product Intent
The product change should make this issue non-ambiguous for users by establishing deterministic behavior, clear user-visible outcomes, and compatibility with current tool semantics (observe, analyze, interact, configure, generate).

## Original Feature References
- docs/features/feature/interact-explore/index.md (stateful interact actions)
- docs/features/feature/ring-buffer/index.md (state persistence and retention patterns)
- docs/features/cli-interface/index.md (installation and CLI UX contract)
- docs/features/npm-preinstall-fix/index.md (npm install/runtime behavior)
- docs/features/feature/interact-explore/index.md (interact screenshot workflow)
- docs/features/feature/annotated-screenshots/index.md (image capture/reporting patterns)
- docs/features/file-upload/index.md (upload path and escalation behavior)
- docs/features/feature/interact-explore/index.md (interact upload action)

## User Experience Changes
1. The affected workflow should become predictable and discoverable in tool responses.
2. Failure modes should return explicit, actionable messages instead of silent or ambiguous outcomes.
3. Existing successful flows should remain backward compatible unless explicitly deprecated.

## Scope
- In scope: behavior needed to make Issue #47 functionally complete for end users.
- Out of scope: unrelated refactors, speculative platform additions not needed for this issue, and broad redesigns outside impacted flows.

## Rollout Expectations
- Ship behind existing capability flags when available.
- Preserve API shape stability unless schema changes are required for correctness.
- Document migration notes if any existing behavior changes for clients.

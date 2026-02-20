# LLM UX Feedback (2026-02-20)

## Context
- Evaluator: Codex
- Repository: `brennhill/gasoline-mcp-ai-devtools`
- Version observed during testing: `v0.7.5`
- Focus: end-to-end LLM operator UX while using `interact`, `analyze`, `observe`, `generate`, and `configure`

## What Was Easy
- Tool surface area is broad and practical for real debugging workflows.
- Async command model with `correlation_id` + `observe(what="command_result")` is clear once running.
- `configure` diagnostics (`doctor`, `health`, `restart`, noise controls) are useful for recovery without leaving the loop.
- Annotation workflows (`draw_mode_start` + analyze/generate) are powerful when state is wired correctly.

## What Was Hard
- Timeout behavior is split across layers (tool-level waits, bridge-level request timeout), which can mask root cause.
- Some workflows duplicated heavy operations (`run_a11y_and_export_sarif` calling a11y twice), creating hidden latency.
- `draw_session` file loading did not automatically hydrate in-memory annotation state for `generate.annotation_*`.
- Some DOM read actions returned `value: null` without reason fields, making LLM reasoning ambiguous.
- Local test runs required specific env setup (`GOCACHE`, `GASOLINE_STATE_DIR`) to avoid sandbox permission failures.

## What I Liked
- Strong observability primitives (errors/logs/network/timeline/failed commands).
- Deterministic command tracking model and explicit lifecycle status fields.
- Good coverage-oriented tests around many tool contracts.

## What Would Make It Easier
- A single timeout model exposed consistently in responses (effective timeout + where it came from).
- Explicit “degraded execution” surfaced in health/doctor by default.
- Standardized reason fields for “successful-but-empty” responses (`value: null` cases).
- Automatic state hydration bridges between persisted artifacts and generators (`draw_session` -> `generate.*`).
- First-class docs snippet for sandbox-safe test env (`GOCACHE`, `GASOLINE_STATE_DIR`) and known network-port test constraints.

## Issues Filed By Codex
- `#159` Commands expire while doctor reports healthy
- `#160` DOM actions can return `complete` + `null` without cause
- `#161` Sync dedupe can skip new commands with reused IDs
- `#162` draw_session output not consumable by annotation generators
- `#163` replay_sequence timeout leaves pending commands
- `#164` run_a11y_and_export_sarif timeout path
- `#165` record_start/record_stop lifecycle ambiguity
- `#166` health/doctor missing command execution readiness

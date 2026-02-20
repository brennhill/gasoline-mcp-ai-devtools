# LLM UX Feedback

Date: 2026-02-20  
Evaluator: Codex (GPT-5)  
Version tested: Gasoline v0.7.5

## Summary

Gasoline has strong observability and diagnostics when command execution is healthy. The biggest UX issue for LLM agents is command lifecycle reliability and clarity during degraded extension state.

## What Was Easy

- `configure.health`, `configure.doctor`, `configure.describe_capabilities`
- `observe.tabs`, `observe.page`, `observe.pilot`, `observe.timeline`
- `observe.network_waterfall`, `observe.network_bodies`, `observe.summarized_logs`
- `analyze.page_summary`, `analyze.forms`, `analyze.form_validation`, `analyze.computed_styles`
- `analyze.link_validation`, `analyze.api_validation`
- `generate.test_classify`, `generate.pr_summary`

## What Was Hard

- `interact` reliability degraded mid-session:
  - multiple commands entered `still_processing` then expired
  - affected `execute_js`, `get_markdown`, `record_start/record_stop`, `run_a11y_and_export_sarif`
- Some actions returned `complete` with `value: null` for DOM interactions, making results hard to trust.
- Health status could remain `healthy` while commands timed out/expired.
- Extension logs repeatedly showed:
  - `"[Sync] Skipping already processed command"` with command IDs (example: `q-5`, `q-9`)
- `draw_history` output is very large for agent consumption.
- Parameter model can be inconsistent across tools (`what`, `action`, `operation`, `store_action`).

## What I Like

- Error objects are high quality:
  - clear retry guidance
  - useful hints
  - good structured fields for automation
- Correlation IDs and command inspection (`pending_commands`, `failed_commands`, `command_result`) are useful.
- Auditability (`configure.audit_log`) is strong for debugging agent behavior.

## How It Could Be Easier (Priority Order)

1. Add command cancellation primitives:
   - `cancel_command(correlation_id)`
   - `cancel_all_pending()`
2. Standardize lifecycle semantics and make them explicit:
   - `accepted`, `running`, `completed`, `failed`, `expired`, `skipped_duplicate`
3. Unify parameter naming across tools:
   - reduce `what` vs `action` vs `operation` ambiguity
4. For `value: null`, return structured cause metadata:
   - `not_found`, `not_visible`, `selector_invalid`, `execution_blocked`, etc.
5. Add pagination and default limits for large outputs (especially draw/session history).
6. Split connectivity health from execution health:
   - extension connected
   - queue/executor healthy
   - last successful command timestamp
   - duplicate-skip counters
7. Improve restart/reconnect behavior to prevent stale command handling after daemon restart.

## Suggested Acceptance Criteria For UX Improvements

- No command remains pending indefinitely without terminal state.
- `doctor` and `health` reflect actionable execution readiness, not only transport readiness.
- Duplicate-command handling is surfaced clearly with a machine-readable status.
- DOM actions do not return bare null without cause.

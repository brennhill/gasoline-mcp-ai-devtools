---
doc_type: feature_index
feature_id: feature-auto-fix
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-28
code_paths:
  - cmd/browser-agent/tools_analyze_page_issues.go
  - cmd/browser-agent/tools_analyze_page_issues_summary.go
  - cmd/browser-agent/tools_analyze_dispatch.go
  - cmd/browser-agent/intent_store.go
  - cmd/browser-agent/intent_handlers.go
  - cmd/browser-agent/handler_tools_call_postprocess.go
  - cmd/browser-agent/terminal_relay.go
  - internal/schema/analyze.go
  - internal/tools/configure/mode_specs_analyze.go
  - src/content/ui/tracked-hover-launcher.ts
  - src/background/message-handlers.ts
  - src/types/runtime-messages.ts
  - npm/gasoline-agentic-browser/skills/qa/SKILL.md
test_paths:
  - cmd/browser-agent/tools_analyze_page_issues_test.go
  - cmd/browser-agent/intent_store_test.go
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Auto-Fix

## TL;DR
- Status: shipped
- Three delivery channels for guided QA debugging
- Spec: `specs/auto-fix.md`
- Plan: `specs/auto-fix-plan.md`
- Flow Map: `flow-map.md`

## Components

### 1. `analyze(what: "page_issues")`
One-call evidence sweep that aggregates console errors, network failures (4xx/5xx), a11y violations, and security findings into a unified prioritized report. Runs checks in parallel with per-check timeouts. Supports `summary:true` for ~80% token reduction.

### 2. `qa` meta-skill
Methodology playbook installed to `~/.claude/skills/`, `~/.codex/skills/`, `~/.gemini/skills/`. Teaches AI agents a 9-step systematic QA walkthrough: health check, baseline scan, navigation discovery, per-page walkthrough, auth wall handling, evidence gathering, ambiguity resolution, report synthesis, and fix offer.

### 3. "Find Problems" hover button
Button on the tracked-page hover widget. Two-path delivery:
1. **PTY injection** (primary): writes a QA prompt directly into the active terminal session
2. **Intent fallback**: stores the intent on the daemon, surfaced as an ACTION REQUIRED nudge in the next tool response (up to 3 nudges before expiry)

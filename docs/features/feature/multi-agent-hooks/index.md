---
doc_type: feature_index
feature_id: feature-multi-agent-hooks
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-07
code_paths:
  - internal/hook/protocol.go
  - cmd/hooks/main.go
test_paths:
  - internal/hook/protocol_test.go
  - cmd/hooks/main_test.go
---

# Multi-Agent Hook Protocol

| Field         | Value                                   |
|---------------|-----------------------------------------|
| **Status**    | proposed                                |
| **Binary**    | gasoline-hooks                          |
| **Agents**    | Claude Code, Gemini CLI, Codex (future) |
| **Parent**    | [Quality Gates](../quality-gates/index.md) |

## Specs

- [Product Spec](./product-spec.md)
- [Tech Spec](./tech-spec.md)

## Summary

The `gasoline-hooks` binary auto-detects which AI coding agent is calling it and adapts its output protocol accordingly. All hooks (quality-gate, compress-output, session-track, blast-radius, decision-guard) work across agents without separate binaries or configuration. The hook logic is agent-agnostic; only the thin I/O protocol layer adapts.

## Supported Agents

| Agent | Hook Event | Config File | Output Format | Session ID |
|-------|-----------|-------------|---------------|------------|
| Claude Code | PostToolUse | `.claude/settings.json` | `{"additionalContext": "..."}` | Derived from `(ppid, cwd)` |
| Gemini CLI | AfterTool | `.gemini/settings.json` | `{"hookSpecificOutput": {"additionalContext": "..."}}` | `GEMINI_SESSION_ID` env var |
| Codex | post_exec (future) | `codex.toml` | TBD | TBD |

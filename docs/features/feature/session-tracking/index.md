---
doc_type: feature_index
feature_id: feature-session-tracking
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-07
code_paths:
  - internal/hook/session_track.go
  - cmd/hooks/main.go
test_paths:
  - internal/hook/session_track_test.go
  - cmd/hooks/main_test.go
---

# Session Tracking

| Field         | Value                                   |
|---------------|-----------------------------------------|
| **Status**    | proposed                                |
| **Binary**    | gasoline-hooks                          |
| **Command**   | `gasoline-hooks session-track`          |
| **Hook**      | PostToolUse on Read, Edit, Write, Bash  |
| **Parent**    | [Quality Gates](../quality-gates/index.md) |

## Specs

- [Product Spec](./product-spec.md)
- [Tech Spec](./tech-spec.md)

## Summary

Session tracking records every file read, edit, and command execution during an AI coding session. On subsequent tool uses, it injects session context — which files were already read, what was changed, what tests passed or failed — so the AI avoids redundant work and maintains awareness of its own actions.

This is the foundation hook that other hooks (`blast-radius`, `decision-guard`) read from to make smarter decisions.

## Install

```bash
# Already included in gasoline-hooks
gasoline-hooks session-track
```

## Hook Configuration

```json
{
  "matcher": "Read|Edit|Write|Bash",
  "hooks": [{"type": "command", "command": "gasoline-hooks session-track", "timeout": 10}]
}
```

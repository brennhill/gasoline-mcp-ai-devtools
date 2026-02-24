---
feature: feature-name-here
status: proposed          # proposed | in-progress | shipped | deprecated
version: null             # Version where this shipped (e.g., 5.1.0)
tool: observe             # observe | generate | configure | interact
mode: mode_name           # The specific mode/action string
authors: []
created: YYYY-MM-DD
updated: YYYY-MM-DD
last_reviewed: 2026-02-16
---

# Feature Name

> One-line summary of what this feature does.

## Problem

What user problem does this solve? Why does this matter for AI coding agents?

## Solution

How does Gasoline solve this? High-level description.

## User Stories

- As an AI coding agent, I want to [action] so that [outcome].
- As a developer using Gasoline, I want to [action] so that [outcome].

## MCP Interface

**Tool:** `<tool_name>`
**Mode/Action:** `<mode_name>`

### Request
```json
{
  "tool": "<tool_name>",
  "arguments": {
    "<key>": "<value>"
  }
}
```

### Response
```json
{
  "<key>": "<value>"
}
```

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Description | must |
| R2 | Description | should |
| R3 | Description | could |

## Non-Goals

What this feature explicitly does NOT do. Drawing clear boundaries prevents scope creep.

- This feature does NOT [thing that might be assumed].
- Out of scope: [related capability that belongs to a different feature].

## Performance SLOs

| Metric | Target |
|--------|--------|
| Response time | < 200ms |
| Memory impact | < 1MB |

## Security Considerations

- What data is captured?
- What is redacted?
- Privacy implications?
- Does this change the attack surface?

## Edge Cases

Consider: no data, malformed input, extension disconnected, tab closed, buffer full, concurrent requests.

- What happens when [edge case]? Expected behavior: [behavior].
- What happens when [edge case]? Expected behavior: [behavior].

## Dependencies

- Depends on: [other features or systems]
- Depended on by: [other features]

## Assumptions

> Added during spec refinement (Phase 3). These become test preconditions.

- A1: [assumption, e.g., "extension is connected and tracking a tab"]
- A2: [assumption, e.g., "buffer contains at least one entry"]

## Open Items

> Added during spec refinement (Phase 3). Unresolved questions or design decisions.

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | [question or decision] | open | [why it matters] |
| OI-2 | [question or decision] | resolved | [resolution] |

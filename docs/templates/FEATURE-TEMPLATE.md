---
feature: feature-name-here
status: proposed          # proposed | in-progress | shipped | deprecated
version: null             # Version where this shipped (e.g., 5.1.0)
tool: observe             # observe | generate | configure | interact
mode: mode_name           # The specific mode/action string
authors: []
created: YYYY-MM-DD
updated: YYYY-MM-DD
---

# Feature Name

> One-line summary of what this feature does.

## Problem

What user problem does this solve? Why does this matter for AI coding agents?

## Solution

How does Gasoline solve this? High-level description.

## User Stories

- As an AI coding agent, I want to [action] so that [outcome].
- As a developer, I want to [action] so that [outcome].

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

## Performance SLOs

| Metric | Target |
|--------|--------|
| Response time | < 200ms |
| Memory impact | < 1MB |

## Security Considerations

- What data is captured?
- What is redacted?
- Privacy implications?

## Edge Cases

- What happens when [edge case]?
- What happens when [edge case]?

## Dependencies

- Depends on: [other features or systems]
- Depended on by: [other features]

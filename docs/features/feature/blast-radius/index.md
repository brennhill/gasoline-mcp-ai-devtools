---
doc_type: feature_index
feature_id: feature-blast-radius
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-07
code_paths:
  - internal/hook/blast_radius.go
  - internal/hook/import_graph.go
  - cmd/hooks/main.go
test_paths:
  - internal/hook/blast_radius_test.go
  - internal/hook/import_graph_test.go
  - cmd/hooks/main_test.go
---

# Blast Radius

| Field         | Value                                   |
|---------------|-----------------------------------------|
| **Status**    | proposed                                |
| **Binary**    | gasoline-hooks                          |
| **Command**   | `gasoline-hooks blast-radius`           |
| **Hook**      | PostToolUse on Edit, Write              |
| **Parent**    | [Quality Gates](../quality-gates/index.md) |

## Specs

- [Product Spec](./product-spec.md)
- [Tech Spec](./tech-spec.md)

## Summary

When the AI edits a file, blast-radius scans the project for files that import or depend on the edited file and injects a warning listing affected dependents. This prevents the AI from making breaking changes without checking downstream consumers. If session-tracking is installed, blast-radius highlights dependents the AI has already visited this session.

## Hook Configuration

```json
{
  "matcher": "Edit|Write",
  "hooks": [{"type": "command", "command": "gasoline-hooks blast-radius", "timeout": 10}]
}
```

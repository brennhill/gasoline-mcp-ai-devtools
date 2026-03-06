---
doc_type: feature_index
feature_id: feature-quality-gates
status: in-progress
feature_type: feature
owners: []
last_reviewed: 2026-03-06
code_paths:
  - cmd/dev-console/tools_configure_quality_gates.go
  - cmd/dev-console/tools_configure_registry.go
  - internal/tools/configure/mode_specs_configure.go
  - internal/schema/configure_properties_core.go
  - internal/schema/configure_properties_runtime.go
test_paths:
  - cmd/dev-console/tools_configure_quality_gates_test.go
last_verified_version: 0.8.1
last_verified_date: 2026-03-06
---

# Quality Gates

| Field         | Value                                   |
|---------------|-----------------------------------------|
| **Status**    | in-progress                             |
| **Tool**      | configure                               |
| **Mode**      | `what="setup_quality_gates"`            |
| **Schema**    | `internal/schema/configure_properties_runtime.go` |
| **Issue**     | [#506](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/issues/506) |

## Specs

- [Flow Map](./flow-map.md)

## Summary

Automated code quality enforcement that catches architectural drift, duplicate code, and pattern violations without burning tokens. Scaffolds `.gasoline.json` and `gasoline-code-standards.md` in the project root. Quality gates are enforced via Claude Code prompt hooks that call Haiku to review edits against the standards doc.

## Architecture

1. **`.gasoline.json`** — minimal config pointing to the standards doc, committed to repo
2. **`gasoline-code-standards.md`** — plain markdown coding conventions, read by Haiku
3. **Claude Code prompt hooks** — PostToolUse on Edit/Write sends diff + standards to Haiku
4. **Haiku review** — ~$0.0001/edit, findings injected as `additionalContext`

## Setup

```
configure(what="setup_quality_gates")
```

Creates both files with sensible defaults. Does not overwrite existing files.

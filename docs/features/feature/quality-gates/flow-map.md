---
doc_type: flow_map
feature_id: feature-quality-gates
last_reviewed: 2026-03-06
---

# Quality Gates Flow Map

## Setup Flow

```
AI or user calls configure(what="setup_quality_gates")
  -> toolConfigureSetupQualityGates()
  -> resolve project dir from server.GetActiveCodebase()
  -> validate target_dir is within project (security)
  -> if .gasoline.json missing: write default config
  -> read code_standards path from config (existing or default)
  -> if default standards file missing: write starter content
  -> return config_path, standards_path, defaults, suggestions
```

## Quality Gate Enforcement Flow (Hook-Driven)

```
AI calls Edit/Write tool
  -> Claude Code PostToolUse hook fires
  -> Prompt hook sends to Haiku:
     - gasoline-code-standards.md (from .gasoline.json pointer)
     - the diff (changed file content)
  -> Haiku checks for violations (~200ms)
  -> Findings returned as additionalContext
  -> Primary model (Opus/Sonnet) sees findings
  -> Fixes immediately in same turn
```

## Code Paths

| Component | Path |
|-----------|------|
| Handler | `cmd/dev-console/tools_configure_quality_gates.go` |
| Registry | `cmd/dev-console/tools_configure_registry.go` |
| Mode spec | `internal/tools/configure/mode_specs_configure.go` |
| Schema enum | `internal/schema/configure_properties_core.go` |
| Schema props | `internal/schema/configure_properties_runtime.go` |
| Tests | `cmd/dev-console/tools_configure_quality_gates_test.go` |

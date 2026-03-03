---
doc_type: tech-spec
feature_id: feature-mcp-persistent-server
status: shipped
last_reviewed: 2026-03-03
---

# MCP Persistent Server Tech Spec

## Architecture
- Startup/bridge orchestration: `cmd/dev-console/bridge*.go`
- Cross-platform process helpers: `internal/util/proc_unix.go`, `internal/util/proc_windows.go`
- Startup convergence/leadership: `bridge_startup_orchestration.go`, lock/state helpers

## Constraints
- Single startup leader per port; followers wait and attach.
- Bounded retries and stale-lock takeover prevent dead startup state.
- Health endpoints and configure restart must be safe under contention.

---
doc_type: qa-plan
feature_id: feature-mcp-persistent-server
status: shipped
last_reviewed: 2026-03-03
---

# MCP Persistent Server QA Plan

## Automated Coverage
- `cmd/dev-console/bridge_startup_contention_test.go`
- `cmd/dev-console/bridge_faststart_extended_test.go`
- `cmd/dev-console/bridge_spawn_race_test.go`

## Required Scenarios
1. Multiple concurrent clients converge to one daemon.
2. Followers connect to leader-started daemon within bounded wait.
3. Stale startup lock is detected and safely reclaimed.
4. Restart path recovers from non-responsive daemon process.

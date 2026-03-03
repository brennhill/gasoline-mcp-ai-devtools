---
doc_type: tech-spec
feature_id: feature-cold-start-queuing
status: implemented
last_reviewed: 2026-03-03
---

# Cold-Start Queuing Tech Spec

## Architecture
- Gate entry: `requireExtension()` in `cmd/dev-console/tools_core.go`
- Wait primitive: `capture.WaitForExtensionConnected(ctx, timeout)`
- Poll cadence: 100ms ticker with context cancellation

## Ordering
1. Pilot/transport prerequisites
2. Extension readiness wait (cold-start gate)
3. Remaining command-specific gates and dispatch

## Constraints
- No goroutine leak on cancellation/shutdown.
- No duplicate wait in downstream async result polling.
- Error surface must remain structured and retryable.

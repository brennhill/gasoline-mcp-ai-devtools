---
status: shipped
scope: feature/deployment-watchdog/review
ai-priority: high
tags: [review, issues]
relates-to: [tech-spec.md, product-spec.md]
last-verified: 2026-01-31
---

# Deployment Watchdog Review

_Migrated from /specs/deployment-watchdog-review.md_

# Deployment Watchdog (Feature 36) Technical Review

**Reviewer:** Principal Engineer
**Date:** 2026-01-26
**Spec:** docs/ai-first/tech-spec-agentic-cicd.md
**Status:** REVIEW REQUIRED BEFORE IMPLEMENTATION

---

## Executive Summary

The Deployment Watchdog spec outlines a valuable post-deployment monitoring capability that leverages existing Gasoline primitives (`diff_sessions`, `observe`, `analyze`, `security_audit`) to detect regressions and trigger rollbacks. The architecture is fundamentally sound as a Claude Code skill rather than a Gasoline tool. However, the spec lacks critical implementation details around **state persistence across deploys**, **rollback authorization boundaries**, and **concurrent monitoring session isolation**. The existing `diff_sessions` and `verify_fix` implementations provide solid foundations but require extensions for production deployment scenarios.

---

## 1. Critical Issues (Must Fix Before Implementation)

### 1.1 Missing Baseline Persistence Across Server Restarts

**Section:** Workflow step 1 ("Pre-deploy: Agent captures session snapshot 'pre-v2.3.0'")

**Problem:** The current `SessionManager` (sessions.go:144-151) stores snapshots in memory with no persistence:

```go
type SessionManager struct {
    mu      sync.RWMutex
    snaps   map[string]*NamedSnapshot  // In-memory only
    order   []string
    maxSize int
    reader  CaptureStateReader
}
```

In a deployment scenario, the Gasoline server itself may restart as part of the deploy. All pre-deployment baselines are lost.

**Impact:** Post-deploy comparison impossible if server restarts during deploy.

**Required Fix:** Add snapshot persistence to the `configure {action: "store"}` mechanism. The spec should mandate:
1. Pre-deploy: `diff_sessions {action: "capture", name: "pre-v2.3.0"}` followed by `configure {action: "store", key: "deploy-baseline-v2.3.0"}`
2. Post-deploy: `configure {action: "load", key: "deploy-baseline-v2.3.0"}` to restore baseline before comparison

### 1.2 Rollback Authorization Undefined

**Section:** Workflow step 8 ("Agent executes: Calls deployment API to rollback")

**Problem:** The spec states AI "decides" severity and "executes" rollback but provides no authorization framework. Questions unanswered:

- What credentials does the agent use to call the deployment API?
- Is there human approval gating for production rollbacks?
- What scope limitations prevent cascading rollbacks?

**Impact:** Unauthorized production rollbacks constitute a critical security risk.

**Required Fix:** Define explicit authorization tiers in the spec:

| Rollback Target | Authorization Level |
|-----------------|---------------------|
| Preview/Staging | Agent autonomous |
| Production (non-critical) | Agent proposes, human approves in Slack within 5min timeout |
| Production (critical path) | Human approval required, no timeout |

### 1.3 No Circuit Breaker for Consecutive Rollback Attempts

**Section:** Risks section mentions "Infinite loops" but only for fix attempts

**Problem:** A flapping deployment (regression detected -> rollback -> re-deploy -> regression detected again) creates a rollback loop. The spec's circuit breaker applies to fix attempts (max 3), but rollback has no equivalent.

**Impact:** Unbounded rollback-deploy cycles can destabilize production.

**Required Fix:** Add to spec:
```yaml
rollback_circuit_breaker:
  max_rollbacks_per_hour: 2
  cooldown_after_rollback: 30min
  on_circuit_open: "notify on-call, pause automated rollbacks"
```

---

## 2. Performance Concerns

### 2.1 Snapshot Memory Footprint

**Location:** sessions.go:17-25

```go
const (
    maxSnapshotNameLen     = 50
    maxConsolePerSnapshot  = 50
    maxNetworkPerSnapshot  = 100
    // ...
)
```

**Analysis:** Each `NamedSnapshot` captures up to 50 console errors + 50 warnings + 100 network requests + WebSocket connections + performance data. In a deployment watchdog scenario where multiple deploys occur per day, the default `maxSize=10` snapshots may be insufficient.

**Concern:** With production traffic generating hundreds of network requests, the 100-request cap may miss regression signals on lower-traffic endpoints.

**Recommendation:** Add deploy-mode configuration:
```yaml
deploy_watchdog:
  snapshot_retention: 50  # Keep more snapshots during deploy windows
  network_sample_rate: 0.1  # Sample 10% of requests for high-traffic deploys
```

### 2.2 Compare Operation Lock Contention

**Location:** sessions.go:259-279

The `Compare` method acquires `RLock`, releases, then potentially acquires again for snapshot B lookup. Under high concurrency (multiple parallel deployments being monitored), this creates potential starvation:

```go
func (sm *SessionManager) Compare(a, b string) (*SessionDiffResult, error) {
    sm.mu.RLock()
    snapA, existsA := sm.snaps[a]
    sm.mu.RUnlock()  // Lock released here

    // ... if b == "current", captures new snapshot (no lock)

    sm.mu.RLock()    // Re-acquired here
    found, exists := sm.snaps[b]
    sm.mu.RUnlock()
```

**Impact:** In the window between releases, another goroutine could evict snapA or snapB.

**Recommendation:** Use a single lock scope for the entire compare operation, or implement copy-on-read for snapshots.

### 2.3 GC Pressure from Snapshot Deep Copies

**Location:** sessions.go:234-239

```go
var perfCopy *PerformanceSnapshot
if perf != nil {
    p := *perf
    perfCopy = &p
}
```

**Analysis:** This shallow copies the struct but doesn't deep copy slices within `PerformanceSnapshot.Resources` or maps in `Network.ByType`. Under rapid snapshot capture (deploy watchdog polling every 5s), this creates GC pressure.

**Recommendation:** Pool `NamedSnapshot` objects or implement proper deep copy with pre-allocated slice capacities.

---

## 3. Concurrency Issues

### 3.1 Race Between Capture and Compare

**Location:** verify.go:300-336

The `Compare` method captures "after" state, but if the extension is actively pushing events during comparison, the baseline and after snapshots may have inconsistent temporal boundaries.

**Scenario:**
1. t=0: Baseline captured
2. t=5: Compare starts, begins capturing "after" state
3. t=5.001: Extension pushes new error
4. t=5.002: "after" capture completes with error
5. Result: False positive regression (error arrived during capture, not from deploy)

**Recommendation:** Add capture timestamps and filter events to only those that occurred after the deploy completion timestamp:

```go
type DeployWatchdogParams struct {
    BaselineSnapshot string
    DeployCompletedAt time.Time  // Filter events before this time
}
```

### 3.2 Verification Session Limit May Block Parallel Deploys

**Location:** verify.go:21

```go
const maxVerificationSessions = 3
```

**Problem:** With only 3 concurrent sessions, monitoring multiple parallel deployments (staging + production + preview) saturates the limit. Additional watchdog sessions will fail.

**Recommendation:** Increase to 10 for deploy watchdog use case, or make configurable via environment variable.

---

## 4. Data Contract Concerns

### 4.1 No Versioning for Snapshot Schema

**Location:** sessions.go:65-76

The `NamedSnapshot` struct has no version field. If future releases add fields (e.g., `TracingSpans []Span`), comparing old snapshots against new ones may produce incorrect diffs.

**Recommendation:** Add schema version:

```go
type NamedSnapshot struct {
    SchemaVersion int       `json:"schema_version"`  // Add this
    Name          string    `json:"name"`
    // ...
}
```

### 4.2 Verdict Enum Not Documented

**Location:** sessions.go:517-526

```go
switch {
case hasRegressions && hasImprovements:
    summary.Verdict = "mixed"
case hasRegressions:
    summary.Verdict = "regressed"
// ...
}
```

**Problem:** The spec workflow step 7 states "Agent decides: Severity HIGH -> trigger rollback" but doesn't define how verdicts map to severities.

**Required Mapping:**
| Verdict | Severity | Rollback Action |
|---------|----------|-----------------|
| `regressed` | HIGH | Auto-rollback (with approval for prod) |
| `mixed` | MEDIUM | Notify, manual decision |
| `unchanged` | LOW | No action |
| `improved` | NONE | Log success |

---

## 5. Security Concerns

### 5.1 Deployment API Credentials in Agent Context

**Section:** Workflow step 8

**Problem:** The agent must have credentials to call the deployment API. If these credentials are stored in the skill definition or passed via environment, they're accessible to the AI model.

**Recommendation:** Use a credential-free proxy pattern:
1. Skill calls local endpoint: `POST /gasoline/rollback-request {deployment_id, reason}`
2. Gasoline server validates request and forwards to a privileged sidecar
3. Sidecar holds credentials and calls actual deployment API

### 5.2 Snapshot Data May Contain Secrets

**Concern:** `SnapshotNetworkRequest.URL` may contain API keys in query strings. `ResponseBody` (if captured) may contain secrets.

**Existing Mitigation:** `security.go` has credential detection, but it's not automatically applied to snapshots.

**Recommendation:** Run `checkCredentials` on snapshot data before storage:

```go
func (sm *SessionManager) Capture(name, urlFilter string) (*NamedSnapshot, error) {
    snapshot := sm.captureCurrentState(name, urlFilter)
    sm.redactSensitiveData(snapshot)  // Add this
    // ...
}
```

### 5.3 Audit Trail Doesn't Capture Rollback Decisions

**Location:** audit_trail.go records tool invocations but not the decision logic for rollbacks.

**Recommendation:** Add `RollbackDecision` audit entry type:

```go
type RollbackDecisionEntry struct {
    Timestamp     time.Time
    DeploymentID  string
    Verdict       string
    Severity      string
    Decision      string  // "rollback", "notify", "ignore"
    Reasoning     string  // AI's explanation
    ApprovalState string  // "autonomous", "pending", "approved", "rejected"
}
```

---

## 6. Maintainability Concerns

### 6.1 Skill Definition Language Undefined

**Section:** Example skill definition (lines 249-270)

The YAML skill schema is shown but not specified. Questions:

- What's the schema for `preconditions.check`?
- How are `triggers.webhook` events validated?
- What's the execution model for `workflow` steps?

**Recommendation:** Either:
1. Define the skill schema formally in a separate spec
2. Use an existing standard (GitHub Actions syntax, Argo Workflows, etc.)

### 6.2 No Observability for Watchdog Itself

The watchdog monitors deployments but has no self-monitoring. If the watchdog fails to detect a regression due to a bug, there's no signal.

**Recommendation:** Add watchdog health metrics:
- `watchdog_comparisons_total` - counter
- `watchdog_regressions_detected` - counter
- `watchdog_rollbacks_triggered` - counter
- `watchdog_last_comparison_time` - gauge

### 6.3 Testing Surface Unclear

**Question:** How do you test a deployment watchdog without deploying?

**Recommendation:** Add test mode in skill:
```yaml
test_mode:
  mock_deploy_api: true
  inject_regression: "console_error"  # Simulate regression for testing
```

---

## 7. Implementation Roadmap

### Phase 1: Foundation (Week 1-2)
1. Add snapshot persistence to `configure {action: "store/load"}`
2. Increase `maxVerificationSessions` to 10
3. Add `schema_version` to `NamedSnapshot`
4. Document verdict-to-severity mapping

### Phase 2: Security Hardening (Week 3)
1. Implement credential-free rollback proxy pattern
2. Add `redactSensitiveData()` to snapshot capture
3. Add `RollbackDecisionEntry` to audit trail
4. Define authorization tiers for rollback actions

### Phase 3: Skill Framework (Week 4-5)
1. Define skill YAML schema formally
2. Implement circuit breaker for rollbacks
3. Add watchdog health metrics
4. Build test mode with regression injection

### Phase 4: Production Hardening (Week 6)
1. Add deploy-mode configuration for snapshot retention
2. Implement capture timestamp filtering for race prevention
3. Load testing with parallel deployment scenarios
4. Document operational runbooks

---

## 8. Questions for Spec Authors

1. **State persistence:** Is snapshot persistence required for MVP, or acceptable to lose baselines on server restart?

2. **Rollback scope:** Can the watchdog only monitor single-service deploys, or must it handle multi-service orchestrated deploys?

3. **Notification channel:** The spec mentions "Posts to Slack" -- is this the only supported channel? What about PagerDuty, OpsGenie, email?

4. **Recovery from false positives:** If the watchdog triggers a rollback incorrectly, what's the recovery path? Can it "learn" from overridden decisions?

5. **Multi-tenancy:** If multiple AI agents are monitoring different deployments simultaneously, how is session isolation guaranteed?

---

## Verdict

**Conditional Approval** -- The spec demonstrates strong product thinking and correctly positions Deployment Watchdog as a skill rather than a Gasoline tool. The dependency on existing primitives (`diff_sessions`, `security_audit`, `analyze`) is appropriate.

However, implementation should not proceed until:
1. Snapshot persistence mechanism is specified
2. Rollback authorization tiers are defined
3. Circuit breaker for rollbacks is added

The existing codebase provides 70% of the required infrastructure. The remaining 30% (persistence, authorization, observability) requires new design work before implementation begins.

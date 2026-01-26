# v6 Roadmap

## Thesis

**AI will be the driving force in development.**

Gasoline's strategic differentiator is enabling AI to **close the feedback loop autonomously** — observe, diagnose, and repair without human intervention. Every feature is evaluated against this thesis.

---

## Completed

| Feature | Description | Merged |
|---------|-------------|--------|
| Tab Targeting (Phase 0) | `tab_id` parameter on all pilot tools, `observe {what: "tabs"}`, `browser_action {action: "open"}` | 2025-01-25 |
| API Contract Validation | `validate_api` tool - track response shapes, detect contract violations | 2025-01-25 |
| Verification Loop | `verify_fix` tool - before/after session comparison for fix verification | 2025-01-25 |
| SRI Hash Generator | `generate_sri` tool - Subresource Integrity hashes for third-party resources | 2025-01-25 |
| Health Metrics | `get_health` tool - server uptime, buffer utilization, memory usage | 2025-01-25 |
| Security Scanner | `security_audit` - credentials, PII, insecure transport, headers, cookies | Pre-v6 |
| CSP Generator | `generate_csp` - Content-Security-Policy from observed origins | Pre-v6 |
| Third-Party Audit | `audit_third_parties` - external domain mapping, risk classification | Pre-v6 |
| Security Diff | `diff_security` - security posture comparison before/after changes | Pre-v6 |
| Session Comparison | `diff_sessions` - named snapshot storage and comparison | Pre-v6 |
| Audit Log | `get_audit_log` - ring-buffer log of MCP tool calls | Pre-v6 |

---

## Priority 0: Usability (Adoption Blocker)

New users struggle to get Gasoline running. This blocks all adoption and must be fixed first.

- [ ] **Usability Improvements** — 5-minute setup goal
  - Spec: [specs/usability.md](specs/usability.md)
  - Critical: NPM package naming, binary install errors, MCP config path issues
  - High: Extension install friction, no setup verification, confusing startup options
  - Effort: 1-2 days

---

## Priority 1: Agentic CI/CD (Thesis Validation)

These features prove the thesis. Build now.

### Parallelization

```
┌─────────────────────────────────────────────────────────────────┐
│                     CAN BUILD IN PARALLEL                        │
├─────────────────────┬─────────────────────┬─────────────────────┤
│  Agent A            │  Agent B            │  Agent C            │
│                     │                     │                     │
│  33. Self-Healing   │  Gasoline CI        │  5. Context         │
│      Tests          │  Infrastructure     │     Streaming       │
│                     │                     │                     │
│  - Claude Code      │  - /snapshot        │  - MCP notifications│
│    integration      │  - /clear           │  - Push alerts      │
│  - Failure diagnosis│  - /test-boundary   │  - Event filtering  │
│  - Auto-fix loop    │  - gasoline-ci.js   │                     │
│                     │  - Playwright fix   │                     │
└─────────────────────┴─────────────────────┴─────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     THEN IN PARALLEL                             │
├─────────────────────┬─────────────────────┬─────────────────────┤
│  Agent A            │  Agent B            │  Agent C            │
│                     │                     │                     │
│  35. PR Preview     │  34. Agentic E2E    │  36. Deployment     │
│      Exploration    │      Repair         │      Watchdog       │
│                     │                     │                     │
│  - Requires: 33     │  - Requires: 33     │  - Requires: 5, CI  │
│  - Preview deploy   │  - Contract drift   │  - Post-deploy mon  │
│  - Auto-explore     │  - Test/mock update │  - Auto-rollback    │
└─────────────────────┴─────────────────────┴─────────────────────┘
```

### Features

- [ ] **33. Self-Healing Tests** — AI observes test failure, diagnoses via Gasoline context, fixes test or code autonomously
  - Branch: `feature/self-healing-tests`
  - Spec: [ai-first/tech-spec-agentic-cicd.md](ai-first/tech-spec-agentic-cicd.md)
  - Prerequisites: ✅ Tab targeting, ✅ Verification loop
  - Unlocks: CI that unblocks itself

- [ ] **Gasoline CI Infrastructure** — Headless browser capture for CI/CD pipelines
  - Spec: [gasoline-ci-specification.md](gasoline-ci-specification.md)
  - Components:
    - [ ] `/snapshot` endpoint - return all captured state
    - [ ] `/clear` endpoint - reset between tests
    - [ ] `/test-boundary` endpoint - correlate entries to tests
    - [ ] `gasoline-ci.js` - standalone capture script (extract from inject.js)
    - [ ] `@gasoline/playwright` fixture - auto-inject, auto-clear, failure attachment
  - Unlocks: Phase 8 features in CI, not just local browser

- [ ] **5. Context Streaming** — Push significant events to AI via MCP notifications
  - Branch: `feature/context-streaming`
  - Spec: v6-specification.md § Feature 5
  - Prerequisites: None
  - Unlocks: Proactive alerts, enables Deployment Watchdog

- [ ] **35. PR Preview Exploration** — Deploy preview → agent explores app → discovers bugs → proposes fixes pre-merge
  - Branch: `feature/pr-preview-exploration`
  - Spec: [ai-first/tech-spec-agentic-cicd.md](ai-first/tech-spec-agentic-cicd.md)
  - Prerequisites: ✅ Tab targeting, Self-Healing Tests (33)
  - Unlocks: Automated QA on every PR

- [ ] **34. Agentic E2E Repair** — AI detects API contract drift, updates tests/mocks automatically
  - Branch: `feature/agentic-e2e-repair`
  - Spec: [ai-first/tech-spec-agentic-cicd.md](ai-first/tech-spec-agentic-cicd.md)
  - Prerequisites: ✅ API contract validation, Self-Healing Tests (33)
  - Unlocks: Zero-maintenance E2E suites

- [ ] **36. Deployment Watchdog** — Post-deploy monitoring; AI detects regressions, triggers rollback
  - Branch: `feature/deployment-watchdog`
  - Spec: [ai-first/tech-spec-agentic-cicd.md](ai-first/tech-spec-agentic-cicd.md)
  - Prerequisites: ✅ Session comparison, Context Streaming (5), Gasoline CI
  - Unlocks: Self-healing production

---

## Priority 2: Enterprise Unlock

Required for team/enterprise sales. Build when pursuing those customers.

### Parallelization

All enterprise features are independent — can build 4+ in parallel.

```
┌───────────────┬───────────────┬───────────────┬───────────────┐
│  Agent A      │  Agent B      │  Agent C      │  Agent D      │
│               │               │               │               │
│  16. TTL      │  19. Custom   │  20. API Key  │  21. Rate     │
│  Retention    │  Redaction    │  Auth         │  Limits       │
└───────────────┴───────────────┴───────────────┴───────────────┘
```

### Features

- [ ] **16. TTL-Based Retention** — Configurable time-to-live; buffers auto-evict old entries
  - Branch: `feature/ttl-retention`
  - Complexity: Easy
  - Sales unlock: Compliance, data governance

- [ ] **19. Configurable Redaction Patterns** — User-defined regex for sensitive data (SSNs, card numbers)
  - Branch: `feature/redaction-patterns`
  - Complexity: Easy
  - Sales unlock: Privacy requirements

- [ ] **20. API Key Authentication** — Optional shared-secret for HTTP API
  - Branch: `feature/api-key-auth`
  - Complexity: Easy
  - Sales unlock: Security-conscious orgs

- [ ] **21. Per-Tool Rate Limits** — Prevent runaway AI loops (e.g., `query_dom` limited to 10/min)
  - Branch: `feature/per-tool-rate-limits`
  - Complexity: Easy
  - Sales unlock: Operational safety

- [ ] **17. Configuration Profiles** — Named bundles (short-lived, restricted, paranoid)
  - Branch: `feature/config-profiles`
  - Complexity: Medium
  - Prerequisites: 16, 19, 21
  - Sales unlock: "Bank mode" one-click setup

---

## Priority 3: Enhanced Generation

Improves quality of AI-generated artifacts. Build when self-healing is working.

- [ ] **6. Test Generation v2** — DOM assertions, fixtures, visual snapshots
  - Branch: `feature/generate-test-v2`
  - Spec: generate-test-v2.md
  - Thesis connection: Better generated tests = better self-healing input

- [ ] **7. Performance Budget Monitor** — Baseline regression detection
  - Branch: `feature/performance-budget-monitor`
  - Spec: performance-budget-spec.md
  - Thesis connection: Weak — perf monitoring isn't AI-native

---

## Priority 4: Operational Polish

Build as needed. Low thesis impact.

### Enterprise Audit (Tier 1 extras)

- [ ] **13. Client Identification** — Identify which AI client (Claude Code, Cursor, etc.)
- [ ] **14. Session ID Assignment** — Unique session ID per MCP connection
- [ ] **15. Redaction Audit Log** — Log when data is redacted (pattern, field, tool)

### Enterprise Multi-Tenant (Tier 4)

- [ ] **24. Project Isolation** — Multiple isolated capture contexts on one server
- [ ] **25. Read-Only Mode** — Accept capture data, disable mutation tools
- [ ] **26. Tool Allowlisting** — Restrict which MCP tools are available

### Developer Experience (Phase 7)

- [ ] **27. Test Fixture Page** — Built-in `/test-page` with error triggers
- [ ] **28. CLI Test Mode** — `--test` flag for automated validation
- [ ] **29. Mock Extension Client** — Go package simulating extension calls
- [ ] **30. Event Timestamps in Diagnostics** — `received_at` in `/diagnostics`
- [ ] **31. MCP Test Harness** — CLI for scripted MCP testing
- [ ] **32. CLI Lifecycle Commands** — `gasoline stop`, `restart`, `status`

### Data Export

- [ ] **18. Data Export** — Export buffer state as JSON Lines

---

## Internal Quality

### Fuzz Tests

Build incrementally as attack surface grows.

- [ ] **FuzzJSONRPCParse** — MCP message parser (no panics, no unbounded alloc)
- [ ] **FuzzHTTPBodyParse** — `/logs`, `/network-body` endpoints
- [ ] **FuzzSecurityPatterns** — Credential/PII regex (no catastrophic backtracking)
- [ ] **FuzzWebSocketFrame** — WS message handling
- [ ] **FuzzNetworkBodyStorage** — Large/malformed body storage

---

## In Progress

| Feature | Branch | Agent |
|---------|--------|-------|
| (none yet) | | |

---

## Dependency Graph

```
                    ┌──────────────────────────────────────────────────────┐
                    │                   COMPLETED                           │
                    │  Tab Targeting, API Validation, Verify Fix,          │
                    │  Session Diff, Security Tools, Audit Log             │
                    └──────────────────────────────────────────────────────┘
                                            │
                    ┌───────────────────────┼───────────────────────┐
                    │                       │                       │
                    ▼                       ▼                       ▼
            ┌───────────────┐       ┌───────────────┐       ┌───────────────┐
            │ 33. Self-     │       │ Gasoline CI   │       │ 5. Context    │
            │ Healing Tests │       │ Infrastructure│       │ Streaming     │
            └───────┬───────┘       └───────┬───────┘       └───────┬───────┘
                    │                       │                       │
        ┌───────────┴───────────┐           │               ┌───────┴───────┐
        │                       │           │               │               │
        ▼                       ▼           │               │               │
┌───────────────┐       ┌───────────────┐   │               │               │
│ 35. PR Preview│       │ 34. Agentic   │   │               │               │
│ Exploration   │       │ E2E Repair    │   │               │               │
└───────────────┘       └───────────────┘   │               │               │
                                            │               │               │
                                            └───────┬───────┘               │
                                                    │                       │
                                                    ▼                       │
                                            ┌───────────────┐               │
                                            │ 36. Deployment│◄──────────────┘
                                            │ Watchdog      │
                                            └───────────────┘
```

---

## Maximum Parallelization

**Wave 1 (Now):** 3 agents
- Agent A: Self-Healing Tests (33)
- Agent B: Gasoline CI Infrastructure
- Agent C: Context Streaming (5)

**Wave 2 (After Wave 1):** 3 agents
- Agent A: PR Preview Exploration (35)
- Agent B: Agentic E2E Repair (34)
- Agent C: Deployment Watchdog (36)

**Wave 3 (Enterprise, anytime):** 4 agents
- Agents A-D: TTL, Redaction, API Key, Rate Limits (16, 19, 20, 21)

**Total: Up to 4 parallel agents** can work productively on v6 at any time.

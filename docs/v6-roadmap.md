# v6 Roadmap

## Overview

v6 adds analysis, verification, proactive intelligence, and security hardening layers on top of existing capture infrastructure. All features operate on data Gasoline already captures — no new browser capture mechanisms needed.

Full specification: [v6-specification.md](v6-specification.md)

## Features

### Phase 1: Analysis Layer

- [ ] **1. Security Scanner (`security_audit`)** — Detect exposed credentials, missing auth, PII leaks, insecure transport, missing security headers (incl. CSP analysis), insecure cookies
  - Branch: `feature/security-audit`
  - Spec: v6-specification.md § Feature 1 (checks 1-6)
  - Status: Specified
  - Proactive: Context streaming pushes alerts for credential exposure, CSP violations, insecure cookies, and missing security headers as they are observed

- [ ] **2. API Contract Validation (`validate_api`)** — Track response shapes, detect contract violations
  - Branch: `feature/validate-api`
  - Spec: v6-specification.md § Feature 4
  - Status: Specified

### Phase 2: Verification Layer

- [ ] **3. Verification Loop (`verify_fix`)** — Before/after session comparison for fix verification
  - Branch: `feature/verify-fix`
  - Spec: v6-specification.md § Feature 2
  - Status: Specified

- [ ] **4. Session Comparison (`diff_sessions`)** — Named snapshot storage and comparison
  - Branch: `feature/diff-sessions`
  - Spec: v6-specification.md § Feature 3
  - Status: Specified

### Phase 3: Proactive Intelligence

- [ ] **5. Context Streaming** — Push significant events to AI via MCP notifications
  - Branch: `feature/context-streaming`
  - Spec: v6-specification.md § Feature 5
  - Status: Specified

### Phase 4: Enhanced Generation (pre-existing specs)

- [ ] **6. Test Generation v2 (`generate_test`)** — DOM assertions, fixtures, visual snapshots
  - Branch: `feature/generate-test-v2`
  - Spec: generate-test-v2.md
  - Status: Specified

- [ ] **7. Performance Budget Monitor (`check_performance`)** — Baseline regression detection
  - Branch: `feature/performance-budget-monitor`
  - Spec: performance-budget-spec.md
  - Status: Specified, partially implemented

### Phase 5: Security Hardening (opt-in)

Developer-triggered tools that generate security configurations from observed traffic. These don't detect problems — they produce solutions.

- [ ] **8. CSP Generator (`generate_csp`)** — Generate a Content-Security-Policy from observed resource origins
  - Branch: `feature/generate-csp`
  - Spec: ai-first/tech-spec-security-hardening.md § Tool 1
  - Status: Specified
  - Unique value: No other tool generates CSP passively from browser observation

- [ ] **9. Third-Party Risk Audit (`audit_third_parties`)** — Map all external domains, classify by risk level, domain reputation scoring, enterprise custom lists
  - Branch: `feature/audit-third-parties`
  - Spec: ai-first/tech-spec-security-hardening.md § Tool 2
  - Status: Specified
  - Includes: Bundled reputation lists (Disconnect.me, Tranco 10K, curated CDNs), domain heuristics, enterprise custom allow/block lists, optional external enrichment (RDAP, CT, Safe Browsing)

- [ ] **10. Security Regression Detection (`diff_security`)** — Compare security posture before/after code changes
  - Branch: `feature/diff-security`
  - Spec: ai-first/tech-spec-security-hardening.md § Tool 3
  - Status: Specified
  - Depends on: `diff_sessions` infrastructure (Phase 2)

- [ ] **11. SRI Hash Generator (`generate_sri`)** — Generate Subresource Integrity hashes for third-party resources
  - Branch: `feature/generate-sri`
  - Spec: ai-first/tech-spec-security-hardening.md § Tool 4
  - Status: Specified

### Phase 6: Enterprise Audit & Governance

Enterprise-readiness features that provide auditability, data governance, and operational safety for teams using Gasoline in regulated or security-conscious environments.

#### Tier 1: AI Audit Trail

- [ ] **12. Tool Invocation Log (`get_audit_log`)** — Ring-buffer log of every MCP tool call with timestamp, tool name, parameters, response size, duration, and client identity
  - Branch: `feature/audit-log`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 1.1
  - Status: Specified

- [ ] **13. Client Identification** — Identify which AI client (Claude Code, Cursor, Windsurf, etc.) is connected via MCP, recorded on every audit entry
  - Branch: `feature/client-id`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 1.2
  - Status: Specified

- [ ] **14. Session ID Assignment** — Unique session ID per MCP connection, correlating all tool calls within a session
  - Branch: `feature/session-id`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 1.3
  - Status: Specified

- [ ] **15. Redaction Audit Log** — Log every time data is redacted (what pattern matched, what field, what tool response), without storing the redacted content itself
  - Branch: `feature/redaction-audit`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 1.4
  - Status: Specified

#### Tier 2: Data Governance

- [ ] **16. TTL-Based Retention** — Configurable time-to-live for all captured data; buffers automatically evict entries older than TTL
  - Branch: `feature/ttl-retention`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 2.1
  - Status: Specified

- [ ] **17. Configuration Profiles** — Named configuration bundles (short-lived, restricted, paranoid) that set TTL, redaction, and rate limits to common security postures
  - Branch: `feature/config-profiles`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 2.2
  - Status: Specified

- [ ] **18. Data Export** — MCP tool to export current buffer state and audit entries as JSON Lines for offline retention
  - Branch: `feature/data-export`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 2.3
  - Status: Specified

- [ ] **19. Configurable Redaction Patterns** — User-defined regex patterns for redacting sensitive data from tool responses (tokens, SSNs, card numbers, custom patterns)
  - Branch: `feature/redaction-patterns`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 2.4
  - Status: Specified

#### Tier 3: Operational Safety

- [ ] **20. API Key Authentication** — Optional shared-secret authentication for the HTTP API, preventing unauthorized tools from connecting
  - Branch: `feature/api-key-auth`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 3.1
  - Status: Specified

- [ ] **21. Per-Tool Rate Limits** — Configurable rate limits per MCP tool (e.g., `query_dom` limited to 10/min) to prevent runaway AI loops
  - Branch: `feature/per-tool-rate-limits`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 3.2
  - Status: Specified

- [ ] **22. Configurable Thresholds** — All server limits (buffer sizes, memory caps, rate limits) configurable via CLI flags or config file
  - Branch: `feature/configurable-thresholds`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 3.3
  - Status: Specified

- [ ] **23. Health & SLA Metrics (`get_health`)** — MCP tool exposing server uptime, buffer utilization, memory usage, request counts, and error rates
  - Branch: `feature/health-metrics`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 3.4
  - Status: Specified

#### Tier 4: Multi-Tenant & Access Control

- [ ] **24. Project Isolation** — Multiple isolated capture contexts (projects) on a single server, each with independent buffers and configuration
  - Branch: `feature/project-isolation`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 4.1
  - Status: Specified

- [ ] **25. Read-Only Mode** — Server mode that accepts capture data but disables all mutation tools (clear, dismiss, checkpoint delete)
  - Branch: `feature/read-only-mode`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 4.2
  - Status: Specified

- [ ] **26. Tool Allowlisting** — Configuration to restrict which MCP tools are available, hiding sensitive tools from untrusted clients
  - Branch: `feature/tool-allowlist`
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 4.3
  - Status: Specified

## Dependencies

Features within a phase can be implemented in parallel. Phases can also be parallelized since there are no hard cross-feature dependencies, though completing Phase 1 before Phase 3 is recommended (context streaming's security alerts use the security audit patterns). Phase 5 tools are independent of each other; `diff_security` benefits from Phase 2's snapshot infrastructure but can be implemented standalone.

Phase 6 features are largely independent of Phases 1-5. Within Phase 6, Tier 1 (audit trail) should be implemented first as Tiers 2-4 reference audit entries. Client identification and session IDs are prerequisites for meaningful audit logs. Configuration profiles depend on TTL and redaction patterns being implemented first.

## In Progress

| Feature | Branch | Agent |
|---------|--------|-------|
| (none yet) | | |

## Internal Quality

### Fuzz Tests

Go fuzz tests (`Fuzz*` functions) for protocol parsing and input handling surfaces. These aren't user-facing features — they improve Gasoline's own resilience.

- [ ] **FuzzJSONRPCParse** — Fuzz the MCP JSON-RPC message parser with malformed payloads
  - Target: `cmd/dev-console/main.go` (MCP handler)
  - Goal: No panics, no unbounded allocations on arbitrary input

- [ ] **FuzzHTTPBodyParse** — Fuzz the `/logs` and `/network-body` HTTP endpoints
  - Target: `cmd/dev-console/main.go` (HTTP handlers)
  - Goal: All malformed bodies return 400, never panic

- [ ] **FuzzSecurityPatterns** — Fuzz the credential/PII regex patterns
  - Target: `cmd/dev-console/security.go` (when implemented)
  - Goal: No regex catastrophic backtracking on adversarial input

- [ ] **FuzzWebSocketFrame** — Fuzz WebSocket message handling
  - Target: `cmd/dev-console/websocket.go`
  - Goal: Malformed frames handled gracefully, buffer limits respected

- [ ] **FuzzNetworkBodyStorage** — Fuzz large/malformed network body storage
  - Target: `cmd/dev-console/network.go`
  - Goal: Memory limits enforced, no OOM on adversarial payloads

**When to run:** Fuzz tests run in CI with a short corpus seed (`-fuzztime=30s`). Extended fuzzing (`-fuzztime=5m`) runs as part of the release PR skill.

## Completed

| Feature | Branch | Merged |
|---------|--------|--------|
| (none yet) | | |

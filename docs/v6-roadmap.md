# v6 Roadmap

## Overview

v6 adds analysis, verification, and proactive intelligence layers on top of existing capture infrastructure. All features operate on data Gasoline already captures — no new browser capture mechanisms needed.

Full specification: [v6-specification.md](v6-specification.md)

## Features

### Phase 1: Analysis Layer

- [ ] **1. Security Scanner (`security_audit`)** — Detect exposed credentials, missing auth, PII leaks, insecure transport
  - Branch: `feature/security-audit`
  - Spec: v6-specification.md § Feature 1
  - Status: Specified

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

## Dependencies

Features within a phase can be implemented in parallel. Phases can also be parallelized since there are no hard cross-feature dependencies, though completing Phase 1 before Phase 3 is recommended (context streaming's security alerts use the security audit patterns).

## In Progress

| Feature | Branch | Agent |
|---------|--------|-------|
| (none yet) | | |

## Completed

| Feature | Branch | Merged |
|---------|--------|--------|
| (none yet) | | |

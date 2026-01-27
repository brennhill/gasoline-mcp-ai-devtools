# Agentic E2E Repair Review

_Migrated from /specs/agentic-e2e-repair-review.md_

# Agentic E2E Repair (Feature 34) - Technical Review

**Reviewer:** Principal Engineer
**Date:** 2026-01-26
**Spec:** `docs/ai-first/tech-spec-agentic-cicd.md` (Lines 120-148)
**Status:** Review Complete

---

## Executive Summary

Feature 34 (Agentic E2E Repair) is a well-scoped extension of the Self-Healing Tests pattern (Feature 33), specializing in API contract drift detection and automated test/mock remediation. The architecture correctly positions this as an **orchestration layer** on top of existing Gasoline primitives (`observe`, `analyze {target: "api"}`, `validate_api`) rather than new MCP tooling. However, the spec lacks critical operational details around codebase search scope, fix selection heuristics, and rollback boundaries that could lead to unbounded agent behavior or destructive changes in production codebases.

---

## 1. Critical Issues (Must Fix Before Implementation)

### 1.1 Unbounded Codebase Search (Line 135)

**Issue:** The workflow states "Agent searches codebase for all 'userName' references" without defining:
- Search scope boundaries (which directories/file patterns)
- Exclusion rules (node_modules, dist, vendor, generated files)
- Maximum file count before requiring human approval

**Risk:** An agent could modify hundreds of files, including generated code, third-party dependencies, or unrelated subsystems that happen to use similar naming.

**Recommendation:** Add explicit scoping:
```yaml
search_constraints:
  include_patterns: ["src/**", "tests/**", "lib/**"]
  exclude_patterns: ["**/node_modules/**", "**/dist/**", "**/generated/**"]
  max_files_before_approval: 10
  require_approval_for: ["*.json", "*.yaml", "package*.json"]
```

### 1.2 Missing Fix Selection Criteria (Line 136)

**Issue:** The spec states "Agent proposes: Update frontend to use 'user_name', OR update test mocks" but provides no decision tree for which fix to apply. This is a critical ambiguity.

**Risk:** Choosing wrong:
- Updating frontend code when the API change was unintentional breaks production
- Updating mocks when the API change was intentional creates false-passing tests

**Recommendation:** Add explicit decision heuristics:
```yaml
fix_selection:
  prefer_mock_update_when:
    - api_change_in_same_pr: false  # Change came from backend team
    - test_is_unit_or_integration: true
    - affected_files_in_different_repo: true
  prefer_frontend_update_when:
    - api_change_in_same_pr: true  # Developer making coordinated change
    - openapi_spec_matches_new_shape: true
    - changelog_mentions_breaking_change: true
  require_human_decision_when:
    - both_strategies_viable: true
    - change_affects_production_code: true
```

### 1.3 No Validation That Fix Matches Actual API (Line 138)

**Issue:** Step 8 says "Agent verifies all affected tests pass" but passing tests only prove syntactic correctness, not semantic correctness. If the agent updates frontend code to match a *buggy* API response, tests pass but the app is broken.

**Risk:** The agent could propagate backend bugs into the frontend codebase, creating a cascade of "working" but incorrect code.

**Recommendation:** Add contract source verification:
```yaml
verification:
  before_fix:
    - compare_actual_api_to_openapi_spec  # If spec exists
    - check_if_api_change_was_deployed_intentionally
    - flag_if_api_response_contains_error_patterns
  after_fix:
    - verify_tests_pass: true
    - verify_no_new_console_errors: true
    - verify_api_call_succeeds_with_expected_status: true
```

### 1.4 Race Condition: API State During Analysis vs Repair (Lines 131-138)

**Issue:** The workflow captures API response at failure time (step 2), but the actual fix happens later. The API could change again between capture and fix application, making the repair incorrect.

**Risk:** In CI environments with concurrent deployments, the agent could fix code for an API version that no longer exists.

**Recommendation:** Add staleness checks:
```yaml
preconditions:
  max_capture_age_seconds: 300  # Re-capture if older than 5 minutes
  verify_api_still_returns_captured_shape: true
  abort_if_api_schema_changed_during_repair: true
```

---

## 2. Performance Analysis

### 2.1 Memory Impact

**Current State:** The `validate_api` tool (implemented in `api_contract.go`) already tracks up to 30 endpoints with status history of 20 calls each. Memory overhead is well-bounded (~300KB as documented in v6-specification.md).

**E2E Repair Addition:** The skill orchestration layer runs in Claude Code, not Gasoline server. No additional server memory impact.

**Assessment:** No performance concerns.

### 2.2 Hot Path Analysis

The critical path is:
1. `validate_api {action: "analyze"}` - O(n) over network bodies, already optimized
2. Codebase search - Delegated to Claude Code's grep/glob tools, not Gasoline's responsibility
3. File modification - Claude Code's edit tool

**Assessment:** No Gasoline hot paths introduced.

### 2.3 Buffer Sizing

The existing `maxContractEndpoints = 30` and `maxStatusHistory = 20` limits (api_contract.go lines 26-27) are appropriate for the E2E repair use case. A typical E2E test suite exercises 10-50 endpoints.

**Recommendation:** Consider exposing these as configurable for teams with larger API surfaces, but current defaults are reasonable.

---

## 3. Concurrency Analysis

### 3.1 Lock Analysis

The `APIContractValidator` uses a single `sync.RWMutex` (api_contract.go line 40) protecting the trackers map. Current usage pattern:
- `Learn()` takes write lock
- `Validate()` takes write lock
- `Analyze()` takes read lock
- `Report()` takes read lock

**Issue:** Both `Learn()` and `Validate()` call each other's functionality internally while holding locks, but since Go's RWMutex is not reentrant, this is handled correctly by keeping all lock acquisitions at the top-level methods.

**Assessment:** No deadlock risk. Lock contention is minimal for typical E2E test throughput.

### 3.2 Goroutine Lifecycle

The validator has no goroutines. All operations are synchronous request handlers.

**Assessment:** No goroutine leak risk.

### 3.3 CI Parallel Workers

The spec references the Gasoline CI infrastructure which supports parallel Playwright workers (docs/gasoline-ci-specification.md). Each worker POSTs to the same Gasoline server.

**Issue:** With parallel workers, the `validate_api` tracker accumulates data from ALL workers simultaneously. This could cause:
- Cross-contamination of endpoint tracking between test suites
- False positive "error_spike" detection when different workers hit different API states

**Recommendation:** Add test isolation via the existing `test_id` tagging mechanism:
```yaml
# In skill definition
isolation:
  scope_contract_tracking_to_test_id: true
  clear_validator_between_test_suites: true
```

---

## 4. Data Contract Analysis

### 4.1 API Surface Consistency

The feature relies on three existing MCP tools:
- `observe {what: "network"}` - Stable, well-tested
- `analyze {target: "api"}` - Returns schema from SchemaStore
- `validate_api` - Returns `APIContractAnalyzeResult`

**Assessment:** All tools have defined response schemas. No breaking changes introduced.

### 4.2 Violation Type Completeness

Current violation types in `api_contract.go`:
- `shape_change` - Missing fields
- `type_change` - Field type changed
- `error_spike` - Success-to-error transition
- `new_field` - Unexpected field appeared
- `null_field` - Field became null

**Missing for E2E Repair:**

# ...existing code...

---
status: shipped
version-applies-to: v5.3
scope: feature-review/security
ai-priority: high
tags: [review, sarif-export, security, compliance]
last-verified: 2026-01-26
relates-to: [product-spec.md, tech-spec.md, adr-sarif-export.md]
supersedes: [SARIF_EXPORT_review.md, sarif-export-review-critical-issues.md, sarif-export-review-recommendations.md]
---

# SARIF Export Feature Review

**Status:** Shipped with known issues requiring fixes
**Priority:** High (security-related issues identified)
**Last Updated:** 2026-01-26
**Reviewer Notes:** Implementation is production-ready but has 3 critical issues + 8 recommendations

---

## Executive Summary

The SARIF Export feature implementation in `export_sarif.go` is complete and well-structured. The feature can ship with the identified issues, but they should be addressed in the next sprint to improve GitHub Code Scanning integration and security hardening.

### Key Findings
- ✅ Core implementation is sound and complete
- ✅ Axe-core to SARIF mapping is correct
- ❌ **3 Critical Issues** identified (source mapping, path traversal, cache TTL)
- ⚠️ **8 Recommendations** for improvement

---

## Critical Issues

### 1. Source Location Mapping is Mostly Fictional

**Severity:** HIGH (affects core feature value)
**Component:** `export_sarif.go` (lines 230-252)

#### Problem

The spec describes three source location strategies:
1. `data-component` or `data-testid` attribute mapping
2. Source maps + framework fiber data
3. Fallback to CSS selector path

In practice:
- Strategy 2 requires `chrome.debugger` attachment (avoided by design)
- Strategy 1 only works if apps use `data-component` attributes (not common)
- Nearly all output falls back to Strategy 3 (CSS selector)

The current implementation puts CSS selectors directly into `artifactLocation.uri`, producing invalid URIs like `html > body > main > div.card > img`. GitHub Code Scanning will not create inline annotations from invalid URIs, so violations appear in "Other" section instead of inline.

#### Impact
- Lower usability for GitHub Code Scanning integration (high-value feature, least reliable)
- Violations not highlighted inline where developers can see them

#### Fix
**Immediate (v5.3+):**
- Set `artifactLocation.uri` to page URL (e.g., `http://localhost:3000/dashboard`)
- Move CSS selector to `logicalLocations[0].fullyQualifiedName` (SARIF standard)
- Produces valid SARIF and groups violations by page

**Future enhancement:**
- Lightweight heuristic: match `data-testid` patterns to source files (e.g., `UserCard` → `src/components/UserCard.tsx`)
- Works without debugger, covers React/Vue projects with naming conventions

---

### 2. Path Traversal in `saveSARIFToFile` is Incomplete

**Severity:** HIGH (security vulnerability)
**Component:** `export_sarif.go` (lines 287-299)

#### Problem

The path validation check has three flaws:

**Symlink Bypass:**
- `filepath.Abs` resolves symlinks, but `strings.HasPrefix` compares against unresolved `cwd`
- Attacker could craft paths resolving through symlink to outside allowed directory

**Windows Compatibility:**
- Hardcoded `/tmp` check is Unix-specific
- On Windows, `os.TempDir()` returns different path; check becomes dead code

**Relative Path Components:**
- `./../../etc/passwd` partially mitigated (Abs resolves `..`), but symlinks remain open

#### Impact
- Path traversal possible via symlink attack
- Windows systems have weaker path validation

#### Fix
```go
// Use filepath.EvalSymlinks on both paths for comparison
resolvedPath, err := filepath.EvalSymlinks(filepath.Dir(absPath))
resolvedCwd, err := filepath.EvalSymlinks(cwd)

// For cross-platform safety, use os.TempDir() instead of hardcoded /tmp
if !strings.HasPrefix(resolvedPath, resolvedCwd) && 
   !strings.HasPrefix(resolvedPath, filepath.Dir(os.TempDir())) {
  return fmt.Errorf("path outside allowed directory")
}
```

---

### 3. Stale Cache TTL Contradiction

**Severity:** MEDIUM (correctness issue)
**Component:** Audit cache interaction (`types.go` line 367, `export_sarif_test.go`)

#### Problem

Spec says: "The `export_sarif` tool always re-runs the audit unless explicitly told to use cache."

But implementation uses a11y cache with 30-second TTL. If agent:
1. Calls `run_accessibility_audit`
2. Modifies the page
3. Calls `export_sarif` within 30 seconds

...the SARIF output will reflect pre-modification state (cache hit). This violates spec intent.

#### Impact
- Stale compliance evidence in SARIF export
- Misleads security/compliance review with outdated data

#### Fix
Add `force_refresh` parameter to `export_sarif` (default `true`):
- When true: bypass a11y cache
- When false: use cache if available
- Recommend always true for SARIF export (re-audit cost justified by accuracy need)

Or simpler: always bypass cache for SARIF export.

---

## Recommendations

### A. SARIF Schema Validation in Tests

**Priority:** HIGH | **Effort:** LOW

Current tests (`export_sarif_test.go`) may only check JSON structure, not SARIF 2.1.0 schema compliance.

**Add:**
- Load official SARIF 2.1.0 JSON Schema
- Validate required fields: `$schema`, `version`, `runs[0].tool.driver.name`, `runs[0].tool.driver.version`
- Verify referential integrity: each `ruleId` has corresponding rule in `rules` array
- Validate `ruleIndex` points to valid index

---

### B. Version Hardcoding

**Priority:** MEDIUM | **Effort:** LOW

Implementation references version constant at line 159. Ensure:
- Same version constant used across project
- SARIF output included in version sync process (see `.claude/docs/version-management.md`)
- Current implementation shows "4.0.0"; verify this matches actual version

---

### C. SARIF `kind` Field for Passes

**Priority:** MEDIUM | **Effort:** LOW

When `include_passes=true`, passed rules should use `kind: "pass"` per SARIF 2.1.0 spec.

**Current:** Sets `level: "none"` for passes (missing `kind` field)
**Expected:** `kind: "pass"` for passes, `kind: "fail"` for violations

**Add to SARIFResult:**
```go
Kind string `json:"kind"` // "pass", "fail", "review", "open", "informational", "notApplicable"
```

---

### D. GitHub Code Scanning Upload Size Limit

**Priority:** LOW | **Effort:** LOW

GitHub accepts SARIF files up to 10MB (gzip). Large pages with hundreds of violations can exceed this.

**Spec says:** "GitHub handles large SARIF files gracefully (up to 5000 results)"

**Recommendation:** Cap results at 5000 with truncation note in response.

---

### E. Missing `incomplete` Results

**Priority:** MEDIUM | **Effort:** MEDIUM

Axe-core includes `incomplete` violations (need manual review, e.g., color contrast unable to compute).

**Current:** Silently dropped
**Should be:** Map to SARIF `kind: "review"` with `level: "warning"`

**Add parameter:** `include_incomplete` (default true)

---

## Implementation Roadmap

Ordered by impact and effort:

1. **Fix `artifactLocation.uri`** (HIGH impact, MEDIUM effort)
   - Use page URL instead of CSS selector
   - Move CSS selector to `logicalLocations[0].fullyQualifiedName`

2. **Add `kind` field** (MEDIUM impact, LOW effort)
   - Set `"fail"` for violations, `"pass"` for passes, `"review"` for incomplete

3. **Fix symlink traversal** (HIGH impact, MEDIUM effort)
   - Use `filepath.EvalSymlinks` on both paths
   - Remove hardcoded `/tmp`, use `os.TempDir()`

4. **Add `force_refresh` parameter** (MEDIUM impact, LOW effort)
   - Default true, bypass a11y cache for SARIF export

5. **Add `incomplete` results** (MEDIUM impact, MEDIUM effort)
   - Process `incomplete` violations with `include_incomplete` parameter

6. **Add SARIF structural validation** (LOW impact, MEDIUM effort)
   - Test `ruleIndex` referential integrity
   - Validate required fields and `kind` values

7. **Add results cap** (LOW impact, LOW effort)
   - 5000 entries max with truncation note

8. **Verify version constant** (LOW impact, LOW effort)
   - Ensure included in version sync process

---

## Related Documents

- **product-spec.md** — Feature requirements and SARIF format spec
- **tech-spec.md** — Implementation details and API reference
- **adr-sarif-export.md** — Architecture decision record
- **../../../core/known-issues.md** — Current blockers for v5.3

---

## Review Metadata

- **Reviewed by:** Principal Engineer
- **Review Date:** 2026-01-26
- **Source Document:** `/docs/specs/sarif-export-review.md` (migrated and consolidated)
- **Status:** Ready to ship with known issues
- **Next Review:** Post-fixes (after implementing critical issues)

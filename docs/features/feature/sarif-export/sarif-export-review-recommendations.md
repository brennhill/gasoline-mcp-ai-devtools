---
> **[MIGRATED: 2026-01-26 | Source: /docs/specs/sarif-export-review.md]**
> This file contains the recommendations and implementation roadmap for the SARIF Export review, split for size and clarity.
---

# SARIF Export Review — Recommendations & Implementation Roadmap

## Recommendations

### A. SARIF Schema Validation in Tests

Test scenario 11 says "Schema validates against SARIF 2.1.0 spec." The current test file (`export_sarif_test.go`) likely checks JSON structure but may not validate against the official SARIF JSON Schema. Go's `encoding/json` does not do schema validation.

Add a test that loads the generated SARIF and validates required fields: `$schema`, `version`, `runs[0].tool.driver.name`, `runs[0].tool.driver.version`. Also validate that every result has a `ruleId` that corresponds to a rule in the `rules` array (referential integrity). The SARIF spec requires `ruleIndex` to be a valid index into the `rules` array -- test this with multiple violations of the same rule.

### B. Version Hardcoding

The implementation references `version` (line 159 of `export_sarif.go`): `Version: version`. Verify this is the same version constant used across the project. The spec example shows `"version": "4.0.0"` (line 129) but the project likely tracks a different version. The version management doc (`.claude/docs/version-management.md`) describes 14 locations where version is synced -- ensure SARIF output is included.

### C. SARIF `kind` Field for Passes

When `include_passes` is true, the spec says passed rules should use `kind: "pass"` (test scenario 13, line 197). The current `nodeToResult` function (lines 230-252) does not set a `kind` field -- it only sets `level: "none"` for passes. SARIF 2.1.0 defines `result.kind` as an enum: `pass`, `fail`, `review`, `open`, `informational`, `notApplicable`. For violations, `kind` defaults to `fail`. For passes, it should be explicitly `pass`.

Add a `kind` field to `SARIFResult` and set it to `"pass"` when generating pass results, `"fail"` for violations.

### D. GitHub Code Scanning Upload Size Limit

GitHub accepts SARIF files up to 10MB after gzip compression. For a page with hundreds of violations, each with HTML snippets, the SARIF file can grow large. The spec says "GitHub handles large SARIF files gracefully (up to 5000 results per file)" (line 166). Consider adding a results cap (e.g., 5000) to stay within GitHub's limits, with a summary note indicating truncation.

### E. Missing `incomplete` Results

The axe-core result includes `incomplete` violations (lines 121 of `export_sarif.go` -- the type is defined but not used in `ExportSARIF`). These are elements that need manual review (e.g., color contrast that could not be computed). They should map to SARIF `kind: "review"` and `level: "warning"`. Currently they are silently dropped.

**Fix**: Add a parameter `include_incomplete` (default true) and map incomplete results to `kind: "review"`.

## Implementation Roadmap

1. **Fix `artifactLocation.uri`** -- Use page URL instead of CSS selector. Move CSS selector to `logicalLocations[0].fullyQualifiedName`. This is the highest-impact change for GitHub Code Scanning usability.
2. **Add `kind` field** to `SARIFResult`. Set `"fail"` for violations, `"pass"` for passes, `"review"` for incomplete.
3. **Fix symlink traversal** in `saveSARIFToFile` using `filepath.EvalSymlinks`. Remove hardcoded `/tmp`.
4. **Add `force_refresh` parameter** (default true) to bypass a11y cache for SARIF export.
5. **Add `incomplete` results** processing with `include_incomplete` parameter.
6. **Add SARIF structural validation test** -- verify `ruleIndex` referential integrity, required fields, and `kind` values.
7. **Add results cap** at 5000 entries with a truncation note in the tool response.
8. **Verify version constant** is included in the version sync process.

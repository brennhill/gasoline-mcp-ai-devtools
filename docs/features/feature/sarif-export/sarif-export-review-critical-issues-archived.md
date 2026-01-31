---
> **[MIGRATED: 2026-01-26 | Source: /docs/specs/sarif-export-review.md]**
> This file contains the critical issues section of the SARIF Export review, split for size and clarity.
---

# SARIF Export Review — Critical Issues

## Executive Summary

This is the cleanest spec of the four and the most production-ready. The implementation in `export_sarif.go` is complete, well-structured, and handles the axe-core to SARIF mapping correctly. The critical issues are limited to the source location mapping (the highest-value feature is the least reliable), a path traversal surface in `saveSARIFToFile`, and missing SARIF validation against the 2.1.0 schema. This spec can ship with minimal changes.

## Critical Issues

### 1. Source Location Mapping is Mostly Fictional

The spec describes three source location strategies (lines 63-70):
1. `data-component` or `data-testid` attribute mapping
2. Source maps + framework fiber data
3. Fallback to CSS selector path

Strategy 2 is described as "best-effort" and "optional." In practice, it requires `chrome.debugger` attachment (to access React DevTools protocol), which this project explicitly avoids as default (see reproduction spec review). Strategy 1 only works if the app uses `data-component` attributes, which most do not.

The result: nearly all SARIF output will use Strategy 3 (CSS selector as URI). The current implementation in `export_sarif.go` (lines 230-252) puts the CSS selector into `artifactLocation.uri`, which produces URIs like `html > body > main > div.card > img`. This is not a valid URI per the SARIF spec, and GitHub Code Scanning will not create inline annotations from it. The result is that violations appear in the "Other" section of the Code Scanning UI, which is significantly less useful than inline annotations.

**Fix**: For the initial release, be honest about this limitation. Set `artifactLocation.uri` to the page URL (e.g., `http://localhost:3000/dashboard`) and move the CSS selector to `logicalLocations` (a SARIF-standard field for logical paths). This produces valid SARIF and at least groups violations by page.

For a future version, consider a lightweight heuristic: if a `data-testid` attribute matches a filename pattern (e.g., `data-testid="UserCard"` might correspond to `src/components/UserCard.tsx`), use that as the artifact URI. This works without debugger attachment and covers React/Vue projects that follow naming conventions.

### 2. Path Traversal in `saveSARIFToFile` is Incomplete

The security check in `export_sarif.go` (lines 287-299) validates that the absolute path starts with `cwd` or `/tmp`. However:
- **Symlink bypass**: If `cwd` contains a symlink, `filepath.Abs` resolves it, but `strings.HasPrefix` compares against the unresolved `cwd`. An attacker could craft a path that resolves through a symlink to outside the allowed directory.
- **Relative path components**: The path `./../../etc/passwd` resolves via `filepath.Abs` to an absolute path, but `filepath.Abs` does resolve `..` components, so this specific attack is handled. The concern is symlinks, not relative components.
- **Windows compatibility**: The check uses `/tmp` with a forward slash, which is Unix-specific. On Windows, `os.TempDir()` returns a different path and the `/tmp` check is dead code.

**Fix**: Use `filepath.EvalSymlinks` on both the target path and `cwd` before comparison. For cross-platform safety, drop the hardcoded `/tmp` check and rely only on `os.TempDir()` and the resolved `cwd`.

```go
resolvedPath, err := filepath.EvalSymlinks(filepath.Dir(absPath))
resolvedCwd, err := filepath.EvalSymlinks(cwd)
```

### 3. Stale Cache TTL Contradiction

The spec says (line 170): "The `export_sarif` tool always re-runs the audit unless explicitly told to use cache." But the implementation uses the a11y cache with a 30-second TTL (`a11yCacheTTL` in types.go line 367). The MCP tool handler for `export_sarif` calls the same audit path as `run_accessibility_audit`, which checks the cache first.

If the agent runs `run_accessibility_audit`, modifies the page, then immediately calls `export_sarif`, the SARIF output will reflect the pre-modification state (cache hit within 30 seconds). This is the exact scenario the spec warns against.

**Fix**: Add a `force_refresh` parameter to `export_sarif` (default `true`). When true, bypass the a11y cache. The existing `SARIFExportOptions` struct already has `Scope` and `IncludePasses` -- add `ForceRefresh bool`. Alternatively, simply always bypass cache for SARIF export, since the cost of re-running the audit is justified by the need for accurate compliance evidence.

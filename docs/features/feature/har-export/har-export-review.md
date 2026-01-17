---
status: shipped
scope: feature/har-export/review
ai-priority: high
tags: [review, issues]
relates-to: [tech-spec.md, product-spec.md]
last-verified: 2026-01-31
---

# HAR Export Spec Review

## Executive Summary

The spec describes a well-scoped feature -- converting already-captured `NetworkBody` data into HAR 1.2 format. The implementation already exists in `export_har.go` and is simpler and more conservative than the spec implies. The primary risk is that the spec over-promises features (resource timing breakdown, `output_path` with directory creation, `time_range` filtering, `include_bodies` toggle, `redact_headers`, `pages` section) that the implementation does not deliver, creating a contract gap that will confuse future contributors and AI consumers.

---

## Critical Issues

### 1. Spec-Implementation Divergence (Sections: MCP Tool, Privacy & Security, Data Model, Edge Cases)

The spec defines an `export_har` tool with 7 parameters. The implementation exposes HAR via `generate { format: "har" }` with only 4 parameters: `url`, `method`, `status_min`, `status_max`, `save_to`. The following spec features are absent from the implementation:

| Spec Feature | Spec Section | Implementation Status |
|---|---|---|
| `output_path` with auto-created directories | MCP Tool `export_har` | Missing. `save_to` exists but does not create parent directories (test at line 877 confirms this). |
| `time_range` filter (`start`/`end` ISO 8601) | MCP Tool parameters | Not implemented. `NetworkBodyFilter` has no time fields. |
| `include_bodies` toggle (default true) | MCP Tool parameters | Not implemented. Bodies are always included. |
| `redact_headers` parameter | MCP Tool parameters, Privacy & Security | Not implemented. No header redaction occurs in HAR export. The existing `redactionEngine` on the `ToolHandler` is not applied to HAR output. |
| Authorization header replacement with `[REDACTED]` | Privacy & Security, item 1 | Not implemented. `NetworkBody` does not store request headers; it only has `HasAuthHeader` boolean and `ResponseHeaders` map. There are no request headers to redact. |
| Cookie value redaction | Privacy & Security, item 2 | Not implemented. `NetworkBody` has no cookie data. |
| `pages` section with `pageref` linkage | Data Model, Pages Section | Not implemented. No page load grouping exists. |
| Granular timing breakdown from resource timing | Timing Distribution, approach 1 | Not implemented. All timing is mapped to `wait` with `send: -1, receive: -1` (correct fallback per spec approach 2, but approach 1 is not available). |
| Binary response base64 encoding | Edge Cases | Not implemented. `BinaryFormat` field on `NetworkBody` is not used during HAR conversion. |
| Response `content.encoding: "base64"` | Edge Cases | Not implemented. |
| Summary response with `total_size_bytes`, `time_range`, asset breakdown | MCP Tool response example | `HARExportResult` returns `saved_to`, `entries_count`, `file_size_bytes` -- no time range or breakdown. |

**Recommendation:** Either update the spec to match the implementation (preferred -- the implementation is solid), or create a phased roadmap for the missing features. The spec should never describe behavior that does not exist.

### 2. No Header Redaction in HAR Output (Section: Privacy & Security)

The spec promises four levels of automatic sanitization: Authorization headers, cookie values, bearer tokens/API keys, and custom header redaction. The implementation performs zero redaction on HAR output. While `NetworkBody.ResponseHeaders` is populated, no request headers are stored, and neither set is filtered during HAR conversion.

The `redactionEngine` on `ToolHandler` operates on `json.RawMessage` tool responses. It runs after `toolExportHAR` returns (line 307 in `tools.go`), so it applies regex-based scrubbing to the serialized HAR text. This is a partial mitigation -- the built-in patterns in `RedactionEngine` will catch some tokens in response bodies -- but it does not handle the structured header/cookie redaction the spec describes.

**Risk:** If response bodies contain auth tokens, API keys, or session data, the exported HAR file will contain them in cleartext. A developer who trusts the spec's privacy claims and shares the HAR with a colleague is leaking credentials.

**Recommendation:** Add explicit header and body redaction in `networkBodyToHAREntry`. For `ResponseHeaders`, iterate and replace values matching sensitive header names (`Authorization`, `Cookie`, `Set-Cookie`, `X-API-Key`, etc.) with `[REDACTED]`. For response/request body text, apply the existing `redactionEngine` patterns.

### 3. `isPathSafe` is Insufficient for CWD-Relative Writes (Section: Path Validation, lines 264-278)

The path validation allows any relative path that does not contain `..`, and absolute paths only under `/tmp` or `os.TempDir()`. This means:

- `save_to: "output.har"` writes to the server's CWD, which in MCP mode is wherever the user launched `gasoline`. This could be `/` if launched from there.
- The spec suggests writing to `.gasoline/reports/` (response example shows `/path/to/project/.gasoline/reports/capture-2026-01-24.har`), but the implementation has no concept of a project-relative path.
- On macOS, `os.TempDir()` returns a unique per-boot path like `/var/folders/xx/.../T/`, and `/tmp` is a symlink to `/private/tmp`. The `strings.HasPrefix` check will fail for `/tmp/foo` if `filepath.Clean` resolves the symlink. In practice this works because `filepath.Clean` does not resolve symlinks, but it is fragile.

**Recommendation:** Either restrict `save_to` to absolute paths under `os.TempDir()` only (simplest, safest), or implement a project-relative path resolver using the CWD from `NewToolHandler`. Document the allowed path set explicitly in the tool schema description.

---

## Recommendations

### 4. Unbounded HAR JSON Response Over MCP (Section: Performance Constraints)

When `save_to` is not specified, `toolExportHAR` serializes the entire HAR into a JSON string and returns it as an MCP text content block. With 100 network bodies (the buffer max) each containing up to 16KB response + 8KB request bodies, the worst-case payload is ~2.4MB of body data plus overhead. The MCP scanner buffer is 10MB (line 964 of `main.go`), so this fits, but the spec's "warning if >10MB" is not implemented.

**Recommendation:** Add a size check after marshaling. If the HAR JSON exceeds a threshold (e.g., 5MB), return an error suggesting the use of `save_to` or filters. This prevents blowing up MCP clients that have lower message limits.

### 5. `ExportHAR` Calls `GetNetworkBodies` Which Reverses and Limits (Section: Export Functions)

`ExportHAR` (line 200) calls `GetNetworkBodies` with a limit of 10000. `GetNetworkBodies` (line 111 in `network.go`) filters, reverses to newest-first, then truncates to `limit`. `ExportHAR` then reverses again to get chronological order. This double-reverse is correct but wasteful. More importantly, `GetNetworkBodies` acquires `v.mu.RLock()` and `ExportHAR` is called from `toolExportHAR` which does not hold the lock -- this is correct because `GetNetworkBodies` handles its own locking.

However, the default limit in `GetNetworkBodies` is 20 (`defaultBodyLimit`). The override to 10000 in `ExportHAR` is a magic number that happens to exceed `maxNetworkBodies` (100). If `maxNetworkBodies` ever increases, the 10000 works, but if someone lowers `defaultBodyLimit` without checking `ExportHAR`, the export silently truncates.

**Recommendation:** Add `ExportAll bool` to `NetworkBodyFilter` that bypasses the limit, or use `math.MaxInt` with a comment. Replace the double-reverse with direct oldest-first iteration in a dedicated export path.

### 6. PostData MimeType Uses Response ContentType (Lines 155-159 of export_har.go)

When building `HARPostData`, the code sets `MimeType` to `body.ContentType`. But `ContentType` on `NetworkBody` is the response content type, not the request content type. A POST request sending `application/json` to an endpoint that returns `text/html` will have `postData.mimeType: "text/html"` in the HAR. This is semantically wrong.

**Recommendation:** `NetworkBody` needs a `RequestContentType` field, or the HAR export should omit `MimeType` when it cannot distinguish request vs. response content type. At minimum, set `postData.mimeType` to `"application/x-www-form-urlencoded"` as a default placeholder, or leave it empty.

### 7. Missing `cookies` Field in HAR Request/Response (Section: Mapping Gasoline Data to HAR)

The HAR 1.2 spec requires `request.cookies` and `response.cookies` arrays (can be empty). The current `HARRequest` and `HARResponse` structs omit these fields. Strict HAR validators (like the official `har-validator` npm package) will reject the output.

**Recommendation:** Add `Cookies []HARCookie` with `json:"cookies"` to both structs, initialized to empty slices (not nil). This matches the pattern already used for `Headers` and `QueryString`.

### 8. `HARTimings` Missing Required Fields (Section: HAR 1.2 Types)

HAR 1.2 requires `blocked`, `dns`, `connect`, `send`, `wait`, `receive`, `ssl`. The implementation only has `send`, `wait`, `receive`. Missing fields should be set to `-1` (meaning "not available" per spec).

**Recommendation:** Add `Blocked`, `DNS`, `Connect`, `SSL` fields to `HARTimings`, all defaulting to `-1`.

### 9. No `Entries` Null-Safety in JSON Serialization (Already Handled)

The tests verify that `entries` serializes as `[]` not `null` (line 319 of test file). This is correctly handled because `entries` is initialized as `make([]HAREntry, 0, ...)`. Good -- no change needed.

### 10. Spec Test Scenarios Not Matched by Implementation Tests

The spec lists 20 test scenarios. Mapping coverage:

| Spec Test | Covered? |
|---|---|
| 1. Single GET | Yes (TestNetworkBodyToHAREntry/basic GET) |
| 2. POST with body | Yes |
| 3. JSON response content type | Yes (response body in content) |
| 4. Multiple entries chronological | Yes |
| 5. URL filter | Yes |
| 6. Status filter | Yes |
| 7. Authorization header redacted | No (not implemented) |
| 8. Cookie values redacted | No (not implemented) |
| 9. Resource timing granular | No (not implemented) |
| 10. No resource timing fallback | Yes (duration maps to timings.wait) |
| 11. Binary response base64 | No (not implemented) |
| 12. Truncated body comment | Yes |
| 13. Empty buffer | Yes |
| 14. Output path creates directories | No (not implemented) |
| 15. Multiple page loads | No (not implemented) |
| 16. HAR validates against schema | Partial (version/creator checked, not full schema validation) |
| 17. include_bodies: false | No (not implemented) |
| 18. Very large export warning | No (not implemented) |
| 19. time_range filter | No (not implemented) |
| 20. Custom redact_headers | No (not implemented) |

11 of 20 spec tests are not covered because the features do not exist.

---

## Implementation Roadmap

Given the spec-implementation gap, I recommend a two-phase approach:

### Phase 1: Harden What Exists (1-2 days)

These changes make the existing implementation spec-compliant for the features it does support, and fix the correctness issues.

1. **Update spec to match implementation.** Remove or mark as "Future" the features that do not exist: `time_range`, `include_bodies`, `redact_headers`, `pages` section, resource timing breakdown, `output_path` auto-directory-creation.
2. **Add missing required HAR fields.** Add `cookies` (empty array) to `HARRequest` and `HARResponse`. Add `blocked`, `dns`, `connect`, `ssl` (all `-1`) to `HARTimings`. Write tests first.
3. **Fix `postData.mimeType` bug.** Either add `RequestContentType` to `NetworkBody` (requires extension change) or default to empty string with a comment noting the limitation.
4. **Add response size warning.** After marshaling HAR JSON in the no-`save_to` path, check size and return a warning if over 2MB.
5. **Add header redaction for `ResponseHeaders`.** In `networkBodyToHAREntry`, populate `response.headers` from `body.ResponseHeaders`, redacting sensitive header names. This uses data already captured.

### Phase 2: Expand Features (3-5 days, optional)

These are the spec features that add genuine value and justify the implementation cost.

6. **`include_bodies` toggle.** Add boolean to filter; when false, omit `postData.text` and `response.content.text`. Simplest new feature, high value for size reduction.
7. **`time_range` filter.** Add `StartTime`/`EndTime` to `NetworkBodyFilter`. Parse ISO 8601 in the tool handler. Requires `NetworkBody.Timestamp` to be parsed as `time.Time` for comparison.
8. **Header redaction for request headers.** Requires extension to capture and forward request headers (currently only `HasAuthHeader` boolean is stored). This is the most expensive change because it touches the extension.
9. **`pages` section.** Requires correlating network entries with page loads. The `PerformanceSnapshot` data has page URLs and timing. Implement by matching `NetworkBody.Timestamp` ranges to performance snapshot windows.
10. **Binary base64 encoding.** Check `BinaryFormat` field; if set, encode `ResponseBody` as base64 and set `content.encoding`.

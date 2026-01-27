# Review: Security Hardening Tools (tech-spec-security-hardening.md)

## Executive Summary

This is the most comprehensive and well-reasoned spec in the batch. The four tools (CSP Generator, Third-Party Audit, Security Regression Detection, SRI Hash Generator) are grounded in real attack scenarios, and the spec's own "Value Assessment" section is unusually honest about what is novel vs. consolidation. However, the spec significantly underestimates memory impact from bundled reputation lists, introduces an unbounded origin accumulator that violates the codebase's ring buffer discipline, and the external enrichment feature conflicts with the project's zero-dependency, zero-network-calls philosophy.

## Critical Issues (Must Fix Before Implementation)

### 1. Origin Accumulator is Unbounded Despite Claims Otherwise

**Section:** "Origin Accumulator"

The spec claims "This structure is bounded by nature -- apps rarely communicate with more than 50 unique origins" and estimates ~10KB. This is wrong in adversarial and even normal conditions.

The accumulator is keyed by `origin + directive_type`. An app with 20 origins and 8 resource types produces up to 160 entries. More critically, the `Pages` map inside each `OriginEntry` stores `map[string]bool` with page URLs -- these are unbounded. A developer navigating 500 unique URL paths (common in SPAs with dynamic routes like `/user/123`) would store 500 strings per origin entry.

The existing implementation in `csp.go` (lines 26-31) confirms this: `Pages map[string]bool` grows without limit.

**Fix:** Cap `Pages` at 10 entries (spec mentions "up to 10" for reporting but does not cap storage). Add a hard limit of 500 origin entries with LRU eviction, consistent with every other buffer in the codebase. Normalize dynamic URL segments before storing pages (reuse the route normalization from the SPA spec if co-implemented).

### 2. Bundled Reputation Lists Break the Zero-Dependency Rule

**Section:** "Bundled Lists (~380KB total, compiled into binary)"

The spec proposes compiling Disconnect.me, Tranco Top 10K, Curated CDN List, and Mozilla PSL as Go map literals into the binary. This has three problems:

- **License compliance:** Disconnect.me uses GPLv3. Embedding it in a proprietary binary may require the entire binary to be GPL-licensed. Tranco data has its own licensing terms. Neither license is analyzed in the spec.
- **Binary size:** 380KB of Go map literals will bloat to significantly more in compiled form due to Go's map initialization overhead. Empirically, a 380KB JSON list compiles to ~800KB-1.2MB of Go binary. This is a 20%+ increase on the current binary size.
- **Update lag:** Lists "ship at the version available when the Gasoline binary was built." The Disconnect.me list updates weekly. A user on a 3-month-old Gasoline binary has a 3-month-old tracker list. The spec acknowledges this but provides no mitigation beyond "update Gasoline."

**Fix:** Ship lists as separate JSON files in a `data/` directory alongside the binary. Load them lazily on first use. This preserves the zero-runtime-dependency guarantee (no network calls) while keeping binary size stable and making list updates independent of binary releases. Add a `gasoline update-lists` CLI command for offline list updates. Verify Disconnect.me GPLv3 compatibility before any embedding.

### 3. External Enrichment Feature is Scope Creep

**Section:** "Optional External Enrichment"

The RDAP, Certificate Transparency, and Google Safe Browsing integrations introduce:
- Network I/O from the server process (currently zero)
- Dependency on external service availability
- Privacy implications (domain names sent to third parties)
- Rate limiting complexity (1 req/s to RDAP, 5 concurrent max, 10s timeout)

This directly contradicts the project's architecture doc: "All data stays on localhost." The opt-in toggle does not change the architectural violation -- it means the server now has codepaths for outbound HTTP, which must be tested, secured (TLS verification, timeout handling, DNS resolution), and maintained.

**Fix:** Remove external enrichment from v1. The bundled lists provide sufficient classification for the stated use cases. If external enrichment is desired later, implement it as a separate CLI command (`gasoline enrich-audit --session-file`) that post-processes an exported audit result, keeping the server process network-free.

### 4. Auth Pattern Detection in diff_security Has High False-Positive Rate

**Section:** "Security Regression Detection" -- Auth comparison

The current implementation (`security_diff.go` lines 505-544) compares `HasAuthHeader` per endpoint. But `HasAuthHeader` is a property of the *request*, not the *endpoint*. If a user happens to browse a page while logged out in the "before" snapshot and logged in for the "after" snapshot, every endpoint will show an "auth_added" improvement -- which is noise, not signal.

Similarly, endpoints that use cookie-based auth (no Authorization header) will never show `HasAuthHeader = true`, making the auth comparison useless for cookie-authenticated apps.

**Fix:** Track auth at the endpoint level across multiple observations rather than from a single request. Require 2+ observations of the same endpoint with consistent auth state before including in the snapshot. Document that cookie-based auth is not detected (major limitation for most web apps).

### 5. Custom Lists File Path Allows Arbitrary File Read

**Section:** "Enterprise Custom Lists" -- `custom_lists_file` parameter

The `custom_lists_file` parameter accepts a file path from the MCP client. Since MCP tools run in the server process, this is an arbitrary file read vulnerability. An attacker who can send MCP messages (same localhost, but potentially a different process) can read any file the server process can access by pointing to `/etc/passwd` or similar -- the server will attempt to parse it as JSON and return parse errors that leak file contents.

**Fix:** Restrict `custom_lists_file` to paths within the project directory (`.gasoline/` or working directory). Validate the resolved path does not escape via `..` traversal. Alternatively, require the file to have a `.json` extension and validate the schema before returning any content-derived error messages.

## Recommendations (Should Consider)

### 1. CSP Generator Should Emit `frame-ancestors` Warning for Meta Tag

**Section:** "Limitations"

The spec notes `frame-ancestors` cannot be set via `<meta>` tag. The implementation generates a meta tag that includes `frame-ancestors 'none'` -- this silently fails in browsers. The meta tag output should strip `frame-ancestors` and append a warning.

### 2. Confidence Scoring Discrepancy Between Spec and Implementation

**Section:** "Confidence Scoring" vs. `csp.go` lines 310-342

The spec says "high" requires "5+ times across 2+ pages." The implementation uses 3+ times AND 2+ pages, with a comment noting the discrepancy. Pick one and update both. The implementation's threshold (3+) is more practical for development-time observation.

### 3. SRI Generator Body Size Limitation is a Deal-Breaker

**Section:** "Network Body Size Requirement"

`maxResponseBodySize` is 16KB (types.go line 341). Most CDN scripts (React at 42KB minified, lodash at 72KB, etc.) exceed this. The spec's proposed fix ("recommend temporarily increasing the capture limit") requires the user to change a config, restart, reload, and re-run -- effectively negating the "zero config" value proposition.

Better approach: For SRI specifically, make a one-time fetch of the resource URL from the server process (localhost to CDN, not from the browser) to get the full body for hashing. This is a read-only GET to a URL the app already loads -- minimal privacy impact.

### 4. PII Detection Needs Configurable Patterns

**Section:** "PII Field Detection in Outbound Data"

The spec says PII patterns are "reused from security_audit check 3" but does not define what those patterns are. Field names like `name`, `address`, and `user_id` will false-positive on legitimate non-PII fields (e.g., `product_name`, `shipping_address_type`, `user_id` as an opaque token). Allow custom PII patterns via the enterprise custom lists file.

### 5. The Spec's Own Recommendation to Demote SRI Should Be Followed

**Section:** "Value Assessment -- Tool 4"

The spec correctly identifies SRI as the weakest standalone tool with a shrinking use case. The `Vary: User-Agent` problem with Google Fonts CSS is particularly damaging since it is one of the most common SRI targets. Demote `generate_sri` to a sub-command of `audit_third_parties` output rather than a standalone MCP tool, reducing tool surface area.

## Implementation Roadmap

1. **CSP Generator hardening** (1-2 days): Cap `Pages` map per entry. Add hard limit to origin accumulator. Fix confidence scoring discrepancy. Add `frame-ancestors` meta tag stripping.

2. **Reputation list extraction** (1 day): Move bundled lists from compiled-in maps to JSON files loaded at startup. Verify Disconnect.me license. Add `--reputation-dir` flag.

3. **Third-Party Audit (bundled only)** (2-3 days): Implement risk classification, PII detection, enterprise custom lists with path validation. Skip external enrichment.

4. **Security Regression Detection** (1-2 days): Implement header/cookie/transport comparison (already done in `security_diff.go`). Fix auth pattern detection to require multiple observations. Add CORS comparison.

5. **SRI as sub-feature** (0.5 days): Already implemented in `sri.go`. Integrate into third-party audit output. Add warning for `Vary` headers.

6. **Enterprise custom lists file validation** (0.5 days): Path restriction, schema validation, expiry enforcement.

Total: ~7-9 days of implementation work. External enrichment deferred to a future release.

---
> **[MIGRATED: 2024-xx-xx | Source: /docs/specs/api-key-auth-review.md]**
> This file was migrated as part of the documentation reorganization. Please update links and references accordingly.
---

# API Key Authentication Spec Review

**Spec:** `docs/ai-first/tech-spec-api-key-auth.md` (Feature 20)
**Reviewer:** Principal Engineer Review
**Date:** 2026-01-26

---

## Executive Summary

The spec is well-structured and covers the core authentication flow with backward compatibility. However, it contains three critical issues that must be resolved before implementation: a broken constant-time guarantee in `validateKey`, a contradiction between the spec's claim to store only hashes and the code that stores plaintext keys in `map[string]struct{}`, and a missing CORS header update that will cause every authenticated extension request to fail at the preflight stage. The remaining recommendations are improvements that reduce implementation risk and improve operational safety.

---

## Critical Issues (Must Fix)

### C1. `validateKey` is NOT constant-time across variable-length keys

**Section:** Implementation Details > `validateKey`

`crypto/subtle.ConstantTimeCompare` is only constant-time when both byte slices have the **same length**. When lengths differ, it returns 0 immediately (Go source: `if len(x) != len(y) { return 0 }`). The spec's loop iterates over `cfg.Keys` (a `map`), so iteration order is randomized by Go's runtime, but the comparison itself leaks key length via timing.

If an attacker knows how many keys are configured (observable via the iteration count, which is fixed), they can measure response time to determine the length of each configured key. With 32-character keys this is a marginal risk, but the spec explicitly claims constant-time behavior and that claim is false.

**Fix:** Hash all keys at configuration time using HMAC-SHA256 with a random per-boot salt. Compare HMAC digests (fixed 32 bytes) instead of raw keys:

```go
type AuthConfig struct {
    keyDigests [][]byte // HMAC-SHA256 of each key
    hmacKey    []byte   // random 32-byte key, generated at startup
    // ...
}

func (cfg *AuthConfig) validateKey(provided string) bool {
    providedMAC := hmacDigest(cfg.hmacKey, provided)
    valid := false
    for _, digest := range cfg.keyDigests {
        if subtle.ConstantTimeCompare(digest, providedMAC) == 1 {
            valid = true
        }
    }
    return valid
}
```

This also fixes C2 below.

### C2. Spec contradicts itself on key storage

**Section:** Security Considerations > Key Exposure

The spec states: _"only the hash of configured keys is retained"_ (line 420). But the `AuthConfig` struct stores raw keys in `Keys map[string]struct{}` (line 220), and `validateKey` compares raw bytes (line 307). The implementation stores plaintext keys in memory for the entire process lifetime.

This is a correctness issue: if a core dump, heap profile, or `/debug/pprof` endpoint is ever exposed, all API keys are trivially extractable from the heap.

**Fix:** Adopt the HMAC approach from C1. The raw keys never need to persist past startup. Parse them, compute digests, zero the originals.

### C3. CORS `Access-Control-Allow-Headers` does not include `X-API-Key`

**Section:** Extension Configuration (implicit) / Existing code

The current CORS middleware at `cmd/dev-console/main.go:652` allows:
```
Access-Control-Allow-Headers: Content-Type, X-Gasoline-Key
```

The spec introduces `X-API-Key` as the primary header, but never mentions updating the CORS allowed headers. Chrome extensions using `fetch()` from a service worker may not require CORS preflight (depends on MV3 permissions), but the existing middleware explicitly handles OPTIONS. If the allowed headers list is not updated, any non-extension HTTP client (CI scripts, curl with preflight, other tools) sending `X-API-Key` will get a CORS rejection on preflight.

**Fix:** Update `corsMiddleware` to include `X-API-Key, Authorization` in `Access-Control-Allow-Headers`. Add this to the "Files to Change" table. Also update the `Access-Control-Allow-Headers` to include `X-Gasoline-Client` (already used for multi-client mode but absent from CORS headers).

---

## Recommendations (Should Consider)

### R1. Map iteration for key comparison leaks key count

**Section:** Implementation Details > `validateKey`

The loop `for key := range cfg.Keys` iterates over all keys, which makes the response time proportional to `len(cfg.Keys)`. An attacker can measure response time to determine how many valid keys exist. This is low severity (key count is not secret), but it defeats the constant-time intent.

**Fix:** When using the HMAC approach (C1), use a fixed-size slice rather than a map. If you want to support up to N keys, always iterate N times (pad with zero digests).

### R2. Audit log allocation on every request is unnecessary when auth is disabled

**Section:** Implementation Details > Middleware

The middleware creates an `AuthAttempt` struct on every request (line 265-270) even in the disabled path (`len(cfg.Keys) == 0` returns early at line 250). This is fine. But when auth IS enabled, the struct is allocated before knowing whether logging is even configured (`cfg.AuditLog` could be nil). The struct allocation itself is trivial (~120 bytes on stack), but `time.Now()` is a syscall on some platforms.

**Fix:** Move `time.Now()` and struct construction after the validation decision, or at minimum after confirming `cfg.AuditLog != nil`. The hot path for Gasoline is `POST /logs` at high frequency from the extension; adding a syscall per request is measurable.

### R3. `respondUnauthorized` message parameter is unused in the response body

**Section:** Implementation Details > `respondUnauthorized`

The function accepts a `message` parameter but the response body hardcodes `"Missing or invalid API key"`. The two call sites pass different messages (`"Missing API key"` vs `"Invalid API key"`), but these are never emitted. Either use the parameter or remove it.

**Fix:** This is also a security consideration. Distinguishing "missing" from "invalid" in responses helps attackers confirm that they are hitting a valid endpoint with a key (vs. no key). Best practice: use a single generic message for all 401 responses. Remove the parameter and hardcode one message.

### R4. Key file is read once at startup with no hot-reload path

**Section:** API Surface > Key File Format

The spec describes `--api-key-file=<path>` but the key file is loaded once at startup. For key rotation (Section: Key Rotation Procedure), the spec requires a server restart (steps 3 and 6). This is acceptable for the initial implementation but worth noting:

- The rotation procedure requires TWO restarts (add new key, then remove old key)
- During restart, there is a brief window where the extension cannot reach the server
- The spec should explicitly state that hot-reload is out of scope (for clarity, not as a requirement)

**Fix (documentation only):** Add a sentence: "Key file changes require a server restart. File watching for hot-reload is out of scope for this iteration."

### R5. `isLocalhost` does not handle IPv4-mapped IPv6 addresses

**Section:** Implementation Details > `isLocalhost`

The function checks `127.0.0.1`, `::1`, and `localhost`. It does not handle `::ffff:127.0.0.1` (IPv4-mapped IPv6), which is what Go's `net.Listen("tcp", ":7890")` can produce on dual-stack systems. The server currently binds `127.0.0.1` explicitly (not `::1`), so this is unlikely to surface today, but if the bind address ever changes, localhost exemption will silently fail.

The test `TestLocalhostExemption_IPv6` at line 604 already demonstrates this gap with its `[::1]:54321` test case, but the IPv6 format `::1:54321` (without brackets) at line 599 will fail to parse correctly with `net.SplitHostPort` because it looks like a bare IPv6 address, not host:port.

**Fix:** Use `net.ParseIP` after `SplitHostPort` and check `ip.IsLoopback()` instead of string comparison:

```go
func isLocalhost(addr string) bool {
    host, _, err := net.SplitHostPort(addr)
    if err != nil {
        host = addr
    }
    ip := net.ParseIP(host)
    return ip != nil && ip.IsLoopback()
}
```

### R6. Missing `malformed_header` failure reason in the flow

**Section:** Authentication Flow / Failure Reasons table

The failure reasons table (line 487) lists `malformed_header` as: "Authorization header present but not in Bearer format." However, the `extractAPIKey` function (line 104-117) simply returns empty string when `Authorization` is present but not `Bearer` prefixed. This is indistinguishable from "no header provided" in the middleware flow -- both result in `key == ""` and reason `no_key_provided`.

**Fix:** Either:
1. Remove `malformed_header` from the table (simpler, no code change), or
2. Return a third value from `extractAPIKeyWithSource` indicating a malformed header was detected, and use it for audit logging.

Option 1 is recommended. Distinguishing malformed headers adds complexity with minimal operational value.

### R7. `AuthConfig` uses value receiver inconsistently

**Section:** Implementation Details

`validateKey` uses a pointer receiver (`func (cfg *AuthConfig)`), but the middleware captures `cfg AuthConfig` by value in the closure. If `AuthConfig` is ever extended with mutable state (e.g., rate limiting, key reload), the value copy in the closure will silently miss updates.

**Fix:** Pass `*AuthConfig` to `APIKeyMiddleware`:

```go
func APIKeyMiddleware(cfg *AuthConfig) func(http.Handler) http.Handler {
```

### R8. Spec violates code-style.md rule on technical specifications

**Section:** Entire spec

Per `.claude/docs/code-style.md` lines 43-49: Tech specs should be "natural-language documents" with code snippets "only as brief illustrative examples (few lines max)." The spec contains full function implementations (50+ lines), complete test signatures, and struct definitions. This makes the spec brittle -- if the implementation diverges from the spec's code, the spec becomes misleading.

**Fix:** Replace inline implementations with behavior descriptions. Keep only the `extractAPIKey` snippet (6 lines) as illustrative. Move test code to the testing strategy section as pseudocode descriptions of what to test, not Go test function signatures.

### R9. Extension key storage in `chrome.storage.local` lacks access control

**Section:** Extension Configuration

The spec states the key is stored in `chrome.storage.local` and claims it is "encrypted at rest by Chrome." This is only partially true:
- Chrome encrypts the LevelDB storage on some platforms (Windows DPAPI, macOS Keychain-backed)
- On Linux, `chrome.storage.local` is NOT encrypted at rest
- Any extension with the same `chrome.storage.local` scope (which is per-extension) cannot access it, but a malicious script with file system access can read the LevelDB directly

This is acceptable for the threat model (localhost-only tool), but the spec should not claim encryption as a security property.

**Fix:** Change "encrypted at rest by Chrome" to "stored in Chrome's extension-local storage, which is sandboxed per-extension but may not be encrypted at rest on all platforms."

### R10. No minimum key length enforcement

**Section:** Key Generation and Storage

The spec recommends 32-character keys but does not enforce a minimum length. A user could configure `--api-key=a` and the server would accept it. This defeats the purpose of authentication.

**Fix:** Enforce a minimum key length at startup (e.g., 16 characters). Print a warning and exit if any configured key is shorter:

```
[gasoline] Error: API key must be at least 16 characters (got 1)
```

---

## Implementation Roadmap

The following order minimizes risk and enables incremental testing:

### Phase 1: Core middleware (auth.go replacement)

1. **HMAC key storage** -- Implement `AuthConfig` with HMAC digests, random per-boot salt, minimum key length validation.
2. **`extractAPIKeyWithSource`** -- Header extraction with `X-API-Key`, `Authorization: Bearer`, and backward-compat `X-Gasoline-Key`.
3. **`isLocalhost` with `net.ParseIP`** -- Robust loopback detection.
4. **`APIKeyMiddleware`** -- Wire the above together. Pass `*AuthConfig` (pointer).
5. **Tests** -- Port all existing `auth_test.go` tests. Add multi-key, localhost exemption, Bearer header, and both-headers-present cases. Follow TDD per project rules.

### Phase 2: Configuration plumbing

6. **CLI flags** -- `--api-key` (repeatable), `--api-key-file`, `--api-key-localhost-exempt`. Wire in `main.go`.
7. **Environment variable parsing** -- `GASOLINE_API_KEY` (comma-separated), `GASOLINE_API_KEY_FILE`, `GASOLINE_API_KEY_LOCALHOST_EXEMPT`.
8. **Key file loader** -- Read file, strip comments/blanks, validate minimum length, compute HMAC digests.
9. **CORS header update** -- Add `X-API-Key, Authorization` to allowed headers.

### Phase 3: Audit integration

10. **Audit logging** -- Integrate `AuthAttempt` with existing `AuditTrail`. Only allocate/log when `AuditLog` is non-nil.
11. **Query support** -- Ensure `get_audit_log` can filter by `type: "auth_attempt"`.

### Phase 4: Extension changes

12. **Options page** -- Add API Key field with show/hide toggle in `options.html` and `options.js`.
13. **Background.js** -- Read key from `chrome.storage.local`, include in `X-API-Key` header on all POST requests.
14. **401 handling** -- Detect 401 responses, surface in extension popup as connection error with actionable message.

### Phase 5: Validation

15. **Integration test** -- End-to-end: server with key, extension with key, verify data flows.
16. **Quality gates** -- `go vet`, `make test`, `node --test` all pass.
17. **Manual test** -- Run through the manual testing checklist in the spec.

---

## Summary of Changes Required to Spec

| Issue | Severity | Action |
|-------|----------|--------|
| C1. `validateKey` not constant-time for different-length keys | Critical | Rewrite to use HMAC digests |
| C2. Claims hash storage, implements plaintext storage | Critical | Align spec text with HMAC approach |
| C3. CORS headers missing `X-API-Key` | Critical | Add to CORS + Files to Change |
| R1. Key count leakage via iteration timing | Low | Fixed-size iteration or accept risk |
| R2. `time.Now()` on hot path when audit disabled | Low | Gate on AuditLog != nil |
| R3. Unused message parameter / info leak | Medium | Single generic 401 message |
| R4. No hot-reload documented | Low | Clarify in spec |
| R5. IPv4-mapped IPv6 not handled | Medium | Use `net.ParseIP().IsLoopback()` |
| R6. `malformed_header` reason unreachable | Low | Remove from table |
| R7. Value vs pointer receiver | Medium | Use pointer receiver |
| R8. Spec has too much inline code | Low | Reduce to behavior descriptions |
| R9. Overstates Chrome storage encryption | Low | Correct the claim |
| R10. No minimum key length | Medium | Enforce 16-char minimum at startup |

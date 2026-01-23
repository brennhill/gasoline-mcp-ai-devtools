# Accessibility Audit Result Caching

## Status: Specification

---

## Purpose

The `run_accessibility_audit` MCP tool currently re-runs axe-core on the page every time it's called, even if the page hasn't changed. A full audit takes 500ms-3s depending on page complexity. AI agents frequently call the audit multiple times in quick succession — once to see violations, then again after each fix to verify. Caching avoids re-running expensive audits when the page state hasn't changed.

---

## How It Works

The server maintains a small in-memory cache of recent audit results. When `run_accessibility_audit` is called, the server checks whether a cached result exists for the same parameters. If a valid (non-expired) cache entry exists, the server returns it immediately without asking the extension to re-run axe-core.

### Cache Key

The cache key is derived from the audit parameters:
- The `scope` parameter (CSS selector string, or empty for full-page)
- The `tags` parameter (sorted alphabetically, then joined — so `["wcag2aa", "wcag2a"]` and `["wcag2a", "wcag2aa"]` produce the same key)

Two calls with the same scope and tags (regardless of tag order) will share a cache entry.

### TTL (Time-To-Live)

Cached results expire after **30 seconds**. After expiry, the next call triggers a fresh audit.

### Cache Invalidation

The cache is cleared entirely (all entries) when:
1. The server receives a log entry or event whose URL differs from the URL stored in the most recent cache entry (heuristic for page navigation)
2. The `force_refresh: true` parameter is passed (clears and re-runs)
3. The server is restarted (cache is in-memory only)

### Cache Size

Maximum **10 entries** per server instance. If a new entry would exceed the limit, the oldest entry (by creation time) is evicted. In practice, most sessions will have 1-3 entries (full-page audit, plus 1-2 scoped audits).

---

## Tool Interface Changes

The existing `run_accessibility_audit` tool gains one new optional parameter:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `force_refresh` | boolean | `false` | If true, bypass cache and re-run the audit |

The response format is unchanged. When a cached result is returned, no additional metadata is added — the AI doesn't need to know whether the result was cached or fresh.

---

## Behavior

### Happy Path (Cache Miss)

1. AI calls `run_accessibility_audit(scope: "#main", tags: ["wcag2aa"])`
2. Server computes cache key from params: `"#main|wcag2aa"`
3. No cache entry exists (or entry is expired)
4. Server creates a pending query, extension runs axe-core
5. Result arrives; server stores it in cache with current timestamp and the page URL from the most recent log entry
6. Server returns result to AI

### Happy Path (Cache Hit)

1. AI calls `run_accessibility_audit(scope: "#main", tags: ["wcag2aa"])` (same params, within 30s)
2. Server computes cache key: `"#main|wcag2aa"`
3. Cache entry exists and is not expired
4. Server returns cached result immediately (no extension round-trip)

### Force Refresh

1. AI calls `run_accessibility_audit(scope: "#main", force_refresh: true)`
2. Server removes the matching cache entry (if any)
3. Proceeds as cache miss — full extension round-trip
4. New result stored in cache

### Page Navigation Invalidation

1. AI runs audit on `https://myapp.com/dashboard` — result cached
2. User navigates to `https://myapp.com/settings`
3. Server receives a log entry (console log, network error, etc.) with the new URL
4. Server detects URL change, clears entire cache
5. Next audit call proceeds as cache miss

### Concurrent Calls

If two MCP calls arrive simultaneously for the same cache key (both are cache misses), only one pending query is created. The second call waits for the same result. Both receive the same response when it arrives. This prevents duplicate axe-core runs.

---

## Edge Cases

| Case | Handling |
|------|---------|
| Empty scope + empty tags | Valid cache key: `"|"` (full page, all rules) |
| Tags in different order | Normalized by sorting before key generation |
| Audit times out | Timeout error is NOT cached; next call retries |
| Extension disconnects mid-audit | Error is NOT cached; next call retries |
| Page has SPA navigation (URL fragment change) | Not detected unless a new log entry arrives with different URL |
| `force_refresh` with scope that wasn't cached | No-op on cache; proceeds as normal miss |
| Server receives events but no URL in them | Cache is not invalidated (URL comparison only triggers on URL presence) |

---

## Performance

| Metric | Without caching | With caching | Measurement |
|--------|----------------|--------------|-------------|
| Repeated audit (same params, <30s) | 500ms-3s | <1ms | Server response time |
| Extension CPU (repeated calls) | Full axe-core run each time | Zero (no request sent) | Extension CPU usage |
| Memory overhead | N/A | ~50-500KB (10 cached results) | Server RSS delta |

---

## Implementation Notes

The cache lives in the `V4Server` struct alongside existing state (query results, WebSocket connections, etc.). It uses a `sync.RWMutex` for thread safety — reads take a read lock, writes (insert/evict/clear) take a write lock.

The cache entry stores:
- The raw result bytes (as returned by the extension)
- The creation timestamp
- The page URL at time of capture (for navigation detection)

The URL for navigation detection comes from the most recent log entry's URL field (already tracked by the server for other purposes). If no URL is available, navigation detection is skipped.

---

## Test Scenarios

1. **Cache miss**: First call with given params triggers extension round-trip, result returned
2. **Cache hit**: Second call with same params within 30s returns immediately, no pending query created
3. **TTL expiry**: Call after 30s triggers fresh audit
4. **Tag normalization**: `["b", "a"]` and `["a", "b"]` share cache entry
5. **Force refresh**: Bypasses cache, creates pending query, updates cache with new result
6. **Navigation invalidation**: Cache cleared when URL changes
7. **Error not cached**: Timeout/error response not stored; retry works
8. **Concurrent dedup**: Two simultaneous calls for same key produce one pending query
9. **Max entries**: 11th unique entry evicts the oldest
10. **Different scope same tags**: Different cache entries for `"#main"` vs `"#footer"`

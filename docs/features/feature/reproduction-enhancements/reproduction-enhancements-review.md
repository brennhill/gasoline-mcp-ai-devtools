---
status: shipped
scope: feature/reproduction-enhancements/review
ai-priority: high
tags: [review, issues]
relates-to: [tech-spec.md, product-spec.md]
last-verified: 2026-01-31
---

# Review: Reproduction Script Enhancements

## Executive Summary

This is the most ambitious of the four specs and the one most likely to ship an incomplete or leaky abstraction. It proposes four distinct capabilities -- screenshot capture, visual assertions, data fixture generation, and bug report generation -- each of which touches both the extension and the server. The existing `reproduction.go` and `codegen.go` implementations cover the basic script/fixture/screenshot flow, but the spec's screenshot storage model (20 PNGs in server memory, up to 10MB) directly conflicts with the project's memory management philosophy. The fixture generation logic is naive and will produce broken tests for any non-trivial API.

## Critical Issues

### 1. Screenshot Storage Blows the Memory Budget

The architecture doc (`architecture.md`) defines a total memory budget: 4MB for WebSocket events, 8MB for network bodies, 2MB for connections, 1KB for queries. The spec proposes adding 10MB for screenshots (20 x 500KB, spec line 48-49). This nearly doubles the server's memory footprint and there is no eviction coordination with the existing memory enforcement system.

The rate limiting circuit breaker opens at 50MB total buffer memory (`memoryHardLimit` in `types.go` line 345). Adding 10MB of screenshot data makes it easier to hit this limit, especially since screenshots are large binary blobs that cannot be incrementally evicted the way ring buffer entries can.

**Fix**: Either (a) reduce the screenshot budget to 5 screenshots at 200KB each (1MB total), which is still useful for bug reproduction, or (b) store screenshots on disk in a temp directory and keep only metadata in memory. Option (b) aligns with the spec's mention of `output_path` for SARIF and keeps the server's memory profile predictable.

Integrate screenshot memory into `calcTotalMemory()` so the circuit breaker accounts for it.

### 2. `chrome.debugger` Attachment is Intrusive and Fragile

The spec proposes using `chrome.debugger.sendCommand` for `Page.captureScreenshot` (lines 193-207). This triggers the "Chrome is being controlled by automated test software" infobar, which:
- Confuses users who did not expect it
- Persists across page reloads until the debugger detaches
- Can break sites that detect automated environments (bot detection)
- Requires an explicit user gesture to dismiss on some Chrome versions

The spec acknowledges this (line 208) and proposes `chrome.tabs.captureVisibleTab` as an alternative. The fallback is the right default, but the spec still describes debugger mode as the primary approach in the "How It Works" section.

**Fix**: Invert the hierarchy. Make `chrome.tabs.captureVisibleTab` the primary and only default. Do not implement `chrome.debugger` attachment in the initial version. Full-page and element-specific captures are nice-to-have features that can be added later behind an explicit user opt-in (similar to the existing AI Web Pilot toggle). This avoids shipping a feature that triggers alarming browser UI by default.

### 3. Fixture Generation Produces Untestable Output

The existing `generateFixtures` in `reproduction.go` (lines 193-228) iterates over network bodies and maps each JSON API response to its URL path. The spec extends this with sanitization, array trimming, and auth-aware setup (lines 96-105).

Problems with the current approach:
- **No request ordering**: Fixtures are emitted in buffer order, but APIs often have dependencies (create user, then create user's items). The `beforeAll` block in the spec (lines 84-95) shows sequential calls, but nothing in the generation logic preserves or detects ordering.
- **No idempotency**: If the fixture creates a user with email `test@example.com` and the test runs twice, the second run fails on a unique constraint. The spec mentions "test-safe values" (line 105) but provides no mechanism for generating unique values.
- **No cleanup**: There is no `afterAll` block to delete created data. In CI, test pollution across runs causes flaky tests.
- **Response body != request body**: The fixture logic uses response bodies to determine what data to create, but responses often contain computed fields (IDs, timestamps, computed totals) that cannot be sent as request bodies.

**Fix**: For the initial version, generate fixtures as `page.route()` mocks (which the implementation already does in `reproduction.go` lines 77-85) rather than as real API calls in `beforeAll`. This is safer, portable, and does not require understanding API semantics. The `beforeAll` approach with real API calls should be a separate, later feature that requires explicit opt-in and integrates with the user's actual test data setup strategy.

### 4. Spec Scope Creep: Four Features in One Spec

This spec defines: (1) screenshot capture infrastructure, (2) visual assertions in generated tests, (3) data fixture generation, (4) a bug report generator. Each is independently valuable and independently complex. Bundling them creates implementation risk:
- A delay in screenshot infrastructure blocks visual assertions
- Fixture generation has completely different testing requirements than screenshot capture
- The bug report generator (`generate_bug_report` tool) is a new MCP tool that needs its own schema, tests, and error handling

**Fix**: Split into two specs. Spec A: screenshot capture + visual assertions (extension changes + codegen changes). Spec B: fixture generation + bug report generation (server-only, no extension changes). Ship A first since it enables the highest-value use case (visual regression testing). Ship B separately.

## Recommendations

### A. Screenshot Trigger Strategy Needs Rate Limiting

The spec proposes auto-capture on navigation, actions, and errors (lines 38-43). With `on_action: true`, a form with 10 input fields generates 10 screenshots from typing alone. At 500KB each, that is 5MB from a single form fill.

Add a cooldown: no more than 1 screenshot per second when `on_action` is enabled. Also consider only capturing on "significant" actions (clicks, form submissions) and not on every keystroke or scroll.

### B. `maxDiffPixels: 100` Default is Too Loose

The spec suggests `maxDiffPixels: 100` for visual assertions (line 64). On a 1280x720 viewport, 100 pixels is 0.01% of the image -- this is actually very strict, not "conservative" as the spec claims. A single anti-aliased text render can differ by more than 100 pixels across runs. Playwright's default is `maxDiffPixelRatio: 0.01` (1%), which is much more forgiving.

Use `maxDiffPixelRatio` instead of `maxDiffPixels` to scale with viewport size. Default to `0.01`.

### C. Thumbnail Generation Should Not Happen Server-Side

The spec says the server generates 320px thumbnails (line 49). The Go server has no image processing dependencies (zero deps rule). Resizing a PNG in pure Go stdlib is possible but slow (~50ms per image using `image/png` + `image/draw`). Since thumbnails are only used for markdown embedding, consider generating them client-side in the extension (using Canvas API) before sending to the server, or skip thumbnails entirely and use the full image with CSS sizing in markdown.

### D. GraphQL Fixture Handling is Underspecified

The spec mentions "GraphQL requests: fixture generation includes the query/mutation string" (line 233). But GraphQL requests are POST to a single endpoint (`/graphql`). The current path-based fixture keying (`fixtures[path]`) means all GraphQL responses map to the same key. This needs operation-name-based keying for GraphQL endpoints.

### E. The `generate_bug_report` Tool Needs Scope Constraints

The spec describes a new MCP tool (lines 125-159) that generates Markdown with base64-embedded images. A bug report with 3 screenshots at 500KB each produces a 2MB+ Markdown string. This is large for an MCP tool response and will consume significant tokens. Add a parameter for `max_screenshots` (default 3) and consider referencing screenshot IDs instead of embedding base64 inline.

## Implementation Roadmap

1. **Implement `chrome.tabs.captureVisibleTab` screenshot capture** in the extension. This is the safe, permission-friendly path. Store screenshots in a server-side ring buffer with strict memory limits (5 screenshots, 200KB each, 1MB total). Integrate with `calcTotalMemory()`.

2. **Add `include_screenshots` to `get_reproduction_script`** -- This is already partially implemented in `reproduction.go`. Verify the screenshot path insertion logic produces valid Playwright scripts. Add tests.

3. **Add `assert_visual` to `generate_test`** using `maxDiffPixelRatio: 0.01`. This is a codegen change only (no extension work). Add the `mask_dynamic` parameter for masking timestamps.

4. **Add `capture_screenshot` MCP tool** with the extension query pipeline (same pattern as `query_dom`). Test the full round-trip: MCP call -> pending query -> extension captures -> server returns base64.

5. **Defer `generate_bug_report`** to a follow-up. It depends on screenshots being reliable and is a new tool with its own failure modes.

6. **Defer `beforeAll` fixture generation** (real API calls) to a follow-up. The existing `page.route()` fixture approach is sufficient for the initial release.

7. **Add screenshot rate limiting**: 1 screenshot per second cooldown, max 5 in buffer.

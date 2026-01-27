# UAT: Schema Improvements & Accessibility Feature

**Version:** 5.1.0
**Date:** 2026-01-27
**Scope:** Test optimized MCP schemas and completed a11y feature

## Pre-UAT Setup

### 1. Build and Run Server
```bash
make dev
./dist/gasoline
```

### 2. Load Extension
- Open Chrome and navigate to `chrome://extensions/`
- Enable Developer mode
- Load unpacked extension from `extension/` directory
- Verify extension icon shows connected status

### 3. Enable Features
- Click extension icon
- Ensure all capture modes enabled:
  - Console Logs: ON
  - Network Bodies: ON
  - WebSocket: ON
  - Enhanced Actions: ON
  - AI Web Pilot: ON

---

## Test 1: network_waterfall Schema

**What Changed:**
- Added unit suffixes: `durationMs`, `startTimeMs`, `fetchStartMs`, `responseEndMs`
- Added size suffixes: `transferSizeBytes`, `decodedBodySizeBytes`, `encodedBodySizeBytes`
- Added `compressionRatio` computed field
- Renamed `timestamp` → `capturedAt`
- Added metadata: `oldestTimestamp`, `newestTimestamp`

**Test Steps:**
1. Navigate to a page with network activity (e.g., https://example.com)
2. Call MCP tool: `observe({what: "network_waterfall"})`
3. Verify response includes:
   - ✅ All timing fields have `Ms` suffix
   - ✅ All size fields have `Bytes` suffix
   - ✅ `compressionRatio` present for compressed resources
   - ✅ `capturedAt` in RFC3339 format
   - ✅ `oldestTimestamp` and `newestTimestamp` in metadata
   - ✅ `cached` boolean correct (true for cached resources)

4. Test URL filtering: `observe({what: "network_waterfall", url: "example.com"})`
   - ✅ Only matching URLs returned

5. Test limit parameter: `observe({what: "network_waterfall", limit: 5})`
   - ✅ Returns last 5 entries only

**Expected Limitations Message:**
```
- No HTTP status codes (use network_bodies for 404s/500s/401s)
- No request methods (GET/POST/etc.)
- No request/response headers or bodies
```

---

## Test 2: network_bodies Schema

**What Changed:**
- Renamed `requests` → `networkRequestResponsePairs`
- Added unit suffixes: `durationMs`, `capturedAt`
- Added size fields: `requestBodySizeBytes`, `responseBodySizeBytes`
- Added metadata: `maxRequestBodyBytes` (8KB), `maxResponseBodyBytes` (16KB)
- Added `binaryFormatInterpretation` (high/medium/low_confidence)
- Renamed truncation flags: `requestBodyTruncated`, `responseBodyTruncated`

**Test Steps:**
1. Navigate to page with POST requests (e.g., login form)
2. Trigger POST request
3. Call MCP tool: `observe({what: "network_bodies"})`
4. Verify response includes:
   - ✅ Top-level array named `networkRequestResponsePairs` (not `requests`)
   - ✅ `durationMs` field present
   - ✅ `capturedAt` timestamp in RFC3339 format
   - ✅ `requestBodySizeBytes` and `responseBodySizeBytes` present when bodies exist
   - ✅ `maxRequestBodyBytes: 8192` in metadata
   - ✅ `maxResponseBodyBytes: 16384` in metadata
   - ✅ `requestBodyTruncated` and `responseBodyTruncated` flags when applicable
   - ✅ `binaryFormatInterpretation` when binary data detected

5. Test status filtering: `observe({what: "network_bodies", status_min: 400})`
   - ✅ Only 4xx/5xx responses returned

6. Test with binary response (e.g., image):
   - ✅ `binaryFormat` detected (e.g., "PNG", "JPEG")
   - ✅ `binaryFormatInterpretation` shows confidence level

---

## Test 3: query_dom Schema

**What Changed:**
- Added context: `url`, `pageTitle`, `selector` echo
- Distinguish counts: `totalMatchCount`, `returnedMatchCount`
- Added metadata: `maxElementsReturned`, `maxDepthQueried`, `maxTextLength`
- Added `textTruncated` boolean to match objects
- Renamed `boundingBox` → `bboxPixels`
- Added helpful hint when no matches found

**Test Steps:**
1. Navigate to any webpage
2. Call MCP tool: `configure({action: "query_dom", selector: "h1"})`
3. Verify response includes:
   - ✅ `url` field with current page URL
   - ✅ `pageTitle` field with page title
   - ✅ `selector` field echoing "h1"
   - ✅ `totalMatchCount` (total matches on page)
   - ✅ `returnedMatchCount` (matches returned, may be capped at 50)
   - ✅ `maxElementsReturned: 50` in metadata
   - ✅ `maxDepthQueried: 5` in metadata
   - ✅ `maxTextLength: 500` in metadata

4. Check match objects:
   - ✅ `bboxPixels` field (not `boundingBox`)
   - ✅ `textTruncated: true` when text >= 500 chars
   - ✅ `textTruncated: false` when text < 500 chars

5. Test with non-existent selector: `configure({action: "query_dom", selector: ".does-not-exist"})`
   - ✅ `hint` field present with helpful message
   - ✅ Suggests trying broader selector

---

## Test 4: validate_api Schema

**What Changed:**
- Added timestamps: `analyzedAt`, `dataWindowStartedAt`, `firstSeenAt`, `lastSeenAt`
- Added `appliedFilter` echo
- Added `summary` object with counts
- Added to violations: `violationType`, `severity`, `affectedCallCount`
- Added `possibleViolationTypes` metadata
- Renamed `lastCalled` → `lastCalledAt`, added `firstCalledAt`
- Added `consistencyScore` (0-1) and `consistencyLevels` explanation

**Test Steps:**
1. Navigate to page making API calls
2. Call MCP tool: `configure({action: "validate_api", api_action: "analyze"})`
3. Verify response includes:
   - ✅ `analyzedAt` timestamp in RFC3339 format
   - ✅ `dataWindowStartedAt` timestamp
   - ✅ `summary` object with `violations`, `endpoints`, `totalRequests`, `cleanEndpoints`
   - ✅ `possibleViolationTypes` array in metadata

4. If violations found, check violation objects:
   - ✅ `violationType` matches `type`
   - ✅ `severity` present (critical/high/medium/low)
   - ✅ `affectedCallCount` >= 1
   - ✅ `firstSeenAt` and `lastSeenAt` timestamps

5. Call report action: `configure({action: "validate_api", api_action: "report"})`
   - ✅ Endpoint objects have `lastCalledAt` (not `lastCalled`)
   - ✅ `firstCalledAt` present
   - ✅ `consistencyScore` between 0 and 1
   - ✅ `consistencyLevels` metadata explains score ranges

6. Test with URL filter: `configure({action: "validate_api", api_action: "analyze", url: "/api/users"})`
   - ✅ `appliedFilter` object echoes the url parameter

---

## Test 5: Accessibility (a11y) Feature

**What's New:**
- Complete message passing chain: background.js → content.js → inject.js
- Axe-core integration for accessibility audits
- Server-side handler processes and caches results

**Test Steps:**
1. Navigate to page with accessibility violations (e.g., page without alt text on images)
2. Call MCP tool: `configure({action: "accessibility"})`
3. Verify response includes:
   - ✅ `violations` array with issues found
   - ✅ Each violation has:
     - `id` (axe rule ID)
     - `impact` (critical/serious/moderate/minor)
     - `description` (what the issue is)
     - `helpUrl` (link to axe documentation)
     - `wcag` array (WCAG tags like "wcag2a", "wcag111")
     - `nodes` array with affected elements
   - ✅ `summary` object with counts (violations, passes, incomplete, inapplicable)

4. Test with scope parameter: `configure({action: "accessibility", scope: "#main"})`
   - ✅ Audit limited to specified selector

5. Test with WCAG tags: `configure({action: "accessibility", tags: ["wcag2a"]})`
   - ✅ Only runs WCAG 2.0 Level A rules

6. Verify caching:
   - Run same query twice
   - ✅ Second call should be faster (cached)

---

## Test 6: Extension Features

### AI Web Pilot Toggle
1. Click extension icon
2. Toggle "AI Web Pilot" off
3. Try `interact({action: "execute_js", script: "1+1"})`
   - ✅ Should return error: `ai_web_pilot_disabled`

4. Toggle "AI Web Pilot" back on
5. Try same command
   - ✅ Should execute successfully

### New Tab Action
1. Call: `interact({action: "new_tab", url: "https://example.com"})`
   - ✅ New tab opens with URL
   - ✅ Tab gets focus

2. Try restricted URL: `interact({action: "new_tab", url: "chrome://extensions"})`
   - ✅ Should be blocked with error

---

## Success Criteria

All schema improvements must:
- ✅ Return data in expected format
- ✅ Include all new metadata fields
- ✅ Use correct field naming (unit suffixes, renamed fields)
- ✅ Show helpful hints when appropriate
- ✅ Work correctly with filter/limit parameters

Accessibility feature must:
- ✅ Successfully run axe-core audits
- ✅ Return properly formatted violation data
- ✅ Support scope and tag filtering
- ✅ Cache results appropriately

---

## Reporting Issues

If any test fails:
1. Note the specific test step
2. Capture the actual vs expected output
3. Check browser console for errors
4. Check server logs for errors
5. Document in issue tracker with `UAT-FAILURE` label

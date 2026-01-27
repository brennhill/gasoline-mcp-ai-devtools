# Gasoline Roadmap

## In Progress

### GitHub Pages Documentation Site
- **Branch:** `feature/github-pages-site`
- **Summary:** SEO-optimized documentation site for cookwithgasoline.com using Jekyll minimal-mistakes theme. Splits monolithic README into targeted pages for search discoverability.

## Immediate - Feature Competitive (BrowserTools MCP Parity)

### SEO Audit Tool
- **Tool:** `generate {type: "seo_audit"}`
- **Capabilities:**
  - Metadata validation (title, description, og tags, canonical)
  - Heading structure (H1-H6 hierarchy)
  - Link structure (internal/external, broken links, nofollow)
  - Image optimization (alt text, dimensions, format)
  - Structured data validation (JSON-LD, schema.org)
  - Mobile-friendliness indicators
- **Effort:** ~300 lines Go + 150 lines extension JS

### Performance Audit Tool (Comprehensive Lighthouse-Style)
- **Tool:** `generate {type: "performance_audit"}`
- **Capabilities:**
  - Render-blocking resource detection (CSS/JS in `<head>`)
  - DOM size analysis (total nodes, depth, child count)
  - Image optimization (uncompressed, wrong format, missing dimensions)
  - JavaScript bundle analysis (size, unused code estimation)
  - CSS analysis (unused rules, specificity issues)
  - Third-party script impact
  - Cache policy effectiveness
  - Resource compression detection
- **Effort:** ~400 lines Go + 200 lines extension JS
- **Note:** Extends existing performance metrics (FCP, LCP, CLS) to comprehensive analysis

### Best Practices Audit Tool
- **Tool:** `generate {type: "best_practices_audit"}`
- **Capabilities:**
  - HTTPS usage verification
  - Deprecated API usage detection (via console warnings)
  - Browser error log analysis
  - Security headers validation (CSP, HSTS, X-Frame-Options)
  - Document metadata completeness
  - JavaScript error rate
  - Console noise level
  - Mixed content detection
- **Effort:** ~250 lines Go + 100 lines extension JS

### Enhanced WCAG Accessibility Audit
- **Tool:** Enhance `observe {what: "accessibility"}` with `wcag_level` and `detailed: true` options
- **Capabilities (beyond current axe-core):**
  - Color contrast analysis (all text/background combinations)
  - Keyboard navigation trap detection (tab order simulation)
  - ARIA attribute validation (role hierarchy, required attributes)
  - Form label completeness
  - Heading hierarchy validation
  - Skip link detection
  - Focus indicator visibility
  - Screen reader text sufficiency
- **Effort:** ~200 lines extension JS (enhanced axe-core configuration + post-processing)

### Auto-Paste Screenshots to IDE
- **Tool:** Add `include_screenshots: true` option to `observe {what: "errors"}` and other relevant tools
- **Capabilities:**
  - Encode screenshot as base64 in MCP responses
  - Add "copy to clipboard" button in extension popup
  - Automatic attachment to error observations
- **Effort:** ~100 lines Go + 50 lines extension JS

### Annotated Screenshots
- **Tool:** `observe {what: "page", annotate_screenshot: true}`
- **Capabilities:**
  - Overlay element labels, bounding boxes, interaction hints on screenshots
  - Help AI vision models understand clickable/visible elements
  - Interactive element highlighting with selector hints
  - ARIA role and label annotations
- **Effort:** ~150 lines extension JS (canvas overlay)

### Form Filling Automation (High-Level API)
- **Tool:** `interact {action: "fill_form", selector: "form", fields: {...}}`
- **Capabilities:**
  - Auto-detect input types (text, email, password, checkbox, radio, select)
  - Handle common validation patterns (email format, required fields)
  - Trigger appropriate events (input, change, blur) for framework reactivity
  - Support for nested field paths (e.g., `fields: {"user.email": "..."}`)
  - Automatic wait for form elements to be interactive
  - Return validation errors if form submission fails
- **Rationale:** More ergonomic than `execute_js` for common form automation tasks
- **Effort:** ~200 lines extension JS

### E2E Testing Integration (CI/CD)
- **Tool:** `generate {type: "playwright_fixture"}` and CI integration guide
- **Capabilities:**
  - Export Gasoline state as Playwright fixtures
  - Attach browser state (console, network, WebSocket) to test failures automatically
  - Script injection via Playwright's `addInitScript()` (no extension needed in CI)
  - Extension loading support for headed CI runs (`--load-extension`)
  - GitHub Actions / GitLab CI examples
  - Test failure enrichment (network bodies, WS messages, DOM state on failure)
- **Rationale:** 26% of developer time goes to CI failure investigation. Browser-level observability in CI eliminates "pull branch, reproduce locally, fail to reproduce" loops.
- **Effort:** ~300 lines Go + 200 lines CI integration + documentation
- **Economic impact:** $30-60K/year per 10-person team in recovered engineering time

**Total estimated effort:** ~2600 lines of code

---

## Planned

### CPU/Network Emulation (Chrome DevTools MCP Feature)
- **Tool:** `configure {action: "emulate", cpu_rate: 4, network: "Slow 3G"}`
- **Capabilities:**
  - Throttle CPU (1x-6x slowdown) to simulate low-end devices
  - Network profiles: Slow 3G, Fast 3G, 4G, Offline, No throttling
  - Test performance on low-end hardware
  - Verify graceful degradation under poor network
  - Reproduce mobile-specific performance issues
  - Test offline functionality
- **Implementation:** Chrome DevTools Protocol `Emulation` domain
- **Rationale:** Complements existing performance monitoring with device/network simulation
- **Effort:** ~150 lines Go (CDP integration) + 50 lines extension JS
- **Priority:** MEDIUM - Nice-to-have for comprehensive performance testing

### Dialog Handling (Chrome DevTools MCP Feature)
- **Tool:** `interact {action: "handle_dialog", accept: true, prompt_text: "..."}`
- **Capabilities:**
  - Handle alert, confirm, prompt, beforeunload dialogs
  - Auto-respond or queue for AI decision
  - Prevent blocking during automation workflows
- **Implementation:** Intercept dialog events via Chrome APIs
- **Rationale:** Currently dialogs block interactions; this enables smooth automation
- **Effort:** ~100 lines extension JS
- **Priority:** MEDIUM - Good addition to AI Web Pilot

### Drag & Drop Automation (Chrome DevTools MCP Feature)
- **Tool:** `interact {action: "drag", from_selector: "...", to_selector: "..."}`
- **Capabilities:**
  - Programmatic drag-and-drop interactions
  - HTML5 drag API + legacy mouse events
  - Common UI pattern support (kanban boards, file uploads, reordering)
- **Implementation:** Synthesize drag events (dragstart, dragover, drop)
- **Rationale:** Common UI pattern in modern apps; currently requires `execute_js` workarounds
- **Effort:** ~150 lines extension JS
- **Priority:** MEDIUM - Nice-to-have for comprehensive automation

### A11y Tree Snapshots (Chrome DevTools MCP Feature)
- **Tool:** `observe {what: "a11y_tree"}`
- **Capabilities:**
  - Export accessibility tree as text with unique element IDs
  - Semantic representation (roles, labels) vs structural DOM
  - More token-efficient than full HTML for LLMs
  - Allow actions by uid reference (e.g., "click button uid=123")
- **Implementation:** Chrome Accessibility API traversal + text serialization
- **Rationale:** AI-first page representation; complements existing DOM queries and annotated screenshots
- **Effort:** ~200 lines extension JS
- **Priority:** LOW-MEDIUM - Interesting but not critical; existing features may cover use case

### Local Web Scraping & Automation (LLM-Controlled)
- **Tool:** `interact {action: "scrape"}` with multi-step automation capabilities
- **Capabilities:**
  - Use Gasoline to control browser as logged-in user (leverage existing sessions/cookies)
  - Navigate, interact, extract structured data from pages
  - Multi-step workflows (login → navigate → paginate → extract)
  - Extract data from authenticated/paywalled content
  - Forward extracted data to external destinations (files, APIs, databases)
  - Maintain privacy: localhost-only, no cloud service dependency
  - Session replay: Record workflows, replay for repeated scraping
- **Use cases:**
  - Scrape personal data from services you're logged into (banking, social media exports)
  - Extract structured data from internal company tools
  - Automate data entry across multiple systems
  - Pull reports from authenticated dashboards
  - Monitor competitor sites with login-required access
- **Differentiation from cloud scrapers (Hyperbrowser):**
  - Uses YOUR browser with YOUR logged-in sessions
  - Localhost-only, no data sent to third parties
  - LLM-controlled via MCP (Claude, Cursor, etc.)
  - Integrated with existing Gasoline telemetry (errors, network, WS)
- **Architecture:**
  - Extends existing `interact` tool with scraping actions
  - Builds on `execute_js`, `navigate`, `query_dom` primitives
  - Add workflow recording/replay mechanism
  - Structured data extraction with schema hints
- **Effort:** ~500 lines Go + 300 lines extension JS + workflow engine
- **Status:** Longer-term, after core audit features ship

### Capture Profiles
- Configurable capture modes (minimal, standard, verbose)
- Per-site profile overrides

### Extension Health Metrics via MCP
- Expose extension internal state (buffer sizes, circuit breaker status)
- MCP tool for AI assistants to check extension health

---

## Completed

### Engineering Resilience (CI/CD & Quality Gates)
- Race detection (`-race` flag in CI)
- Zero-dependency verification (go.mod/go.sum checks)
- Stdlib-only import verification
- Binary size gate (15MB max)
- Test coverage gate (60% minimum, currently ~77%)
- E2E test suite in CI (Playwright + xvfb-run)
- Goroutine leak detection (TestMain wrapper)
- Fuzz testing (POST /logs, MCP requests, screenshot endpoint)
- Performance benchmarks (entry ingestion, log rotation, MCP tools, HTTP)
- Golden file / snapshot tests (MCP initialize, tools/list, get_browser_errors)
- Typed MCP response structs (replaces map[string]interface{})
- Contract validation (level allowlist, 1MB entry size limit)
- Lefthook pre-commit (lint, format, typecheck, vet) and pre-push (Go + JS tests)
- Security scanning (gosec + eslint security rules)

---

### v3 — Baseline Capture

#### Console Logging
- All console.* API calls captured (error, warn, log, info, debug)
- Full object/array serialization with safe circular reference handling
- Argument truncation at 10KB per arg
- Level-based filtering (errors only, warnings+, all)

#### Network Error Capture
- Failed API calls (4xx and 5xx status codes)
- HTTP method, URL, status code, response body
- Request duration tracking
- Header sanitization (removes auth, cookies, tokens)

#### Exception Tracking
- Uncaught errors and unhandled promise rejections
- Full stack traces with source file/line/column
- Source map resolution (inline base64 and external)
- VLQ decoding for original source positions
- Source map caching (50 files max, 2s fetch timeout)

#### User Action Capture
- Click events with multi-strategy selectors (data-testid > aria > role > CSS path)
- Input/textarea events (values redacted by default)
- Scroll events (throttled)
- Keypress tracking
- Navigation tracking (pushState, replaceState, popstate)
- Select/option changes
- Password field redaction

#### Screenshots on Error
- Auto-capture as JPEG (quality 80%)
- Rate limited (5s between captures, 10/session max)
- Auto-triggered on exceptions and errors
- Manual trigger via popup
- Server persists to disk and references in log entries

#### Error Grouping & Deduplication
- Error signature from message + stack + URL
- 5-second deduplication window
- Periodic flush of duplicate counts
- Aggregated tracking (_aggregatedCount, _firstSeen, _lastSeen)

#### Circuit Breaker + Exponential Backoff
- Extension resilience when server is down
- States: closed → open (after N failures) → half-open (after timeout)
- Backoff: doubles each failure, caps at configurable max

#### Debug Logging
- Circular buffer (200 entries max)
- Category-based logging (connection, capture, error, lifecycle, settings)
- Export debug log as JSON
- Debug mode toggle

---

### v4 — Real-Time Monitoring

#### WebSocket Monitoring
- WebSocket constructor interception and lifecycle tracking
- Connection state tracking (open/close/error)
- Message capture (incoming and outgoing)
- Adaptive sampling for high-frequency streams:
  - <10 msg/s: log all; 10–50: ~10/s; 50–200: ~5/s; >200: ~2/s
- Schema detection on first 5 messages per connection
- Binary message handling (hex preview <256B, size+magic for larger)
- Per-connection stats (rates, bytes, last message preview)
- Closed connection history (last 10)
- 500-event ring buffer, 4MB cap, 4KB message truncation

#### Network Body Capture
- Request/response payload capture for fetch/XHR
- Request bodies (POST/PUT/PATCH), 8KB truncation
- Response bodies (JSON, text, binary indicator), 16KB truncation
- Content-type aware handling
- 100-entry ring buffer, 8MB cap
- Header sanitization (auth, cookie, token, secret, key, password patterns)

#### Live DOM Queries (On-Demand)
- CSS selector-based DOM inspection
- Element attributes, text content, bounding box
- Optional computed styles and child subtree (depth-limited)
- 50 element max per query, 10s timeout, 500-char text truncation

#### Page Info
- URL, title, viewport size, scroll position, document height
- Form list with field detection
- Headings, links, images, interactive elements summary

#### Accessibility Auditing
- On-demand axe-core injection (200KB, dynamically loaded)
- Scoped audit (full page or selector-limited)
- WCAG rule tag filtering (wcag2a, wcag2aa, etc.)
- Result caching (30s per URL)
- Impact levels with node snippets and fix recommendations

#### Memory Enforcement (Auto-Eviction)
- Per-buffer limits: 4MB WebSocket, 8MB Network Bodies
- Global soft limit (20MB): reduce buffers by 50%
- Global hard limit (50MB): disable network bodies, server returns 503
- FIFO eviction of oldest entries

#### Performance SLOs & Guardrails
- Page load impact: <20ms target, 50ms hard limit
- Main thread per intercept: <1ms target, 5ms hard limit
- DOM query: <50ms target, 200ms hard limit
- a11y audit: <3s target, 10s hard limit
- Extension memory: <20MB soft, 50MB hard
- fetch() overhead: <0.5ms target
- WebSocket handler: <0.1ms/msg target

#### MCP Tools (v4)
- `get_websocket_events` — filter by connection, URL, direction, limit
- `get_websocket_status` — connection states, rates, schemas, sampling info
- `get_network_bodies` — filter by URL, method, status range, limit
- `query_dom` — CSS selector with styles/children/depth options
- `get_page_info` — page metadata and element summary
- `run_accessibility_audit` — scoped axe audits with WCAG filtering

---

### v5 — AI Preprocessing & Reproduction

#### AI Context Enrichment
- Async enrichment pipeline (never blocks main thread)
- Source code snippets from source maps (top 3 frames, 5 lines context, 10KB cap)
- Component ancestry (React fiber, Vue instances, Svelte meta)
- Application state snapshot (Redux, Zustand, Pinia, Svelte stores)
- Formatted AI context summary block per error entry
- Framework detection with prop/state extraction
- Timeout budgets per enrichment step

#### Enhanced Action Capture
- Multi-strategy selector generation (data-testid > aria-label > role > CSS path)
- Implicit ARIA role detection
- Dynamic class filtering (hash/random suffixes)
- Click location metadata
- Enhanced action buffer (server-side indexed)

#### Reproduction Script Generation
- User actions → Playwright test script
- Multi-strategy selectors in generated code
- Click, input, keypress, navigate, select, scroll steps
- Pause comments for gaps >2s
- Error message annotation
- 50KB output cap

#### Session Timeline
- Unified timeline of actions, network, console entries
- Chronological ordering with category filtering
- URL filtering

#### Test Generation
- Session timeline → Playwright tests
- DOM assertions, network response assertions, error assertions
- Configurable base_url

#### Persistent Storage (Cross-Session Memory)
- `.gasoline/` directory in project root (auto-added to .gitignore)
- Key-value persistence by namespace
- 1MB per file, 10MB per project caps
- Background flush (30s interval)
- Project metadata tracking (session count, first created)
- Error history with fingerprints and resolution status

#### Noise Filtering & Configuration
- Auto-detect noisy patterns from buffer data
- Categories: console, network, websocket
- Classifications: extension, framework, cosmetic, analytics, infrastructure, repetitive, dismissed
- Match specs: messageRegex, sourceRegex, urlRegex, method, status ranges, level
- Quick pattern dismissal with audit trail

#### Checkpoint & Diff System
- Named checkpoints (persist across sessions)
- Auto-advancing checkpoint
- Timestamp-based checkpoints (ISO 8601)
- Compressed diffs between checkpoints
- Console deduplication, network failure tracking, WebSocket disconnections
- Severity and category filtering

#### Performance Baselines & Regression Detection
- Page load timing snapshots (DCL, load, FCP, LCP, TTFB, DomInteractive)
- Network metrics (request count, transfer size)
- Long task tracking with CLS
- Running averages per URL (LRU eviction)
- Regression detection (% threshold)

#### Context Annotation Monitoring
- Tracks cumulative _context data size per entry
- 20KB threshold, 60s warning window
- Popup UI warning badge after 3 excessive entries

#### MCP Tools (v5)
- `check_performance` — snapshot + baseline comparison + regression flags
- `get_enhanced_actions` — filtered enhanced action buffer
- `get_reproduction_script` — Playwright script from actions
- `get_session_timeline` — unified chronological timeline
- `generate_test` — Playwright test from timeline
- `session_store` — persistent key-value store (save/load/list/delete/stats)
- `load_session_context` — project metadata, baselines, noise rules, error history
- `configure_noise` — add/remove/list/reset/auto_detect noise rules
- `dismiss_noise` — quick pattern dismissal
- `get_changes_since` — compressed diff from checkpoint

---

### E2E Test Coverage
- Console capture flow
- WebSocket capture and status
- Network body capture
- On-demand DOM queries
- Accessibility audits
- Feature toggles
- Performance budgets
- Popup status UI
- MCP HTTP protocol
- Reliability/reconnection
- v5 features

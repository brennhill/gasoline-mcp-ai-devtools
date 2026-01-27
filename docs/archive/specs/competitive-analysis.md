# Competitive Feature Analysis

**Date:** 2026-01-27
**Objective:** Achieve feature parity with BrowserTools MCP and analyze other competitors

---

## BrowserTools MCP - Feature Parity Required

### Current Gasoline Coverage

✅ **Already Implemented:**
- Real-time XHR/fetch monitoring (v4)
- Console log capture (v3)
- DOM element tracking (v4)
- Screenshot capture (v3)
- Network traffic monitoring (v4)
- Accessibility audits via axe-core (v4)

### Missing Features - IMMEDIATE ROADMAP

❌ **SEO Audit Tool**
- **What it does:** Lighthouse-powered analysis of on-page SEO factors
- **Capabilities needed:**
  - Metadata validation (title, description, og tags, canonical)
  - Heading structure (H1-H6 hierarchy)
  - Link structure (internal/external, broken links, nofollow)
  - Image optimization (alt text, dimensions, format)
  - Structured data validation (JSON-LD, schema.org)
  - Mobile-friendliness indicators
- **Output format:** Structured JSON with issues, warnings, and suggestions
- **Tool:** `generate {type: "seo_audit"}`
- **Effort:** ~300 lines Go + 150 lines extension JS

❌ **Performance Audit Tool (Comprehensive)**
- **What it does:** Lighthouse-style performance analysis
- **Capabilities needed:**
  - Render-blocking resource detection (CSS/JS in <head>)
  - DOM size analysis (total nodes, depth, child count)
  - Image optimization (uncompressed, wrong format, missing dimensions)
  - JavaScript bundle analysis (size, unused code estimation)
  - CSS analysis (unused rules, specificity issues)
  - Third-party script impact
  - Cache policy effectiveness
  - Resource compression detection
- **Output format:** Structured JSON with metrics, bottlenecks, recommendations
- **Tool:** `generate {type: "performance_audit"}`
- **Effort:** ~400 lines Go + 200 lines extension JS
- **Note:** Gasoline has basic performance metrics (FCP, LCP, CLS) - this extends to comprehensive analysis

❌ **Best Practices Audit Tool**
- **What it does:** Lighthouse-powered best practices evaluation
- **Capabilities needed:**
  - HTTPS usage
  - Deprecated API usage detection (via console warnings)
  - Browser error log analysis
  - Security headers validation (CSP, HSTS, X-Frame-Options)
  - Document metadata completeness
  - JavaScript error rate
  - Console noise level
  - Mixed content detection
- **Output format:** Structured JSON with pass/fail checks and recommendations
- **Tool:** `generate {type: "best_practices_audit"}`
- **Effort:** ~250 lines Go + 100 lines extension JS

❌ **Enhanced WCAG Accessibility Audit**
- **What it does:** Beyond basic axe-core - comprehensive WCAG compliance checking
- **Capabilities needed (beyond current axe-core integration):**
  - Color contrast analysis (all text/background combinations)
  - Keyboard navigation trap detection (tab order simulation)
  - ARIA attribute validation (role hierarchy, required attributes)
  - Form label completeness
  - Heading hierarchy validation
  - Skip link detection
  - Focus indicator visibility
  - Screen reader text sufficiency
- **Output format:** Structured JSON with violations by WCAG level (A, AA, AAA)
- **Tool:** Enhance existing `observe {what: "accessibility"}` with `wcag_level` and `detailed: true` options
- **Effort:** ~200 lines extension JS (enhanced axe-core configuration + post-processing)

❌ **Auto-Paste Screenshots to IDE**
- **What it does:** After screenshot capture, automatically paste into active IDE (Cursor, VS Code)
- **Capabilities needed:**
  - Detect active IDE from MCP client connection
  - Copy screenshot to clipboard
  - Trigger IDE paste action
  - Optional: Attach screenshot to MCP response as base64
- **Implementation:**
  - Server: Add screenshot base64 encoding in MCP responses
  - Extension: Add "copy to clipboard" button in popup
  - MCP client integration: Suggest including screenshots in observe responses when available
- **Tool:** Add `include_screenshots: true` option to `observe {what: "errors"}` and other relevant tools
- **Effort:** ~100 lines Go + 50 lines extension JS

---

## Chrome DevTools MCP - Feature Gap Analysis

### What It Is

**Type:** Official Google/Chrome browser automation + debugging MCP server

**Architecture:** Node.js server using Puppeteer + Chrome DevTools Protocol (CDP)

**Approach:** Automation-first with debugging capabilities (opposite of Gasoline's telemetry-first approach)

### Core Capabilities

**26 Tools in 6 Categories:**

1. **Input Automation (8 tools):** `click`, `drag`, `fill`, `fill_form`, `handle_dialog`, `hover`, `press_key`, `upload_file`
2. **Navigation Automation (6 tools):** `close_page`, `list_pages`, `navigate_page`, `new_page`, `select_page`, `wait_for`
3. **Debugging (5 tools):** `evaluate_script`, `get_console_message`, `list_console_messages`, `take_screenshot`, `take_snapshot`
4. **Performance Analysis (3 tools):** `performance_start_trace`, `performance_stop_trace`, `performance_analyze_insight`
5. **Network Monitoring (1 tool):** `list_network_requests`
6. **Emulation (3 tools):** `emulate_cpu`, `emulate_network`, `resize_page`

**Key Features:**
- Uses Puppeteer for browser automation
- Chrome DevTools Protocol (CDP) for deep debugging access
- Performance traces with automatic insight extraction (LCP, TBT, CLS, etc.)
- A11y tree-based page snapshots (text representation for AI)
- Network request monitoring
- Console message capture
- CPU/network throttling emulation
- Auto-connect to active Chrome sessions with user permission (new feature)

### Gasoline Coverage Analysis

✅ **Already Covered:**
- Console message capture → Gasoline v3 (more comprehensive: captures in real-time, not on-demand)
- Network monitoring → Gasoline v4 (real-time capture with bodies, not just request list)
- Screenshot capture → Gasoline v3 (automatic on errors + manual trigger)
- JavaScript execution → `interact {action: "execute_js"}` (v4)
- Navigation → `interact {action: "navigate/refresh/back/forward/new_tab"}` (v4)
- Form filling → Gasoline v4 (`execute_js`) + planned high-level API

### Missing Features - WORTH CONSIDERING

❌ **CPU/Network Emulation**
- **What it does:** Throttle CPU and network to simulate low-end devices or slow connections
- **Tools:** `emulate_cpu(rate: 1-6)`, `emulate_network(profile: "Slow 3G", "Fast 3G", "Offline", etc.)`
- **Use cases:**
  - Test performance on low-end devices (4x CPU slowdown)
  - Verify graceful degradation under poor network conditions
  - Reproduce mobile-specific performance issues
  - Test offline functionality
- **Gasoline equivalent:** None currently
- **Implementation:**
  - Use Chrome DevTools Protocol CDP `Emulation` domain
  - Add to `configure` tool: `configure {action: "emulate", cpu_rate: 4, network: "Slow 3G"}`
- **Verdict:** USEFUL - Complements existing performance monitoring
- **Effort:** ~150 lines Go (CDP integration) + ~50 lines extension JS
- **Priority:** MEDIUM - Nice-to-have for comprehensive performance testing

❌ **Performance Trace Analysis with Automatic Insight Extraction**
- **What it does:** Record performance trace, then automatically extract actionable insights
- **Tools:** `performance_start_trace()`, `performance_stop_trace()`, `performance_analyze_insight()`
- **Capabilities:**
  - Calculate LCP, TBT, CLS from trace data
  - Identify render-blocking resources
  - Detect long tasks and main thread bottlenecks
  - Generate human-readable recommendations
- **Gasoline current:** Has FCP/LCP/CLS snapshots (v5) but no trace-based analysis or automatic recommendations
- **Difference:** Chrome DevTools MCP generates AI-friendly insights ("Your LCP is 4.2s because image.jpg is 3MB uncompressed"); Gasoline just reports raw metrics
- **Implementation:**
  - Use Chrome DevTools Protocol `Tracing` domain
  - Add trace recording to performance monitoring
  - Parse trace events to extract bottlenecks
  - Generate structured recommendations
- **Verdict:** CONSIDER - Overlaps with planned Performance Audit Tool but uses different approach (trace-based vs DOM analysis)
- **Effort:** ~400 lines Go (trace parsing + analysis) + ~100 lines extension JS
- **Priority:** MEDIUM - Defer until Performance Audit Tool ships, then evaluate if trace-based analysis adds value

❌ **A11y Tree Snapshots (Text-Based Page Representation)**
- **What it does:** Export accessibility tree as text with unique IDs for elements
- **Tool:** `take_snapshot()` - Returns text representation of page based on a11y tree
- **Format:** Lists page elements with unique identifiers (uid) for AI consumption
- **Use cases:**
  - AI reads page structure without parsing HTML
  - Identify interactive elements by uid (e.g., "click button uid=123")
  - More token-efficient than full DOM for LLMs
- **Gasoline current:** `configure {action: "query_dom"}` returns HTML/attributes, not a11y tree text
- **Difference:** A11y tree is semantic (roles, labels) vs DOM is structural (tags, classes)
- **Implementation:**
  - Use Chrome Accessibility API
  - Export tree as text with uid mapping
  - Allow actions by uid reference
- **Verdict:** CONSIDER - Useful for AI-first interactions, complements existing DOM queries
- **Effort:** ~200 lines extension JS (a11y tree traversal + text serialization)
- **Priority:** LOW-MEDIUM - Interesting but not critical; Gasoline's DOM queries + annotated screenshots may cover this use case

❌ **Dialog Handling**
- **What it does:** Handle browser dialogs (alert, confirm, prompt, beforeunload)
- **Tool:** `handle_dialog(accept: true/false, text: "...")`
- **Gasoline current:** No explicit dialog handling (dialogs block interactions)
- **Implementation:**
  - Intercept dialog events via `window.addEventListener`
  - Auto-respond or queue for AI decision
  - Add to `interact` tool: `interact {action: "handle_dialog", accept: true}`
- **Verdict:** USEFUL - Low-effort, high-utility for automation workflows
- **Effort:** ~100 lines extension JS
- **Priority:** MEDIUM - Good addition to AI Web Pilot

❌ **Drag & Drop Automation**
- **What it does:** Programmatic drag-and-drop interactions
- **Tool:** `drag(from_selector, to_selector)`
- **Gasoline current:** No drag-and-drop support (would require `execute_js` workaround)
- **Implementation:**
  - Synthesize drag events (dragstart, dragover, drop)
  - Handle both HTML5 drag API and legacy mouse events
  - Add to `interact` tool: `interact {action: "drag", from: "...", to: "..."}`
- **Verdict:** USEFUL - Common UI pattern in modern apps
- **Effort:** ~150 lines extension JS
- **Priority:** MEDIUM - Nice-to-have for comprehensive automation

### Features Gasoline Does Better

**Real-time continuous capture vs on-demand queries:**
- Chrome DevTools MCP: Call `list_console_messages` → get messages at that moment
- Gasoline: Continuous capture in ring buffer, query anytime, `get_changes_since` for deltas

**WebSocket monitoring:**
- Chrome DevTools MCP: No WebSocket-specific tools
- Gasoline: Dedicated WS capture with adaptive sampling, schema detection, connection tracking

**Cross-session persistence:**
- Chrome DevTools MCP: Session-only data (lost when browser closes)
- Gasoline: `.gasoline/` directory with error history, noise rules, baselines

**Zero-dependency single binary:**
- Chrome DevTools MCP: Node.js + Puppeteer + CDP dependencies
- Gasoline: Single Go binary, stdlib-only

**4-tool constraint:**
- Chrome DevTools MCP: 26 tools → higher cognitive load for LLMs
- Gasoline: 4 tools → simpler, faster tool selection

**Framework-aware context enrichment:**
- Chrome DevTools MCP: Generic JavaScript execution
- Gasoline: React/Vue/Svelte component ancestry, state snapshots

### Competitive Positioning

| Aspect | Chrome DevTools MCP | Gasoline |
|--------|---------------------|----------|
| **Primary focus** | Automation + debugging | Telemetry capture + observation |
| **Architecture** | Node.js + Puppeteer | Go server + Chrome extension |
| **Tool count** | 26 tools | 4 tools |
| **Browser control** | Creates new instances OR connects to active | Extension in active browser only |
| **Data capture** | On-demand queries | Continuous real-time capture |
| **Performance** | Trace-based analysis (on-demand) | Continuous metrics + baseline regression |
| **WebSocket** | None | Adaptive sampling + schema detection |
| **Persistence** | Session-only | Cross-session (`.gasoline/` directory) |
| **Dependencies** | Node.js + Puppeteer + CDP | Zero (stdlib-only Go) |

**Verdict:** Chrome DevTools MCP and Gasoline have different primary use cases:
- **Chrome DevTools MCP:** Best for automation workflows with occasional debugging
- **Gasoline:** Best for continuous development telemetry with reproduction/testing support

**Gasoline should NOT try to compete on automation breadth** (Chrome DevTools MCP has official Google backing and Puppeteer integration). Instead, double down on differentiation: real-time telemetry, WebSocket monitoring, cross-session memory, framework-aware enrichment.

---

## Browserbase MCP & Browser MCP - Feature Gap Analysis

### Features Aligned with Gasoline's Mission

✅ **Already Covered:**
- Navigate to URLs → `interact {action: "navigate"}` (v4)
- Extract text content → `configure {action: "query_dom"}` (v4)
- Observe/find elements → `configure {action: "query_dom"}` (v4)
- Capture screenshots → Automatic on errors (v3) + manual trigger
- Session management → `interact {action: "save_state/load_state"}` (v4)
- Local execution, privacy-first → Core architecture ✓

### Enhancements - ADDED TO IMMEDIATE ROADMAP

✅ **Annotated Screenshots**
- **What it does:** Overlay element labels, bounding boxes, interaction hints on screenshots
- **Value:** Helps AI vision models understand what's clickable/visible
- **Implementation:**
  - Capture screenshot as base64
  - Overlay element selectors, ARIA labels, or custom annotations
  - Return annotated image
- **Tool:** `observe {what: "page", annotate_screenshot: true}`
- **Verdict:** IMMEDIATE - Useful for AI vision models, moderate effort
- **Effort:** ~150 lines extension JS (canvas overlay)

✅ **Form Filling Automation (High-Level API)**
- **What it does:** Populate form fields programmatically with ergonomic API
- **Current gap:** AI Web Pilot can execute JS to fill forms, but no high-level API
- **Implementation:**
  - Add `interact {action: "fill_form", selector: "form", fields: {...}}`
  - Auto-detect input types, handle validation
  - Trigger appropriate events for framework reactivity
- **Verdict:** IMMEDIATE - More ergonomic than `execute_js` for common use case
- **Effort:** ~200 lines extension JS

✅ **E2E Testing Integration (CI/CD)**
- **What it does:** Deep Playwright/Cypress integration, export test artifacts, CI observability
- **Current gap:** Have reproduction script generation (v5), but no CI integration
- **Implementation:**
  - Export Gasoline state as Playwright fixtures
  - Attach browser state to test failures automatically
  - Script injection via `addInitScript()` (no extension in CI)
  - CI integration guide (GitHub Actions, GitLab CI)
- **Tool:** `generate {type: "playwright_fixture"}`
- **Verdict:** IMMEDIATE - High economic impact ($30-60K/year per team), aligns with roadmap
- **Effort:** ~500 lines total (Go + CI integration + docs)

### Potential Enhancements - PLANNED (LONGER-TERM)

⚠️ **Local Web Scraping & Automation (LLM-Controlled)**
- **What it does:** Use Gasoline to scrape/automate as logged-in user, controlled by LLM
- **Value:** Leverage existing browser sessions to scrape authenticated content
- **Use cases:**
  - Scrape personal data from services you're logged into (banking, social media)
  - Extract data from internal company tools
  - Automate data entry across multiple systems
  - Monitor competitor sites with login-required access
- **Differentiation from Hyperbrowser:**
  - Uses YOUR browser with YOUR logged-in sessions
  - Localhost-only, no cloud service
  - Integrated with existing Gasoline telemetry
- **Implementation:**
  - Add `interact {action: "scrape"}` with multi-step workflow support
  - Workflow recording/replay mechanism
  - Structured data extraction with schema hints
- **Tool:** Extends existing `interact` tool
- **Verdict:** PLANNED - Useful longer-term, after core audit features ship
- **Effort:** ~800 lines (Go + extension JS + workflow engine)

### Deferred

⚠️ **Natural Language Action Execution (LLM-Powered)**
- **What it does:** Allow AI to control browser with natural language (e.g., "click the login button")
- **Current gap:** Gasoline requires explicit selectors or JavaScript code
- **Considerations:**
  - Adds complexity (LLM inference in the loop)
  - Overlaps with AI Web Pilot's `execute_js` action
  - May violate "capture, don't interpret" philosophy
- **Verdict:** DEFER - AI clients can already construct selectors themselves
- **Effort:** High (~500 lines + LLM integration)

---

## Hyperbrowser MCP - Inspirational (Local Variant)

**Type:** Cloud-based web scraping/crawling service

**Why direct adoption not applicable:**
- Gasoline is localhost-only telemetry capture, not a cloud scraping service
- Hyperbrowser scrapes arbitrary external sites at scale; Gasoline observes active browsing
- Features like CAPTCHA solving, proxy rotation, Bing search are irrelevant to Gasoline's mission

**However, the scraping use case IS valuable in a local context:**

**Local Web Scraping & Automation (Planned Feature)**
- Use Gasoline to scrape/automate as a logged-in user, controlled by LLM
- Leverage existing browser sessions/cookies (the user's actual browser)
- Localhost-only, no cloud service, no data sent to third parties
- Use cases:
  - Scrape personal data from services you're logged into (banking, social media exports)
  - Extract structured data from internal company tools
  - Automate data entry across multiple systems
  - Monitor competitor sites with login-required access
- Differentiates from Hyperbrowser:
  - Uses YOUR browser with YOUR logged-in sessions (not ephemeral cloud instances)
  - Privacy-first (no data leaves localhost)
  - Integrated with existing Gasoline telemetry (errors, network, WebSocket)

**Verdict:** Adopt the concept, implement as local-first LLM-controlled automation (longer-term roadmap)

---

## Competitive Positioning Summary

| Competitor | Type | Overlap with Gasoline | Action |
|------------|------|----------------------|---------|
| **BrowserTools MCP** | Browser monitoring + auditing | HIGH - Direct competitor, same use case | ✅ **Feature parity required (immediate)** |
| **Chrome DevTools MCP** | Browser automation + debugging (official Google) | MEDIUM - Automation-first vs telemetry-first | ✅ **Adopt select features: emulation, dialog handling, drag & drop (planned)** |
| **Browserbase MCP** | Cloud browser automation | MEDIUM - Similar goals, different execution | ✅ **Adopt vision features (annotated screenshots, form filling)** |
| **Browser MCP** | Local browser automation | MEDIUM - Similar architecture, different focus | ✅ **Adopt E2E testing integration** |
| **Hyperbrowser MCP** | Cloud scraping service | LOW (but inspirational) | Consider local scraping variant (longer-term) |

---

## Immediate Roadmap Additions (Feature Competitive)

**Priority: HIGH - BrowserTools MCP Parity**

1. **SEO Audit Tool** - `generate {type: "seo_audit"}` (~450 lines total)
2. **Performance Audit Tool** - `generate {type: "performance_audit"}` (~600 lines total)
3. **Best Practices Audit Tool** - `generate {type: "best_practices_audit"}` (~350 lines total)
4. **Enhanced WCAG Accessibility Audit** - Enhance `observe {what: "accessibility"}` (~200 lines)
5. **Auto-Paste Screenshots to IDE** - Add `include_screenshots: true` to MCP responses (~150 lines total)

**Priority: HIGH - Browserbase/Browser MCP Enhancements**

6. **Annotated Screenshots** - `observe {what: "page", annotate_screenshot: true}` (~150 lines)
7. **Form Filling Automation** - `interact {action: "fill_form"}` with high-level API (~200 lines)
8. **E2E Testing Integration (CI/CD)** - Playwright fixtures, CI observability, test failure enrichment (~500 lines)

**Total estimated effort:** ~2600 lines of code across Go server + extension JS + CI integration

**Priority: PLANNED**

**Chrome DevTools MCP Enhancements (MEDIUM priority):**

9. **CPU/Network Emulation** - `configure {action: "emulate"}` with device/network throttling (~200 lines)
10. **Dialog Handling** - `interact {action: "handle_dialog"}` for alert/confirm/prompt (~100 lines)
11. **Drag & Drop Automation** - `interact {action: "drag"}` for drag-and-drop UI patterns (~150 lines)
12. **A11y Tree Snapshots** - `observe {what: "a11y_tree"}` for text-based page representation (~200 lines)

**Priority: PLANNED (Longer-Term)**

13. **Local Web Scraping & Automation** - LLM-controlled scraping as logged-in user (~800 lines)

**Deferred:**
- Natural language action execution (overlaps with existing capabilities, AI clients handle this)
- Performance trace analysis (overlaps with planned Performance Audit Tool; defer until that ships)

---

## Gasoline's Unique Competitive Advantages

**What competitors DON'T have that Gasoline does:**

1. **Real-time WebSocket telemetry capture** - No competitor captures WS message streams with adaptive sampling
2. **Cross-session persistent memory** - Error history, noise rules, baselines persisted in `.gasoline/` directory
3. **Reproduction script generation** - User actions → Playwright tests
4. **Time-windowed diffs** - `get_changes_since` for token-efficient incremental reads
5. **Zero-dependency Go server** - Single binary, no supply chain risk
6. **Async command architecture with correlation IDs** - Non-blocking browser control
7. **4-tool constraint** - Simpler API surface for LLMs vs 15+ tools in competitors
8. **Privacy-first localhost-only** - No cloud dependency, no data leaves machine
9. **Framework-aware context enrichment** - React/Vue/Svelte component ancestry in error context
10. **Performance regression detection** - Baseline tracking with automatic regression flagging

**Maintain these advantages while adding audit features for parity.**

# QA Plan: CPU/Network Emulation

> QA plan for the CPU/Network Emulation feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

**Note:** No TECH_SPEC.md is available for this feature. This QA plan is based solely on the PRODUCT_SPEC.md.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Gasoline runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | chrome.debugger grants elevated CDP access beyond emulation scope | Verify the extension only sends `Emulation.setCPUThrottlingRate` and `Network.emulateNetworkConditions` CDP commands. No other CDP domains are used (e.g., no `Network.getResponseBody`, `DOM.getDocument`, `Runtime.evaluate` via debugger). | critical |
| DL-2 | Debugger attachment exposes page data via CDP | Verify that attaching the debugger does not enable any data-reading CDP domains beyond what is needed for emulation. Only `Emulation` and `Network.enable` + `Network.emulateNetworkConditions` should be invoked. | critical |
| DL-3 | Emulation state leaks to response metadata | Verify emulation response only contains emulation parameters (cpu_rate, network profile), tab_id, and status -- no page content or captured data. | medium |
| DL-4 | Emulation affects browser security features | Verify CPU/network throttling does not disable HTTPS validation, certificate checks, CORS enforcement, or other browser security mechanisms. | critical |
| DL-5 | Stale emulation left on tab after disconnect | Verify emulation is automatically cleared when extension disconnects, tab closes, or new MCP session begins. Stale throttling is a usability hazard. | high |
| DL-6 | Emulation opt-in toggle bypass | Verify emulation cannot be activated without the user explicitly enabling "Allow Emulation" in extension options. No server-side override. | critical |
| DL-7 | Custom network profile parameters allow negative values or overflow | Verify custom_network.download, upload, and latency are validated (non-negative, reasonable bounds). Negative values could cause unexpected CDP behavior. | medium |
| DL-8 | Tab ID parameter allows targeting non-tracked tabs | Verify `tab_id` parameter only accepts IDs of tabs currently tracked by the extension. No emulation of untracked tabs. | high |

### Negative Tests (must NOT leak)
- [ ] Debugger attachment must NOT enable `Runtime.evaluate` or other code execution CDP domains
- [ ] Debugger attachment must NOT enable `Network.getResponseBody` or other data-reading CDP domains
- [ ] Emulation must NOT be possible when "Allow Emulation" toggle is OFF in extension options
- [ ] Emulation response must NOT contain page content, cookies, or captured telemetry data
- [ ] Emulation must NOT disable browser security features (HTTPS, CORS, CSP enforcement)
- [ ] Stale emulation must NOT persist after extension disconnect or tab close

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Status values are unambiguous | Response `status` is one of: `"emulation_active"`, `"emulation_cleared"`. Error has `"error"` field. | [ ] |
| CL-2 | CPU rate description is human-readable | `cpu.description` says "4x CPU slowdown" (not just the numeric rate). | [ ] |
| CL-3 | Network profile includes all relevant metrics | `network` object includes profile name, download/upload in KB/s, and latency in ms. | [ ] |
| CL-4 | Warning about debugger banner is present | Response includes `warning` field noting "chrome.debugger attached" banner is visible. | [ ] |
| CL-5 | Permission denied error is actionable | Error response includes a `hint` telling the LLM to ask the user to enable emulation. | [ ] |
| CL-6 | Reset response clearly indicates normal state | Reset response shows `"cpu": "normal"` and `"network": "normal"`. | [ ] |
| CL-7 | Custom network takes precedence over named profile | When both `network` and `custom_network` are provided, response shows `custom_network` values. | [ ] |
| CL-8 | Invalid parameter errors specify valid ranges | Error for cpu_rate=7 includes "valid range: 1-6". Error for unknown network profile lists valid profiles. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM may assume emulation persists across page reloads. Clarify behavior in response (CDP debugger stays attached across navigations).
- [ ] LLM may not realize the "controlled by automated software" banner is visible to the user. Verify warning is prominent in response.
- [ ] LLM may confuse cpu_rate=1 (no throttle) with cpu_rate=0 (invalid). Verify error messages clarify minimum is 1.
- [ ] LLM may issue observe calls during emulation without noting throttled conditions. Verify emulation state is accessible via health check.
- [ ] LLM may try to apply emulation to chrome:// pages. Verify clear error when debugger cannot attach.

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium (requires user opt-in before use)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Apply CPU + network throttling | 1 step: `configure({action: "emulate", cpu_rate: 4, network: "Slow 3G"})` | No -- already minimal for combined throttling |
| Apply CPU throttling only | 1 step: `configure({action: "emulate", cpu_rate: 4})` | No |
| Apply network throttling only | 1 step: `configure({action: "emulate", network: "Slow 3G"})` | No |
| Simulate offline mode | 1 step: `configure({action: "emulate", network: "Offline"})` | No |
| Reset all emulation | 1 step: `configure({action: "emulate", reset: true})` | No |
| First-time setup (opt-in) | User must manually enable "Allow Emulation" in extension options + 1 MCP call | Cannot simplify -- security requirement |
| Performance test workflow | 3 steps: apply emulation + observe vitals + reset | Could combine into a single "performance test" mode but 3 steps is acceptable |

### Default Behavior Verification
- [ ] Feature is disabled by default (opt-in toggle OFF in extension options)
- [ ] Calling emulate without opt-in returns actionable error with hint
- [ ] Reset when no emulation is active returns success (idempotent)
- [ ] Named network profiles use standard Chrome DevTools defaults without configuration

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Parse emulate action | `{action: "emulate", cpu_rate: 4}` | Route to emulation handler with rate=4 | must |
| UT-2 | Validate cpu_rate range (1-6) | `cpu_rate: 0`, `1`, `3`, `6`, `7`, `-1` | Accept 1-6, reject others with range error | must |
| UT-3 | Validate named network profile | `"Slow 3G"`, `"Fast 3G"`, `"4G"`, `"Offline"`, `"No Throttle"` | All accepted with correct parameters | must |
| UT-4 | Reject unknown network profile | `"5G"`, `""`, `"slow3g"` | Error listing valid profile names | must |
| UT-5 | Parse custom_network object | `{download: 375000, upload: 75000, latency: 200}` | Custom profile created with specified values | must |
| UT-6 | Custom network takes precedence | Both `network: "Slow 3G"` and `custom_network` provided | custom_network values used | should |
| UT-7 | Reset ignores other parameters | `{reset: true, cpu_rate: 4}` | Only reset is processed; cpu_rate ignored | must |
| UT-8 | Validate custom_network values are non-negative | `{download: -1}` | Error: download must be non-negative | must |
| UT-9 | Build CDP command for CPU throttling | cpu_rate=4 | `Emulation.setCPUThrottlingRate` with `{rate: 4}` | must |
| UT-10 | Build CDP command for network conditions (Slow 3G) | network="Slow 3G" | `Network.emulateNetworkConditions` with `{offline: false, latency: 2000, downloadThroughput: 50000, uploadThroughput: 6250}` | must |
| UT-11 | Build CDP command for Offline | network="Offline" | `Network.emulateNetworkConditions` with `{offline: true, ...}` | must |
| UT-12 | Emulation state tracking | Apply cpu_rate=4 then query state | State shows cpu_rate=4 for tab | must |
| UT-13 | Emulation state cleared on reset | Apply then reset | State shows no active emulation | must |
| UT-14 | Idempotent reset | Reset when no emulation active | Success response, no error | must |
| UT-15 | Response format for active emulation | Apply cpu_rate=4, network="Slow 3G" | JSON with status, cpu, network, tab_id, warning fields | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end CPU throttling | Go server + Extension + chrome.debugger + CDP | CPU throttling applied; response confirms settings | must |
| IT-2 | End-to-end network throttling | Go server + Extension + chrome.debugger + CDP | Network throttling applied; page loads visibly slower | must |
| IT-3 | Emulation with opt-in disabled | Go server + Extension (toggle OFF) | `{error: "emulation_disabled"}` with hint | must |
| IT-4 | Emulation reset flow | Go server + Extension + CDP | Throttling removed; debugger detached; response confirms | must |
| IT-5 | Emulation via async command pipeline | Go server pending queries + Extension poll | Pending query dispatched, extension executes CDP, result returned | must |
| IT-6 | Extension disconnect during active emulation | Extension disconnects | Extension resets all emulation before disconnect (detach debugger) | must |
| IT-7 | Tab closed during active emulation | Close tab with emulation | Debugger auto-detaches; server clears emulation state for tab | should |
| IT-8 | Emulation state in health response | Apply emulation then check health | Health response includes current emulation state | should |
| IT-9 | Observe vitals under throttled conditions | Apply Slow 3G + observe vitals | Vitals reflect degraded performance (longer load times) | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Emulation apply response time (first call, includes debugger attach) | End-to-end latency | < 500ms | must |
| PT-2 | Emulation apply response time (subsequent calls, debugger already attached) | End-to-end latency | < 200ms | must |
| PT-3 | Emulation reset response time | End-to-end latency | < 200ms | must |
| PT-4 | Server memory for emulation state | Memory overhead | < 1KB per tab | must |
| PT-5 | Browsing performance when emulation is OFF | Page load impact | Zero (no debugger attached) | must |
| PT-6 | Extension poll latency for emulation command | Pending query pickup time | < 2s | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Emulate with opt-in disabled | Call without enabling toggle | `emulation_disabled` error with hint | must |
| EC-2 | Debugger already attached by another tool | Other CDP user active | Reuse existing debugger session; no double-attach | should |
| EC-3 | Tab closed while emulation active | User closes tab | Debugger auto-detaches; state cleared on next check | must |
| EC-4 | Extension disconnects with active emulation | Extension process stops | Extension resets all emulation on disconnect | must |
| EC-5 | Invalid cpu_rate | cpu_rate: 0, -1, 7, 100 | Structured error with valid range [1-6] | must |
| EC-6 | Both network and custom_network provided | Both specified | custom_network takes precedence; documented in response | should |
| EC-7 | Reset when no emulation active | Reset with no prior emulation | `emulation_cleared` success (idempotent) | must |
| EC-8 | Multiple tabs with different emulation | Tab A: Slow 3G, Tab B: 4G | Independent emulation per tab | could |
| EC-9 | User manually detaches debugger | chrome://inspect detach | Extension detects onDetach event, clears state | should |
| EC-10 | chrome:// page target | Emulate on chrome://settings | Error: cannot attach debugger to browser internal pages | must |
| EC-11 | DevTools page target | Emulate on devtools:// URL | Error: cannot attach debugger to DevTools pages | must |
| EC-12 | Emulate called during page navigation | Emulate while page is loading | CDP commands applied; may need re-application after navigation completes | should |
| EC-13 | Apply emulation, then apply different settings | cpu_rate=2 then cpu_rate=4 | Second call updates to cpu_rate=4; response shows new state | must |
| EC-14 | Apply only CPU, then add network | cpu_rate=4 then network="Slow 3G" | Both active; response shows combined state | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Gasoline server running: `./dist/gasoline --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web page loaded in tracked tab (preferably with images/resources for visible network throttling)
- [ ] "Allow Emulation" toggle enabled in extension options

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "configure", "arguments": {"action": "emulate", "cpu_rate": 4}}` | Chrome shows "controlled by automated software" banner | Response: `{status: "emulation_active", cpu: {rate: 4, description: "4x CPU slowdown"}}` | [ ] |
| UAT-2 | Reload the page manually | Page noticeably slower due to CPU throttling | Interactions feel sluggish; JavaScript execution is delayed | [ ] |
| UAT-3 | `{"tool": "configure", "arguments": {"action": "emulate", "network": "Slow 3G"}}` | Page resources load very slowly on next navigation | Response shows network profile with Slow 3G values (400 KB/s down, 50 KB/s up, 2000ms latency) | [ ] |
| UAT-4 | Navigate to a resource-heavy page | Images and scripts load with visible delay | Network waterfall shows throttled transfer rates | [ ] |
| UAT-5 | `{"tool": "observe", "arguments": {"what": "vitals"}}` | (AI reads the result) | Web Vitals show degraded values compared to unthrottled baseline | [ ] |
| UAT-6 | `{"tool": "configure", "arguments": {"action": "emulate", "network": "Offline"}}` | Page shows offline/connection error on next navigation | Response shows `{network: {profile: "Offline", ...}}` | [ ] |
| UAT-7 | Try to load any URL | Browser shows offline error | No network requests succeed | [ ] |
| UAT-8 | `{"tool": "configure", "arguments": {"action": "emulate", "reset": true}}` | "Controlled by automated software" banner disappears | Response: `{status: "emulation_cleared", cpu: "normal", network: "normal"}` | [ ] |
| UAT-9 | Reload the page | Page loads at normal speed | Full speed; no throttling | [ ] |
| UAT-10 | Disable "Allow Emulation" toggle in extension, then: `{"tool": "configure", "arguments": {"action": "emulate", "cpu_rate": 2}}` | No debugger attachment | Response: `{error: "emulation_disabled", message: "...", hint: "Ask the user to enable..."}` | [ ] |
| UAT-11 | `{"tool": "configure", "arguments": {"action": "emulate", "custom_network": {"download": 375000, "upload": 75000, "latency": 200}}}` (with toggle re-enabled) | Custom throttling applied | Response shows custom network values | [ ] |
| UAT-12 | Close the tracked tab while emulation is active | Tab closes | Emulation state cleared; no stale throttling on next tab | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Debugger does not read page data | During active emulation, check that no page content appears in server logs or MCP responses beyond normal telemetry | Only emulation state in emulation-related responses; no extra page data | [ ] |
| DL-UAT-2 | Emulation opt-in is enforced | With toggle OFF, attempt emulation via MCP | Denied with actionable error | [ ] |
| DL-UAT-3 | Security features not affected | Visit HTTPS site during emulation; check for certificate warnings or CORS relaxation | Normal browser security behavior maintained | [ ] |
| DL-UAT-4 | Auto-cleanup on disconnect | Disconnect extension while emulation active; reconnect | Emulation cleared; no stale throttling | [ ] |
| DL-UAT-5 | No extra CDP domains activated | During emulation, check chrome://inspect for active CDP sessions | Only Emulation and Network domains active, no others | [ ] |

### Regression Checks
- [ ] Existing `configure` actions (capture, health, store, query_dom) still work correctly
- [ ] Observe tools produce normal output when emulation is not active
- [ ] Extension performance not degraded when emulation toggle is OFF (no debugger attached)
- [ ] Extension reconnect after disconnect works normally
- [ ] Other pending query types (DOM queries, a11y audits) are not affected by emulation commands

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| LLM Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |

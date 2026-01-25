# Product Specification: Gasoline (v2.0)

**Version:** 2.0 (Draft)  
**Status:** Planning  
**Repository:** [https://github.com/brennhill/gasoline](https://github.com/brennhill/gasoline)  
**Core Objective:** Transform Gasoline from a passive browser observer into an active, state-aware debugging partner utilizing the Model Context Protocol (MCP).

---

## 1. System Architecture Overview

To support "Supercharged" debugging, the architecture must move beyond simple DOM serialization to support high-fidelity, bi-directional communication between the Browser context and the MCP Server.

### Data Flow
1.  **The Target:** Chrome/Browser running the `gasoline-client` (Extension or injected script).
2.  **The Bridge:** Local HTTP Server (Python/Node) acting as the **MCP Host**.
3.  **The Brain:** AI Client (Claude Desktop/Cursor/IDE) connecting via MCP.

### Global Data Structure (The Session Context)
The MCP Server must maintain a `SessionState` object in memory to provide context beyond the immediate request:

```typescript
interface SessionState {
  url: string;
  domTree: Node;
  consoleBuffer: LogEntry[];       // Circular buffer of last 50 logs
  networkBuffer: NetworkRequest[]; // Circular buffer of last 50 XHR/Fetch
  actionHistory: Action[];         // Recorded steps for test generation
  knownSelectors: Map<string, string>; // Semantic memory (e.g., "login_btn" -> "#u_0_2")
}
```

---

## 2. Phase 1: Deep Observability
*Goal: Allow the AI to see invisible failures (Network/Console) and verify visual layout.*

### 2.1 Feature: Console Log Streaming
**User Story:** As a developer, I want the AI to see the JavaScript errors (stack traces) occurring in the console so it can diagnose logic failures immediately.

**Implementation:**
* **Browser:** Overload `window.console.error`, `window.console.warn`, and `window.onerror`.
* **Bridge:** Maintain a sliding window (last 50 logs).
* **MCP Tool:** `get_browser_logs`

**Tool Schema:**
```json
{
  "name": "get_browser_logs",
  "description": "Retrieves the last N console logs, errors, or warnings from the browser.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "level": { 
        "type": "string", 
        "enum": ["all", "error", "warn", "info"],
        "default": "error"
      },
      "limit": { "type": "integer", "default": 20 }
    }
  }
}
```

### 2.2 Feature: Network Request Interception (HAR-lite)
**User Story:** As a developer, I want the AI to analyze failed API calls (400/500 errors) to see if the payload I sent was incorrect.

**Implementation:**
* **Browser:** Monkey-patch `window.fetch` and `XMLHttpRequest`.
* **Filter:** STRICTLY ignore media types (images, fonts, css). Capture `application/json` payloads.
* **MCP Tool:** `get_network_activity`

**Tool Schema:**
```json
{
  "name": "get_network_activity",
  "description": "Retrieves recent network requests, specifically looking for failures.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "status_filter": { "type": "string", "enum": ["failed", "all"], "default": "failed" },
      "include_bodies": { "type": "boolean", "default": true }
    }
  }
}
```

### 2.3 Feature: Visual Snapshotting
**User Story:** As a developer, I want the AI to verify if an element is actually visible to the user (checking for z-index or overflow issues).

**Implementation:**
* **Browser:** Use `html2canvas` or `chrome.tabs.captureVisibleTab`.
* **Optimization:** Convert to Grayscale + Resize to max 800px width before sending to save tokens.
* **MCP Tool:** `take_screenshot`

**Tool Schema:**
```json
{
  "name": "take_screenshot",
  "description": "Returns a base64 encoded screenshot of the current viewport or a specific element.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "selector": { "type": "string", "description": "Optional CSS selector to crop to." }
    }
  }
}
```

---

## 3. Phase 2: Bi-Directional Interaction
*Goal: Create a "Sidecar" experience where the AI communicates ON the screen.*

### 3.1 Feature: AI Visual Pointer (Highlighting)
**User Story:** As a developer, I want the AI to point to elements on my screen so we can communicate clearly about UI components.

**Implementation:**
* **Browser:** Inject a generic `div` overlay (`#gasoline-highlighter`) with `border: 4px solid red; z-index: 99999;`.
* **Logic:** Update the overlay's position based on the target element's `getBoundingClientRect()`.
* **MCP Tool:** `highlight_element`

**Tool Schema:**
```json
{
  "name": "highlight_element",
  "description": "Draws a red box around a specific element on the user's screen to indicate focus.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "selector": { "type": "string" },
      "duration_ms": { "type": "integer", "default": 5000 }
    },
    "required": ["selector"]
  }
}
```

### 3.2 Feature: Runtime Code Injection
**User Story:** As a developer, I want the AI to check the value of a Redux store or global variable not rendered in the HTML.

**Implementation:**
* **Security:** Only allow on `localhost`.
* **Browser:** Execute via `eval()` or `new Function()`. Return JSON-serialized results.
* **MCP Tool:** `execute_javascript`

**Tool Schema:**
```json
{
  "name": "execute_javascript",
  "description": "Executes raw JavaScript in the browser context and returns the result.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "script": { "type": "string", "description": "The JS code to run. Must return a JSON-serializable value." }
    },
    "required": ["script"]
  }
}
```

---

## 4. Phase 3: Learning & Persistence
*Goal: Reduce "Time to Context" by giving the AI a long-term memory.*

### 4.1 Feature: Semantic Selector Cache
**User Story:** As a developer, I want the AI to remember that the "Submit" button is `#complex-id-123` so it doesn't have to re-analyze the DOM every run.

**Implementation:**
* **Storage:** `gasoline_memory.json` in project root.
* **Logic:**
    1. Check Memory for semantic name (e.g., "checkout_submit").
    2. If missing, analyze DOM.
    3. If DOM analysis succeeds, call `learn_selector` to save for next time.

**Tool Schema:**
```json
{
  "name": "learn_selector",
  "description": "Saves a semantic name for a CSS selector to long-term memory.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "semantic_name": { "type": "string", "description": "E.g., 'login_button'" },
      "selector": { "type": "string" }
    }
  }
}
```

### 4.2 Feature: State Checkpointing (Time Travel)
**User Story:** As a developer, I want to save the state of "Cart Full" so I can instantly restore it to test a checkout bug without clicking through the shop again.

**Implementation:**
* **Capture:** Serialize `localStorage`, `sessionStorage`, and `document.cookie`.
* **MCP Tool:** `manage_state`

**Tool Schema:**
```json
{
  "name": "manage_state",
  "description": "Save or Load a browser state snapshot.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "action": { "type": "string", "enum": ["save", "load", "list"] },
      "snapshot_name": { "type": "string" }
    }
  }
}
```

---

## 5. Phase 4: Deliverables
*Goal: Turn debugging sessions into permanent code assets.*

### 5.1 Feature: Self-Healing Test Generator
**User Story:** As a developer, after the AI automates a flow, I want it to output a Playwright test file so I can add it to my CI pipeline.

**Implementation:**
* **Recorder:** Bridge records all successful actions in `session.actionHistory`.
* **Generator:** AI consumes history and maps it to Playwright syntax (e.g., `page.click(selector)`).
* **MCP Tool:** `generate_test_file`

**Tool Schema:**
```json
{
  "name": "generate_test_file",
  "description": "Converts the recent session history into a Playwright/Cypress test.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "framework": { "type": "string", "enum": ["playwright", "cypress"], "default": "playwright" },
      "test_name": { "type": "string" }
    }
  }
}
```

---

## 6. Security & Risks

1.  **Arbitrary Code Execution:** The `execute_javascript` tool is powerful.
    * *Mitigation:* Bind server strictly to `127.0.0.1`. Display a visible warning in the terminal when active.
2.  **Sensitive Data Leaks:** HAR logs may contain Auth Tokens.
    * *Mitigation:* Implement a Redaction Middleware to mask `Authorization` headers and `password` fields before sending data to the LLM.
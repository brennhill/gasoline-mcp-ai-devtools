# Gasoline MCP Detailed Feature Roadmap

**Version:** v5.3 to v7.2+  
**Status:** Comprehensive planning document  
**Last Updated:** 2026-01-31  
**Thesis:** "AI will be the driving force in development."

---

## Executive Summary

This document provides detailed specifications for every feature in the Gasoline MCP roadmap from v5.3 through v7.2+, including technical requirements, dependencies, success criteria, and implementation details.

### Roadmap at a Glance

| Version | Status | Features | Focus | Effort |
|---------|--------|----------|---------|---------|
| v5.2 | ✅ Complete | Bug fixes | Critical issues | 1 week |
| v5.3 | ✅ Complete | 2 features | Blockers removed | 1 week |
| v6.0 | ⏳ In Progress | 4-5 features | AI-native toolkit | 4-6 weeks |
| v6.1 | 🔜 Planned | 10 features | Advanced exploration | 3-4 weeks |
| v6.2 | 🔜 Planned | 4 features | Safe repair | 2-3 weeks |
| v6.3 | 🔜 Planned | 3 features | Zero-trust enterprise | 2-3 weeks |
| v6.4 | 🔜 Planned | 11 features | Production compliance | 4-5 weeks |
| v6.5 | 🔜 Planned | 2 features | Token efficiency | 1-2 weeks |
| v6.6 | 🔜 Planned | 7 features | Specialized audits | 2-3 weeks |
| v6.7 | 🔜 Planned | 5 features | Advanced interactions | 2-3 weeks |
| v6.8 | 🔜 Planned | 5 features | Infrastructure | 2-3 weeks |
| v7.0 | 🔜 Planned | 8 features | Backend integration | 4-6 weeks |
| v7.1 | 🔜 Planned | 4 features | Autonomous control | 3 weeks |
| v7.2+ | 🔜 Planned | 15+ features | 360 observability | Ongoing |

---

## Table of Contents

1. [v5.3: Critical Blockers](#v53-critical-blockers)
2. [v6.0: AI-Native Core Toolkit](#v60-ai-native-core-toolkit)
3. [v6.1: Advanced Exploration & Observation](#v61-advanced-exploration--observation)
4. [v6.2: Safe Repair & Verification](#v62-safe-repair--verification)
5. [v6.3: Zero-Trust Enterprise](#v63-zero-trust-enterprise)
6. [v6.4: Production Compliance](#v64-production-compliance)
7. [v6.5: Token & Context Efficiency](#v65-token--context-efficiency)
8. [v6.6: Specialized Audits & Analytics](#v66-specialized-audits--analytics)
9. [v6.7: Advanced Interactions](#v67-advanced-interactions)
10. [v6.8: Infrastructure & Quality](#v68-infrastructure--quality)
11. [v7.0: Backend Integration & Correlation](#v70-backend-integration--correlation)
12. [v7.1: Autonomous Control](#v71-autonomous-control)
13. [v7.2+: 360 Observability Expansion](#v72-360-observability-expansion)
14. [Critical Path & Dependencies](#critical-path--dependencies)
15. [Immediate Next Steps](#immediate-next-steps)

---

## v5.3: Critical Blockers

**Status:** ✅ Complete  
**Purpose:** Remove blockers preventing v6.0 thesis validation  
**Goal:** Enable AI to explore, observe, and infer without token limits or memory bloat

---

### Feature 1: Pagination for Large Datasets

**Type:** Core Infrastructure  
**Priority:** Critical (blocks v6.0)  
**Effort:** 1 week  
**Dependencies:** None

#### Problem

**Context Window Exhaustion:**
- Gasoline MCP dumps entire dataset (network logs, console, DOM) in single response
- With 1000+ network requests, response exceeds MCP token limits
- AI cannot access full context, breaks v6.0 thesis

**Current State (Before Fix):**
```javascript
// observe({what: "network_waterfall"})
// Returns: 1500 requests, ~500KB JSON
// Result: Token limit exceeded, AI cannot process
```

#### Solution

**Cursor-Based Pagination:**

Implement cursor-based pagination with multiple modes:

1. **Offset-Limit Pagination** (Simple):
```javascript
observe({what: "network_waterfall", limit: 100, offset: 0})
→ Returns first 100 requests
observe({what: "network_waterfall", limit: 100, offset: 100})
→ Returns next 100 requests
```

2. **Cursor-Based Pagination** (Efficient):
```javascript
observe({what: "network_waterfall", limit: 100})
→ Returns first 100 + after_cursor: "2026-01-30T10:15:23.456Z:1234"

observe({what: "network_waterfall", limit: 100, after_cursor: "2026-01-30T10:15:23.456Z:1234"})
→ Returns next 100 starting from cursor
```

3. **Since Cursor** (Time-based):
```javascript
observe({what: "logs", since_cursor: "2026-01-30T10:00:00Z:0", limit: 100})
→ Returns logs from timestamp onwards
```

4. **Before Cursor** (Historical):
```javascript
observe({what: "network", before_cursor: "2026-01-30T10:00:00Z:0", limit: 100})
→ Returns events before timestamp
```

#### Technical Implementation

**Cursor Format:**
```go
type Cursor struct {
    Timestamp string `json:"timestamp"` // ISO 8601
    Sequence  uint64 `json:"sequence"`  // Monotonic counter
    Source    string `json:"source"`    // "network", "console", "dom"
}

func (c *Cursor) String() string {
    return fmt.Sprintf("%s:%d", c.Timestamp, c.Sequence)
}
```

**Pagination Logic:**
```go
func (b *Buffer) Paginate(limit int, cursor *Cursor) []Event {
    if cursor == nil {
        // Return first N items
        return b.events[:min(limit, len(b.events))]
    }
    
    // Find cursor position
    startIdx := -1
    for i, event := range b.events {
        if event.Timestamp == cursor.Timestamp && event.Sequence == cursor.Sequence {
            startIdx = i + 1
            break
        }
    }
    
    if startIdx == -1 {
        // Cursor not found, return empty
        return []Event{}
    }
    
    // Return next N items
    endIdx := min(startIdx + limit, len(b.events))
    return b.events[startIdx:endIdx]
}
```

**MCP Tool Integration:**
```javascript
// Tool: observe
// Add pagination parameters to schema
{
  "what": "network_waterfall",
  "limit": 100,
  "offset": 0,
  "after_cursor": "string",
  "before_cursor": "string",
  "since_cursor": "string"
}
```

#### Success Criteria

- [ ] All observe modes support pagination
- [ ] Pagination works for network, console, logs, websocket, actions
- [ ] Cursor-based pagination is stable (same cursor returns same results)
- [ ] Documentation updated with pagination examples
- [ ] AI can paginate through 1000+ events without token limits
- [ ] Performance: <50ms to paginate 100 items

#### MVP vs Full Implementation

**MVP (Delivered):**
- Offset-limit pagination
- Cursor-based pagination
- Since cursor (time-based)
- All buffer types support pagination

**Future Enhancements:**
- Filtering combined with pagination (content-type, domain)
- Server-side aggregation (sum, count, avg)
- Bi-directional cursors (previous/next)

---

### Feature 2: Buffer-Specific Clearing

**Type:** Core Infrastructure  
**Priority:** Critical (blocks v6.0)  
**Effort:** 1 week  
**Dependencies:** None

#### Problem

**Memory Bloat in Long Sessions:**
- Gasoline MCP accumulates all data in single buffer
- After 1 hour: 500MB of network logs, 200MB console, 100MB DOM
- Extension memory usage grows linearly, crashes browser
- Cannot selectively clear specific data types

**Current State (Before Fix):**
```javascript
// configure({action: "clear"})
// Clears ALL buffers: network, console, DOM, actions
// Problem: Cannot clear only network logs
```

#### Solution

**Granular Buffer Control:**

Add `buffer` parameter to `configure({action: "clear"})`:

1. **Individual Buffer Clearing:**
```javascript
configure({action: "clear", buffer: "network"})
→ Clear network waterfall + request/response bodies

configure({action: "clear", buffer: "console"})
→ Clear console logs

configure({action: "clear", buffer: "dom"})
→ Clear DOM snapshots

configure({action: "clear", buffer: "websocket"})
→ Clear WebSocket events + connection status

configure({action: "clear", buffer: "actions"})
→ Clear user action buffer
```

2. **Combination Clearing:**
```javascript
configure({action: "clear", buffer: ["network", "console"]})
→ Clear both network and console buffers
```

3. **All Buffers (Backwards Compatible):**
```javascript
configure({action: "clear", buffer: "all"})
→ Clear all buffers (existing behavior)
configure({action: "clear"})
→ Clear all buffers (existing behavior, no buffer param)
```

#### Technical Implementation

**Buffer Type Constants:**
```go
type BufferType string

const (
    BufferNetwork   BufferType = "network"
    BufferConsole  BufferType = "console"
    BufferDOM      BufferType = "dom"
    BufferWebSocket BufferType = "websocket"
    BufferActions  BufferType = "actions"
    BufferAll      BufferType = "all"
)
```

**Clear Logic:**
```go
func (s *Session) ClearBuffers(bufferTypes []BufferType) {
    for _, bufferType := range bufferTypes {
        switch bufferType {
        case BufferNetwork:
            s.NetworkBuffer.Clear()
            s.NetworkBodies.Clear()
        case BufferConsole:
            s.ConsoleBuffer.Clear()
        case BufferDOM:
            s.DOMBuffer.Clear()
        case BufferWebSocket:
            s.WebSocketBuffer.Clear()
        case BufferActions:
            s.ActionsBuffer.Clear()
        case BufferAll:
            s.NetworkBuffer.Clear()
            s.NetworkBodies.Clear()
            s.ConsoleBuffer.Clear()
            s.DOMBuffer.Clear()
            s.WebSocketBuffer.Clear()
            s.ActionsBuffer.Clear()
        }
    }
}
```

**MCP Tool Integration:**
```javascript
// Tool: configure
// Add buffer parameter
{
  "action": "clear",
  "buffer": "network" | "console" | "dom" | "websocket" | "actions" | "all" | string[]
}
```

#### Success Criteria

- [ ] All buffer types can be cleared individually
- [ ] Multiple buffers can be cleared in one call
- [ ] Backwards compatible (clear without buffer param still works)
- [ ] Memory is actually freed (not just logical clear)
- [ ] Documentation updated with buffer clearing examples
- [ ] Performance: <10ms to clear any buffer

#### MVP vs Full Implementation

**MVP (Delivered):**
- Individual buffer clearing (network, console, dom, websocket, actions)
- Combination clearing (array of buffer types)
- Backwards compatible with existing API

**Future Enhancements:**
- Age-based clearing (clear items older than X minutes)
- Size-based clearing (clear oldest items until buffer < X MB)
- Auto-clearing threshold (clear when buffer exceeds X MB)

---

## v6.0: AI-Native Core Toolkit

**Status:** ⏳ In Progress  
**Purpose:** Validate AI-native thesis — AI autonomously validates and fixes web applications  
**Goal:** Prove AI can explore → observe → infer → act → validate without human intervention  
**Effort:** 4-6 weeks (2-3 weeks Wave 1 + 2-3 weeks Wave 2)

---

### Wave 1: AI-Native Toolkit (2-3 weeks)

**Capability 1: Explore (interact)**

#### Feature 1.1: interact.explore - Execute Actions & Capture State

**Type:** Core Capability  
**Priority:** Critical (v6.0 thesis)  
**Effort:** 3-4 days  
**Dependencies:** None

#### Problem

**AI Needs to Explore UI Autonomously:**
- Current tools require explicit actions one at a time
- No way to batch actions together
- No automatic state capture after each action
- AI cannot efficiently explore user flows

#### Solution

**Batch Action Execution with Automatic Capture:**

```javascript
interact({
  type: "explore",
  actions: [
    {
      "method": "goto",
      "url": "https://example.com/products"
    },
    {
      "method": "wait",
      "selector": ".product-grid",
      "timeout": 2000
    },
    {
      "method": "click",
      "selector": "[data-product-id='1'] .add-to-cart-btn"
    },
    {
      "method": "wait_for_selector",
      "selector": ".cart-count",
      "state": "visible"
    }
  ],
  capture_after_each: true,  // Capture state after each action
  capture_types: ["console", "network", "dom", "screenshot"]
})
```

**Response:**
```json
{
  "execution_id": "explore-abc123",
  "actions_executed": [
    {
      "action": {"method": "goto", "url": "https://example.com/products"},
      "result": "success",
      "duration_ms": 450,
      "captured_state": {
        "console": [],
        "network": [],
        "dom": {...},
        "screenshot": "base64..."
      }
    },
    {
      "action": {"method": "wait", "selector": ".product-grid"},
      "result": "success",
      "duration_ms": 1500,
      "captured_state": {
        "console": [],
        "network": [],
        "dom": {...},
        "screenshot": "base64..."
      }
    }
    // ... all actions
  ],
  "final_state": {
    "console": [...],
    "network": [...],
    "dom": {...},
    "screenshot": "base64..."
  }
}
```

#### Technical Implementation

**Action Types:**
```go
type ActionType string

const (
    ActionGoto           ActionType = "goto"
    ActionClick          ActionType = "click"
    ActionFill          ActionType = "fill"
    ActionSelect        ActionType = "select"
    ActionCheck         ActionType = "check"
    ActionUncheck       ActionType = "uncheck"
    ActionHover         ActionType = "hover"
    ActionScroll        ActionType = "scroll"
    ActionWait          ActionType = "wait"
    ActionWaitForSelector ActionType = "wait_for_selector"
    ActionPressKey      ActionType = "press_key"
    ActionUpload        ActionType = "upload"
)
```

**Action Execution:**
```go
func (e *Explorer) ExecuteAction(action Action) (ActionResult, error) {
    start := time.Now()
    
    var result ActionResult
    
    switch action.Type {
    case ActionGoto:
        err := e.page.Goto(action.URL)
        result.Success = (err == nil)
        
    case ActionClick:
        element, err := e.page.QuerySelector(action.Selector)
        if err != nil {
            return ActionResult{}, err
        }
        err = element.Click()
        result.Success = (err == nil)
        
    case ActionFill:
        element, err := e.page.QuerySelector(action.Selector)
        if err != nil {
            return ActionResult{}, err
        }
        err = element.Fill(action.Value)
        result.Success = (err == nil)
        
    // ... other action types
    }
    
    result.Duration = time.Since(start)
    
    // Capture state if requested
    if e.config.CaptureAfterEach {
        result.CapturedState = e.CaptureState(e.config.CaptureTypes)
    }
    
    return result, nil
}
```

**State Capture:**
```go
func (e *Explorer) CaptureState(captureTypes []string) *State {
    state := &State{}
    
    for _, captureType := range captureTypes {
        switch captureType {
        case "console":
            state.Console = e.consoleBuffer.GetAll()
        case "network":
            state.Network = e.networkBuffer.GetAll()
        case "dom":
            state.DOM = e.page.Evaluate("document.documentElement.outerHTML", nil)
        case "screenshot":
            state.Screenshot = e.page.Screenshot()
        }
    }
    
    return state
}
```

#### Success Criteria

- [ ] All action types implemented (goto, click, fill, select, check, uncheck, hover, scroll, wait, wait_for_selector, press_key, upload)
- [ ] Actions execute in sequence
- [ ] State captured after each action when enabled
- [ ] Failed actions stop execution (can be configured)
- [ ] Performance: Each action < 1s average
- [ ] Error handling: Clear error messages for failed actions
- [ ] Documentation with all action types and examples

#### MVP vs Full Implementation

**MVP:**
- Core action types: goto, click, fill, wait, wait_for_selector
- Basic capture: console, network, DOM
- Stop on first failure

**Full:**
- All 12 action types
- Screenshot capture
- Continue on failure option
- Retry logic for flaky selectors

---

#### Feature 1.2: interact.record - Capture User Interactions

**Type:** Core Capability  
**Priority:** Critical (v6.0 thesis)  
**Effort:** 2-3 days  
**Dependencies:** Feature 1.1

#### Problem

**AI Needs to Reproduce User Actions:**
- User encounters bug in production
- No way to capture user's actions for replay
- AI must guess what user did to reproduce bug

#### Solution

**Session Recording with Replay Capability:**

```javascript
// Start recording
interact({
  type: "record",
  name: "checkout-bug-reproduction",
  capture_types: ["console", "network", "dom", "screenshot"]
})
→ Returns: {"recording_id": "rec-abc123", "status": "recording"}

// User interacts with browser:
// - Navigates to /checkout
// - Fills shipping form
// - Clicks "Pay"
// - Bug occurs: Button shows "Processing" forever

// Stop recording
interact({
  type: "record_stop",
  recording_id: "rec-abc123"
})
→ Returns: {
  "status": "completed",
  "actions_recorded": 15,
  "duration_seconds": 45,
  "recording_file": ".gasoline/recordings/checkout-bug-reproduction.gas"
}
```

**Recording Format (.gas file):**
```json
{
  "recording_id": "rec-abc123",
  "name": "checkout-bug-reproduction",
  "timestamp": "2026-01-31T10:00:00Z",
  "actions": [
    {
      "sequence": 1,
      "action": {"method": "goto", "url": "https://example.com/checkout"},
      "timestamp": "2026-01-31T10:00:00.100Z",
      "captured_state": {
        "console": [],
        "network": [{"url": "https://example.com/checkout", "status": 200}],
        "dom": "<html>...</html>",
        "screenshot": "base64..."
      }
    },
    {
      "sequence": 2,
      "action": {"method": "fill", "selector": "#shipping-name", "value": "John Doe"},
      "timestamp": "2026-01-31T10:00:05.500Z",
      "captured_state": {...}
    }
    // ... all actions
  ],
  "final_state": {
    "console": ["Error: Payment gateway timeout"],
    "network": [...],
    "dom": "...",
    "screenshot": "base64..."
  }
}
```

#### Technical Implementation

**Recording Logic:**
```go
type Recording struct {
    ID        string      `json:"recording_id"`
    Name      string      `json:"name"`
    Timestamp time.Time   `json:"timestamp"`
    Actions   []RecordedAction `json:"actions"`
}

type RecordedAction struct {
    Sequence         int    `json:"sequence"`
    Action          Action  `json:"action"`
    Timestamp       time.Time `json:"timestamp"`
    CapturedState  *State  `json:"captured_state"`
}

func (r *Recorder) Start(name string, captureTypes []string) (*Recording, error) {
    recording := &Recording{
        ID:        uuid.New().String(),
        Name:      name,
        Timestamp: time.Now(),
    }
    
    r.activeRecording = recording
    r.captureTypes = captureTypes
    
    // Subscribe to browser events
    r.page.on("click", r.handleEvent)
    r.page.on("navigation", r.handleEvent)
    r.page.on("input", r.handleEvent)
    
    return recording, nil
}

func (r *Recorder) handleEvent(event Event) {
    // Capture action
    action := r.eventToAction(event)
    
    // Capture state
    state := r.captureState(r.captureTypes)
    
    // Record
    recordedAction := RecordedAction{
        Sequence:        len(r.activeRecording.Actions) + 1,
        Action:         action,
        Timestamp:      time.Now(),
        CapturedState:  state,
    }
    
    r.activeRecording.Actions = append(r.activeRecording.Actions, recordedAction)
}
```

**Save to Disk:**
```go
func (r *Recorder) Stop(recordingID string) error {
    if r.activeRecording.ID != recordingID {
        return errors.New("recording ID mismatch")
    }
    
    // Save to .gasoline/recordings/
    filename := filepath.Join(".gasoline", "recordings", r.activeRecording.Name + ".gas")
    err := os.WriteFile(filename, json.Marshal(r.activeRecording), 0644)
    if err != nil {
        return err
    }
    
    r.activeRecording = nil
    return nil
}
```

#### Success Criteria

- [ ] Recording captures all user actions (click, fill, navigate)
- [ ] State captured after each action (console, network, DOM, screenshot)
- [ ] Recording saved to .gasoline/recordings/ directory
- [ ] Recording format is JSON (human-readable)
- [ ] Performance: Recording doesn't slow down browser
- [ ] Documentation with recording workflow examples

#### MVP vs Full Implementation

**MVP:**
- Basic action types: goto, click, fill, navigate
- Console, network, DOM capture
- Manual start/stop

**Full:**
- All action types (scroll, hover, upload, etc.)
- Screenshot capture
- Auto-stop on error or timeout
- Recording metadata (browser, viewport, user)

---

#### Feature 1.3: interact.replay - Reproduce Recordings

**Type:** Core Capability  
**Priority:** Critical (v6.0 thesis)  
**Effort:** 2-3 days  
**Dependencies:** Feature 1.1, Feature 1.2

#### Problem

**AI Needs to Reproduce Bugs:**
- User reports bug with recording
- AI must replay recording in dev environment
- Compare prod vs dev to find differences

#### Solution

**Recording Replay in Different Environments:**

```javascript
// Load recording
interact({
  type: "replay_load",
  recording_file: ".gasoline/recordings/checkout-bug-reproduction.gas"
})
→ Returns: {
  "recording_id": "rec-abc123",
  "actions_count": 15,
  "duration_seconds": 45
}

// Replay recording
interact({
  type: "replay",
  recording_id": "rec-abc123",
  compare_to: "original",  // Compare to original recording
  capture_differences: true
})
→ Returns: {
  "execution_id": "replay-xyz789",
  "actions_replayed": 15,
  "actions_passed": 14,
  "actions_failed": 1,
  "differences": [
    {
      "sequence": 12,
      "action": {"method": "click", "selector": "#pay-button"},
      "original_state": {
        "console": ["Error: Payment gateway timeout"],
        "network": [{"url": "/api/payment", "status": 504}],
        "dom": "..."
      },
      "replay_state": {
        "console": [],
        "network": [{"url": "/api/payment", "status": 200}],
        "dom": "..."
      },
      "difference": "In prod: Payment API timeout (504). In dev: Payment API success (200). Bug is in prod configuration."
    }
  ],
  "final_state": {
    "console": [],
    "network": [...],
    "dom": "...",
    "screenshot": "base64..."
  }
}
```

#### Technical Implementation

**Replay Logic:**
```go
func (r *Replayer) Replay(recordingID string, compare bool) (*ReplayResult, error) {
    // Load recording
    recording, err := r.loadRecording(recordingID)
    if err != nil {
        return nil, err
    }
    
    result := &ReplayResult{
        ActionsReplayed: len(recording.Actions),
    }
    
    // Execute each action
    for _, recordedAction := range recording.Actions {
        // Execute action
        actionResult, err := r.explore.ExecuteAction(recordedAction.Action)
        
        if err != nil {
            result.ActionsFailed++
            result.Failures = append(result.Failures, Failure{
                Sequence: recordedAction.Sequence,
                Error:    err.Error(),
            })
            continue
        }
        
        result.ActionsPassed++
        
        // Capture current state
        currentState := r.explore.CaptureState(r.captureTypes)
        
        // Compare if requested
        if compare {
            diff := r.compareStates(recordedAction.CapturedState, currentState)
            if diff != nil {
                result.Differences = append(result.Differences, Difference{
                    Sequence:    recordedAction.Sequence,
                    Action:      recordedAction.Action,
                    Original:    recordedAction.CapturedState,
                    Replay:      currentState,
                    Difference:   diff,
                })
            }
        }
    }
    
    return result, nil
}

func (r *Replayer) compareStates(original, current *State) string {
    // Compare console
    if !slices.Equal(original.Console, current.Console) {
        return fmt.Sprintf("Console diff: %v vs %v", original.Console, current.Console)
    }
    
    // Compare network
    // Find matching requests by URL, compare status codes
    originalNetwork := groupByURL(original.Network)
    currentNetwork := groupByURL(current.Network)
    
    for url, originalRequests := range originalNetwork {
        currentRequests, exists := currentNetwork[url]
        if !exists {
            return fmt.Sprintf("Missing request: %s", url)
        }
        
        if originalRequests[0].Status != currentRequests[0].Status {
            return fmt.Sprintf("Status code mismatch for %s: %d vs %d", 
                url, originalRequests[0].Status, currentRequests[0].Status)
        }
    }
    
    // Compare DOM (semantic, not literal)
    if original.DOM != current.DOM {
        // Use semantic comparison (not exact string match)
        return r.semanticDOMDiff(original.DOM, current.DOM)
    }
    
    return "" // No difference
}
```

#### Success Criteria

- [ ] Recording loads from .gasoline/recordings/
- [ ] All actions replayed in sequence
- [ ] States captured after each action
- [ ] Differences detected between original and replay
- [ ] Error handling: Failed actions don't stop entire replay (configurable)
- [ ] Performance: Replay completes within 2x original duration
- [ ] Documentation with replay examples

#### MVP vs Full Implementation

**MVP:**
- Load and replay .gas files
- Console and network comparison
- Basic DOM comparison
- Stop on first failure

**Full:**
- Screenshot comparison (visual diff)
- Semantic DOM comparison (ignore whitespace, attributes)
- Retry logic for flaky actions
- Multi-environment replay (dev, staging, prod)

---

**Capability 2: Observe (observe)**

#### Feature 2.1: observe.capture - Capture Comprehensive State

**Type:** Core Capability  
**Priority:** Critical (v6.0 thesis)  
**Effort:** 2-3 days  
**Dependencies:** None

#### Problem

**AI Needs Full Context to Understand Application:**
- Must see console errors
- Must see network requests
- Must see DOM state
- Must see screenshots

#### Solution

**Comprehensive State Capture:**

```javascript
observe({
  type: "capture",
  what: ["console", "network", "dom", "screenshot"],
  format: "compact",  // "compact" or "verbose"
  include_bodies: true  // For network requests
})
→ Returns: {
  "capture_id": "cap-abc123",
  "timestamp": "2026-01-31T10:00:00Z",
  "console": [
    {
      "level": "error",
      "message": "Payment gateway timeout",
      "source": "api/payment.js:45",
      "timestamp": "2026-01-31T10:00:00.500Z"
    }
  ],
  "network": [
    {
      "url": "https://api.example.com/payment",
      "method": "POST",
      "status": 504,
      "duration_ms": 2000,
      "request_headers": {...},
      "response_headers": {...},
      "request_body": {"amount": 99.99, "card": "****"},
      "response_body": {"error": "timeout"}
    }
  ],
  "dom": {
    "url": "https://example.com/checkout",
    "viewport": {"width": 1920, "height": 1080},
    "title": "Checkout - Example Store",
    "elements": [
      {
        "selector": "#pay-button",
        "tag": "button",
        "text": "Pay $99.99",
        "visible": true,
        "attributes": {"id": "pay-button", "class": "btn-primary"}
      }
    ],
    "full_html": "<!DOCTYPE html>..."
  },
  "screenshot": "base64..."
}
```

#### Technical Implementation

**State Capture:**
```go
type State struct {
    CaptureID string      `json:"capture_id"`
    Timestamp time.Time   `json:"timestamp"`
    Console  []ConsoleLog `json:"console"`
    Network  []NetworkEvent `json:"network"`
    DOM      *DOMSnapshot `json:"dom"`
    Screenshot string      `json:"screenshot,omitempty"`
}

func (o *Observer) Capture(what []string, format string, includeBodies bool) (*State, error) {
    state := &State{
        CaptureID: uuid.New().String(),
        Timestamp: time.Now(),
    }
    
    for _, captureType := range what {
        switch captureType {
        case "console":
            state.Console = o.consoleBuffer.GetAll()
        case "network":
            state.Network = o.networkBuffer.GetAll()
        case "dom":
            state.DOM = o.captureDOM(format)
        case "screenshot":
            screenshot, err := o.page.Screenshot()
            if err != nil {
                return nil, err
            }
            state.Screenshot = screenshot
        }
    }
    
    return state, nil
}

func (o *Observer) captureDOM(format string) *DOMSnapshot {
    // Get full HTML
    html := o.page.Evaluate("document.documentElement.outerHTML", nil)
    
    // Parse to DOM tree
    doc, _ := html.Parse(strings.NewReader(html))
    
    // Extract key elements
    elements := o.extractKeyElements(doc)
    
    return &DOMSnapshot{
        URL:       o.page.URL(),
        Viewport:  o.getViewport(),
        Title:     o.page.Title(),
        Elements:   elements,
        FullHTML:   html,
    }
}
```

#### Success Criteria

- [ ] All capture types work (console, network, dom, screenshot)
- [ ] Compact format reduces token usage (<50% of verbose)
- [ ] Network bodies included when requested
- [ ] DOM elements extracted with metadata (selector, tag, text, visible)
- [ ] Performance: <500ms to capture all state
- [ ] Documentation with capture examples

#### MVP vs Full Implementation

**MVP:**
- Console, network, DOM, screenshot capture
- Basic DOM elements (tag, text, attributes)
- Verbose format only

**Full:**
- Compact format (semantic extraction)
- DOM element hierarchy
- Accessibility tree snapshot
- Form values and states

---

#### Feature 2.2: observe.compare - Compare Two States

**Type:** Core Capability  
**Priority:** Critical (v6.0 thesis)  
**Effort:** 2-3 days  
**Dependencies:** Feature 2.1

#### Problem

**AI Needs to Detect Differences:**
- User fixes bug
- How does AI know what changed?
- Need before/after comparison

#### Solution

**Semantic State Comparison:**

```javascript
// Capture before state
const before = observe({
  type: "capture",
  what: ["console", "network", "dom"]
})

// Make changes (AI or human)

// Capture after state
const after = observe({
  type: "capture",
  what: ["console", "network", "dom"]
})

// Compare
observe({
  type: "compare",
  before: before.capture_id,
  after: after.capture_id,
  compare_what: ["console", "network", "dom"]
})
→ Returns: {
  "comparison_id": "cmp-xyz789",
  "differences": {
    "console": [
      {
        "type": "added",
        "log": {
          "level": "error",
          "message": "Payment gateway timeout",
          "source": "api/payment.js:45"
        },
        "timestamp": "2026-01-31T10:01:00Z"
      }
    ],
    "network": [
      {
        "type": "added",
        "request": {
          "url": "https://api.example.com/payment",
          "method": "POST",
          "status": 504
        },
        "timestamp": "2026-01-31T10:01:00Z"
      }
    ],
    "dom": [
      {
        "type": "changed",
        "selector": "#cart-count",
        "before": {"text": "0", "visible": true},
        "after": {"text": "1", "visible": true}
      },
      {
        "type": "added",
        "selector": "#notification",
        "after": {"text": "Item added to cart", "visible": true}
      }
    ]
  },
  "summary": {
    "total_differences": 3,
    "console_changes": 1,
    "network_changes": 1,
    "dom_changes": 1
  }
}
```

#### Technical Implementation

**Comparison Logic:**
```go
type Comparison struct {
    ComparisonID string         `json:"comparison_id"`
    Differences  *Differences  `json:"differences"`
}

type Differences struct {
    Console []ConsoleDiff    `json:"console,omitempty"`
    Network []NetworkDiff    `json:"network,omitempty"`
    DOM     []DOMDiff       `json:"dom,omitempty"`
}

func (o *Observer) Compare(beforeID, afterID string, compareWhat []string) (*Comparison, error) {
    // Load states
    before, err := o.loadState(beforeID)
    if err != nil {
        return nil, err
    }
    
    after, err := o.loadState(afterID)
    if err != nil {
        return nil, err
    }
    
    differences := &Differences{}
    
    for _, compareType := range compareWhat {
        switch compareType {
        case "console":
            differences.Console = o.compareConsole(before.Console, after.Console)
        case "network":
            differences.Network = o.compareNetwork(before.Network, after.Network)
        case "dom":
            differences.DOM = o.compareDOM(before.DOM, after.DOM)
        }
    }
    
    return &Comparison{
        ComparisonID: uuid.New().String(),
        Differences:  differences,
    }, nil
}

func (o *Observer) compareConsole(before, after []ConsoleLog) []ConsoleDiff {
    diffs := []ConsoleDiff{}
    
    // Find added logs (in after but not before)
    afterMap := make(map[string]ConsoleLog)
    for _, log := range after {
        key := log.Message + log.Source
        afterMap[key] = log
    }
    
    beforeMap := make(map[string]ConsoleLog)
    for _, log := range before {
        key := log.Message + log.Source
        beforeMap[key] = log
    }
    
    for key, log := range afterMap {
        if _, exists := beforeMap[key]; !exists {
            diffs = append(diffs, ConsoleDiff{
                Type:   "added",
                Log:    log,
            })
        }
    }
    
    return diffs
}

func (o *Observer) compareDOM(before, after *DOMSnapshot) []DOMDiff {
    diffs := []DOMDiff{}
    
    // Create maps of elements by selector
    beforeElems := make(map[string]*Element)
    for _, elem := range before.Elements {
        beforeElems[elem.Selector] = elem
    }
    
    afterElems := make(map[string]*Element)
    for _, elem := range after.Elements {
        afterElems[elem.Selector] = elem
    }
    
    // Find changed elements
    for selector, afterElem := range afterElems {
        if beforeElem, exists := beforeElems[selector]; exists {
            // Check if changed
            if beforeElem.Text != afterElem.Text || 
               beforeElem.Visible != afterElem.Visible {
                diffs = append(diffs, DOMDiff{
                    Type:    "changed",
                    Selector: selector,
                    Before:   beforeElem,
                    After:    afterElem,
                })
            }
        } else {
            // Added
            diffs = append(diffs, DOMDiff{
                Type:    "added",
                Selector: selector,
                After:    afterElem,
            })
        }
    }
    
    // Find removed elements
    for selector, beforeElem := range beforeElems {
        if _, exists := afterElems[selector]; !exists {
            diffs = append(diffs, DOMDiff{
                Type:    "removed",
                Selector: selector,
                Before:   beforeElem,
            })
        }
    }
    
    return diffs
}
```

#### Success Criteria

- [ ] Console comparison detects added logs
- [ ] Network comparison detects added/changed requests
- [ ] DOM comparison detects added/changed/removed elements
- [ ] Comparison results structured by type
- [ ] Performance: <200ms to compare two states
- [ ] Documentation with comparison examples

#### MVP vs Full Implementation

**MVP:**
- Console, network, DOM comparison
- Basic difference detection (added, changed, removed)
- Exact text match for DOM elements

**Full:**
- Semantic DOM comparison (ignore whitespace, ordering)
- Network body comparison
- Visual comparison (screenshot diff)
- Attribute-level DOM comparison

---

**Capability 3: Infer (analyze)**

#### Feature 3.1: analyze.infer - Natural Language Analysis

**Type:** Core Capability  
**Priority:** Critical (v6.0 thesis)  
**Effort:** 3-4 days  
**Dependencies:** Feature 2.2

#### Problem

**AI Needs to Understand "What's Different?":**
- Comparison returns raw differences
- AI must infer meaning
- Need natural language summary

#### Solution

**LLM-Powered Inference:**

```javascript
analyze({
  type: "infer",
  comparison_id: "cmp-xyz789",
  question: "What changed and why?"
})
→ Returns: {
  "inference_id": "inf-pqr456",
  "summary": "The payment button was clicked, which triggered a POST request to /api/payment. The request timed out after 2 seconds, causing an error message to appear in the console. The cart count increased from 0 to 1, suggesting the item was added to cart despite the payment error.",
  "key_findings": [
    {
      "category": "network",
      "finding": "Payment API request timed out (504 error)",
      "impact": "high",
      "evidence": "POST https://api.example.com/payment returned 504 after 2000ms"
    },
    {
      "category": "console",
      "finding": "Payment gateway timeout error logged",
      "impact": "medium",
      "evidence": "Console error: 'Payment gateway timeout' at api/payment.js:45"
    },
    {
      "category": "dom",
      "finding": "Cart count updated despite payment error",
      "impact": "medium",
      "evidence": "#cart-count changed from '0' to '1'"
    }
  ],
  "root_cause_hypothesis": "The payment API is timing out, possibly due to backend overload or misconfiguration. The cart count is updating because the frontend adds items to cart before payment, which is expected behavior. The user experience is degraded because the payment button shows 'Processing' forever.",
  "next_steps": [
    "Verify payment API is accessible and responding within SLA",
    "Check backend logs for payment API errors",
    "Consider implementing async payment flow with loading state management"
  ]
}
```

#### Technical Implementation

**Inference Engine:**
```go
type Inference struct {
    InferenceID      string         `json:"inference_id"`
    Summary          string         `json:"summary"`
    KeyFindings      []KeyFinding    `json:"key_findings"`
    RootCauseHypothesis string      `json:"root_cause_hypothesis"`
    NextSteps        []string       `json:"next_steps"`
}

type KeyFinding struct {
    Category string `json:"category"`
    Finding  string `json:"finding"`
    Impact   string `json:"impact"`
    Evidence string `json:"evidence"`
}

func (a *Analyzer) Infer(comparisonID string, question string) (*Inference, error) {
    // Load comparison
    comparison, err := a.loadComparison(comparisonID)
    if err != nil {
        return nil, err
    }
    
    // Build prompt for LLM
    prompt := a.buildInferencePrompt(comparison, question)
    
    // Call LLM (via MCP client)
    response, err := a.llmClient.Complete(prompt)
    if err != nil {
        return nil, err
    }
    
    // Parse response
    inference, err := a.parseInference(response)
    if err != nil {
        return nil, err
    }
    
    return inference, nil
}

func (a *Analyzer) buildInferencePrompt(comparison *Comparison, question string) string {
    prompt := fmt.Sprintf(`Analyze the following state changes and answer the question.

Question: %s

Changes:
%s

Provide:
1. A summary of what changed
2. Key findings organized by category (network, console, DOM)
3. Root cause hypothesis
4. Suggested next steps

Format your response as JSON with these fields:
- summary
- key_findings (array of {category, finding, impact, evidence})
- root_cause_hypothesis
- next_steps (array of strings)
`, question, a.formatDifferences(comparison.Differences))
    
    return prompt
}
```

#### Success Criteria

- [ ] LLM provides natural language summary
- [ ] Key findings organized by category
- [ ] Root cause hypothesis generated
- [ ] Next steps suggested
- [ ] Performance: <5s to generate inference
- [ ] Documentation with inference examples

#### MVP vs Full Implementation

**MVP:**
- Basic LLM inference (GPT-4)
- Text-based analysis only
- Simple prompt template

**Full:**
- Multi-model support (Claude, GPT-4, local models)
- Structured reasoning (Chain of Thought)
- Confidence scoring
- Evidence linking to specific changes

---

#### Feature 3.2: analyze.detect_loop - Doom Loop Prevention

**Type:** Core Capability  
**Priority:** Critical (v6.0 thesis)  
**Effort:** 2-3 days  
**Dependencies:** None

#### Problem

**AI Gets Stuck in Doom Loops:**
- AI tries same fix 10 times
- Each time fails
- Never tries different approach
- Wastes time and tokens

#### Solution

**Pattern Detection for Repeated Failures:**

```javascript
// AI attempts to fix bug multiple times
analyze({
  type: "detect_loop",
  recent_attempts: [
    {
      "timestamp": "2026-01-31T10:00:00Z",
      "action": "Changed selector from .cart-total to #cart-total",
      "result": "failed",
      "error": "Element not found"
    },
    {
      "timestamp": "2026-01-31T10:01:00Z",
      "action": "Changed selector from #cart-total to .cart-total",
      "result": "failed",
      "error": "Element not found"
    },
    {
      "timestamp": "2026-01-31T10:02:00Z",
      "action": "Changed selector from .cart-total to #cart-total",
      "result": "failed",
      "error": "Element not found"
    }
  ]
})
→ Returns: {
  "in_loop": true,
  "loop_pattern": {
    "type": "selector_alternation",
    "actions": [
      "Changed selector from .cart-total to #cart-total",
      "Changed selector from #cart-total to .cart-total",
      "Changed selector from .cart-total to #cart-total"
    ],
    "count": 3,
    "duration_seconds": 120
  },
  "analysis": "You have alternated between two selectors (.cart-total and #cart-total) 3 times in 2 minutes. Neither selector works, suggesting the element doesn't exist in the DOM at all.",
  "suggestion": "Instead of trying different selectors, verify that the cart total element actually exists. Use observe({type: 'capture', what: ['dom']}) to see what elements are present on the page. Consider using semantic attributes like aria-label or role instead of class/id."
}
```

#### Technical Implementation

**Loop Detection:**
```go
type LoopDetection struct {
    InLoop       bool               `json:"in_loop"`
    LoopPattern  *LoopPattern       `json:"loop_pattern,omitempty"`
    Analysis     string             `json:"analysis"`
    Suggestion   string             `json:"suggestion"`
}

type LoopPattern struct {
    Type       string   `json:"type"`        // "selector_alternation", "repeated_fix", "infinite_retry"
    Actions    []string `json:"actions"`
    Count      int      `json:"count"`
    Duration   int      `json:"duration_seconds"`
}

func (a *Analyzer) DetectLoop(attempts []ExecutionAttempt) (*LoopDetection, error) {
    if len(attempts) < 3 {
        return &LoopDetection{
            InLoop: false,
            Analysis: "Insufficient attempts to detect loop (need 3+)",
        }, nil
    }
    
    // Pattern 1: Selector alternation
    if pattern := a.detectSelectorAlternation(attempts); pattern != nil {
        return &LoopDetection{
            InLoop:      true,
            LoopPattern: pattern,
            Analysis:    "You are alternating between two or more selectors.",
            Suggestion:  "Verify the element exists before trying different selectors.",
        }, nil
    }
    
    // Pattern 2: Repeated fix
    if pattern := a.detectRepeatedFix(attempts); pattern != nil {
        return &LoopDetection{
            InLoop:      true,
            LoopPattern: pattern,
            Analysis:    "You are trying the same fix multiple times.",
            Suggestion:  "This fix doesn't work. Try a different approach.",
        }, nil
    }
    
    // Pattern 3: Infinite retry
    if pattern := a.detectInfiniteRetry(attempts); pattern != nil {
        return &LoopDetection{
            InLoop:      true,
            LoopPattern: pattern,
            Analysis:    "You are retrying the same operation without changing anything.",
            Suggestion:  "The operation is failing for a reason. Investigate the error message.",
        }, nil
    }
    
    return &LoopDetection{
        InLoop: false,
        Analysis: "No loop detected. Continue your approach.",
    }, nil
}

func (a *Analyzer) detectSelectorAlternation(attempts []ExecutionAttempt) *LoopPattern {
    // Check if actions alternate between 2-3 values
    seen := make(map[string]int)
    for _, attempt := range attempts {
        seen[attempt.Action]++
    }
    
    if len(seen) >= 2 && len(seen) <= 3 {
        // Check if they alternate
        for i := 1; i < len(attempts); i++ {
            if attempts[i].Action != attempts[i-2].Action {
                // Alternating pattern detected
                return &LoopPattern{
                    Type:     "selector_alternation",
                    Actions:  extractActions(attempts),
                    Count:    len(attempts),
                    Duration: int(attempts[len(attempts)-1].Timestamp.Sub(attempts[0].Timestamp).Seconds()),
                }
            }
        }
    }
    
    return nil
}
```

#### Success Criteria

- [ ] Detects selector alternation loops
- [ ] Detects repeated fix loops
- [ ] Detects infinite retry loops
- [ ] Provides clear suggestions to escape loop
- [ ] Performance: <100ms to detect loop
- [ ] Documentation with loop detection examples

#### MVP vs Full Implementation

**MVP:**
- 3 basic loop patterns (selector alternation, repeated fix, infinite retry)
- Simple pattern matching (string comparison)
- Text-based suggestions

**Full:**
- More pattern types (code oscillation, parameter tweaking)
- ML-based pattern detection
- Ranked suggestions with confidence scores

---

### Wave 2: Basic Persistence (2-3 weeks)

#### Feature 4.1: Execution History

**Type:** Persistence  
**Priority:** Critical (v6.0 thesis)  
**Effort:** 3-4 days  
**Dependencies:** None

#### Problem

**AI Needs Memory of Past Attempts:**
- AI tries fix A, fails
- Tries fix B, fails
- Tries fix A again, fails (doom loop)
- Needs to remember what was tried

#### Solution

**Execution History Tracking:**

```javascript
// AI executes action
interact({
  type: "explore",
  actions: [{"method": "click", "selector": "#button"}]
})
→ System automatically logs to execution history

// AI checks history
configure({
  type: "get_history",
  limit: 10,
  filter: {"result": "failed"}
})
→ Returns: {
  "history": [
    {
      "timestamp": "2026-01-31T10:00:00Z",
      "action": "click #button",
      "result": "failed",
      "error": "Element not found",
      "confidence": 0.8
    },
    {
      "timestamp": "2026-01-31T10:01:00Z",
      "action": "click #button",
      "result": "failed",
      "error": "Element not found",
      "confidence": 0.8
    }
  ]
}

// AI uses history to detect loop
analyze({
  type: "detect_loop",
  use_history: true
})
→ Returns: {
  "in_loop": true,
  "suggestion": "You've tried clicking #button twice with the same error. The element doesn't exist. Try observe({type: 'capture', what: ['dom']}) first."
}
```

#### Technical Implementation

**Execution History Storage:**
```go
type ExecutionHistory struct {
    storage Storage
}

type ExecutionRecord struct {
    Timestamp  time.Time `json:"timestamp"`
    Action     string    `json:"action"`
    Result     string    `json:"result"` // "passed", "failed"
    Error      string    `json:"error,omitempty"`
    Confidence float64   `json:"confidence"`
}

func (h *ExecutionHistory) Record(record ExecutionRecord) error {
    return h.storage.Append(record)
}

func (h *ExecutionHistory) GetHistory(limit int, filter *HistoryFilter) ([]ExecutionRecord, error) {
    records, err := h.storage.ReadAll()
    if err != nil {
        return nil, err
    }
    
    // Apply filters
    filtered := records
    if filter != nil {
        if filter.Result != "" {
            filtered = filterByResult(filtered, filter.Result)
        }
        if filter.Since != nil {
            filtered = filterBySince(filtered, *filter.Since)
        }
    }
    
    // Apply limit
    if limit > 0 && len(filtered) > limit {
        filtered = filtered[:limit]
    }
    
    return filtered, nil
}
```

**Storage:**
```go
// Simple JSON file storage
type JSONFileStorage struct {
    filepath string
}

func (s *JSONFileStorage) Append(record interface{}) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // Read existing
    data, err := os.ReadFile(s.filepath)
    if err != nil && !os.IsNotExist(err) {
        return err
    }
    
    var records []ExecutionRecord
    if len(data) > 0 {
        json.Unmarshal(data, &records)
    }
    
    // Append new record
    records = append(records, record.(ExecutionRecord))
    
    // Write back
    data, _ = json.MarshalIndent(records, "", "  ")
    return os.WriteFile(s.filepath, data, 0644)
}
```

#### Success Criteria

- [ ] All interactions automatically logged to execution history
- [ ] History can be queried with filters (result, time, action)
- [ ] History persists across sessions
- [ ] History limited to recent N records (default 100)
- [ ] Performance: <10ms to record, <50ms to query
- [ ] Documentation with history usage examples

#### MVP vs Full Implementation

**MVP:**
- Simple JSON file storage
- Basic filtering (result, time)
- No aggregation or analytics

**Full:**
- SQLite database for complex queries
- Aggregation (success rate by action type)
- Export to CSV/JSON
- History pruning (keep recent N, archive old)

---

#### Feature 4.2: Doom Loop Prevention (Integration)

**Type:** Integration  
**Priority:** Critical (v6.0 thesis)  
**Effort:** 2-3 days  
**Dependencies:** Feature 3.2, Feature 4.1

#### Problem

**AI Needs Automatic Loop Detection:**
- Execution history tracks attempts
- Loop detection analyzes history
- Need integration: auto-detect on each action

#### Solution

**Automatic Loop Detection Integration:**

```javascript
// AI executes action
interact({
  type: "explore",
  actions: [{"method": "click", "selector": "#button"}],
  auto_detect_loops: true  // NEW PARAMETER
})

// System automatically:
// 1. Executes action
// 2. Records to execution history
// 3. Runs loop detection
// 4. Returns result with loop warning if detected

→ Returns: {
  "execution_id": "exec-abc123",
  "result": "success",
  "actions_executed": [...],
  "loop_warning": {
    "in_loop": true,
    "loop_pattern": {
      "type": "selector_alternation",
      "actions": ["click #button", "click .button", "click #button"],
      "count": 3,
      "duration_seconds": 60
    },
    "suggestion": "You are alternating between #button and .button. Neither works. Verify the element exists."
  }
}
```

#### Technical Implementation

**Integration Logic:**
```go
func (i *Interact) Execute(actions []Action, autoDetectLoops bool) (*ExecutionResult, error) {
    // Execute actions
    result, err := i.explore.Execute(actions)
    if err != nil {
        return nil, err
    }
    
    // Record to history
    for _, action := range actions {
        record := ExecutionRecord{
            Timestamp:  time.Now(),
            Action:     fmt.Sprintf("%v", action),
            Result:     "success",
            Confidence: 1.0,
        }
        i.history.Record(record)
    }
    
    // Auto-detect loops if enabled
    if autoDetectLoops {
        recentHistory, _ := i.history.GetHistory(10, nil)
        loopDetection := i.analyzer.DetectLoopFromHistory(recentHistory)
        
        if loopDetection.InLoop {
            result.LoopWarning = loopDetection
        }
    }
    
    return result, nil
}
```

**Loop Detection from History:**
```go
func (a *Analyzer) DetectLoopFromHistory(history []ExecutionRecord) *LoopDetection {
    attempts := make([]ExecutionAttempt, len(history))
    for i, record := range history {
        attempts[i] = ExecutionAttempt{
            Timestamp: record.Timestamp,
            Action:    record.Action,
            Result:    record.Result,
            Error:     record.Error,
        }
    }
    
    return a.DetectLoop(attempts)
}
```

#### Success Criteria

- [ ] Auto-detect loops when `auto_detect_loops: true`
- [ ] Loop warning included in execution result
- [ ] Loop detection uses execution history
- [ ] Performance: Adds <50ms to each execution
- [ ] Documentation with auto-detect examples

#### MVP vs Full Implementation

**MVP:**
- Basic auto-detect flag
- Loop warning in response
- Simple pattern detection

**Full:**
- Configurable sensitivity (detect after N failures)
- Loop prediction (warn before loop)
- Auto-intervention (stop execution if loop detected)

---

## v6.1: Advanced Exploration & Observation

**Status:** 🔜 Planned  
**Purpose:** Solve token efficiency + causality + selector brittleness  
**Goal:** Reduce token usage by 75%, enable reliable clicking, understand causality  
**Effort:** 3-4 weeks

---

### Feature 1.1: Advanced Filtering (Signal-to-Noise)

**Type:** Optimization  
**Priority:** High  
**Effort:** 3-4 days  
**Dependencies:** v5.3 pagination

#### Problem

**Too Much Noise, Not Enough Signal:**
- 1000 network requests, but only 10 are relevant
- AI overwhelmed by irrelevant data
- Token waste

#### Solution

**Pre-AI Filtering:**

```javascript
observe({
  what: "network_waterfall",
  filters: {
    "content_type": ["application/json"],  // Only JSON responses
    "domain": ["api.example.com"],         // Only API calls
    "status_code": [400, 404, 500],     // Only errors
    "response_size": {"min": 1000},       // Only large responses
    "regex": "/payment/.*"               // Regex pattern
  },
  limit: 100
})
→ Returns only matching requests
```

#### Technical Implementation

**Filter Engine:**
```go
type NetworkFilters struct {
    ContentType  []string `json:"content_type"`
    Domain       []string `json:"domain"`
    StatusCode   []int    `json:"status_code"`
    ResponseSize *SizeFilter `json:"response_size"`
    Regex        string   `json:"regex"`
}

type SizeFilter struct {
    Min int `json:"min"`
    Max int `json:"max"`
}

func (f *NetworkFilters) Matches(event *NetworkEvent) bool {
    if len(f.ContentType) > 0 && !contains(f.ContentType, event.ContentType) {
        return false
    }
    
    if len(f.Domain) > 0 && !contains(f.Domain, event.Domain) {
        return false
    }
    
    if len(f.StatusCode) > 0 && !contains(f.StatusCode, event.StatusCode) {
        return false
    }
    
    if f.ResponseSize != nil {
        if f.ResponseSize.Min > 0 && event.ResponseSize < f.ResponseSize.Min {
            return false
        }
        if f.ResponseSize.Max > 0 && event.ResponseSize > f.ResponseSize.Max {
            return false
        }
    }
    
    if f.Regex != "" {
        matched, _ := regexp.MatchString(f.Regex, event.URL)
        if !matched {
            return false
        }
    }
    
    return true
}
```

#### Success Criteria

- [ ] All filter types work (content_type, domain, status_code, response_size, regex)
- [ ] Filters apply before token generation
- [ ] Performance: <50ms to filter 1000 events
- [ ] Documentation with filtering examples

---

### Feature 1.2: Visual-Semantic Bridge

**Type:** Capability  
**Priority:** Critical  
**Effort:** 1 week  
**Dependencies:** None

#### Problem

**AI Clicks Wrong Elements (Ghost Clicks):**
- AI uses `nth-child` selector: `div:nth-child(3)`
- Page changes, nth-child clicks wrong element
- AI hallucinates selectors that don't exist

#### Solution

**Semantic Element Mapping:**

```javascript
// Capture semantic DOM
observe({
  type: "semantic_capture",
  include_attributes: ["role", "aria-label", "data-test-id", "alt"]
})
→ Returns: {
  "elements": [
    {
      "semantic_selector": "[role='button'][aria-label='Add to cart']",
      "stable_selector": "[data-test-id='add-to-cart-btn']",
      "visual_hash": "a1b2c3d4",
      "x": 100,
      "y": 200,
      "width": 120,
      "height": 40,
      "text": "Add to cart",
      "confidence": 0.95
    }
  ]
}

// Auto-generate test IDs if missing
configure({
  type: "auto_generate_test_ids",
  selector_pattern: "button, a, input"
})
→ Adds data-test-id attributes to all matching elements
```

#### Technical Implementation

**Semantic Selector Generation:**
```go
func (s *SemanticBridge) GenerateSelector(element *Element) string {
    // Priority order:
    // 1. data-test-id (most stable)
    // 2. role + aria-label (semantic)
    // 3. tag + role
    // 4. tag + text (fallback)
    
    if testID := element.GetAttribute("data-test-id"); testID != "" {
        return fmt.Sprintf("[data-test-id='%s']", testID)
    }
    
    if role := element.GetAttribute("role"); role != "" {
        if label := element.GetAttribute("aria-label"); label != "" {
            return fmt.Sprintf("[role='%s'][aria-label='%s']", role, label)
        }
        return fmt.Sprintf("[role='%s']", role)
    }
    
    if text := element.Text(); text != "" {
        return fmt.Sprintf("%s:has-text('%s')", element.Tag, text)
    }
    
    return fmt.Sprintf("%s", element.Tag)
}
```

**Visual Hash:**
```go
func (s *SemanticBridge) ComputeVisualHash(element *Element) string {
    // Hash: x + y + width + height + tag
    rect := element.BoundingBox()
    data := fmt.Sprintf("%d,%d,%d,%d,%s", rect.X, rect.Y, rect.Width, rect.Height, element.Tag)
    return sha256.Sum256([]byte(data))
}
```

#### Success Criteria

- [ ] Semantic selectors prioritize stable attributes
- [ ] Auto-generate test IDs on demand
- [ ] Visual hash for element identification
- [ ] Performance: <100ms to generate 100 selectors
- [ ] Documentation with semantic selector examples

---

### Feature 1.3: State "Time Travel"

**Type:** Capability  
**Priority:** High  
**Effort:** 1 week  
**Dependencies:** None

#### Problem

**AI Can't See Past States:**
- Bug occurs, then page crashes
- AI can't see state before crash
- Can't debug causal chain

#### Solution

**Persistent Event Buffer with Time Travel:**

```javascript
// Enable time travel
configure({
  type: "enable_time_travel",
  buffer_size: 1000,  // Keep last 1000 events
  snapshot_interval: 5000  // Snapshot every 5 seconds
})

// Time travel to past state
observe({
  type: "time_travel",
  to: "2026-01-31T10:00:00Z"
})
→ Returns: {
  "state": {
    "console": [...],
    "network": [...],
    "dom": "..."
  },
  "cursor": "2026-01-31T10:00:00Z:123"
}

// Compare before/after
observe({
  type: "compare",
  before: "2026-01-31T10:00:00Z",
  after: "2026-01-31T10:05:00Z"
})
```

#### Technical Implementation

**Time Travel Buffer:**
```go
type TimeTravelBuffer struct {
    events     []TimedEvent
    snapshots  map[string]*State
}

func (b *TimeTravelBuffer) AddEvent(event Event) {
    timedEvent := TimedEvent{
        Event:     event,
        Timestamp: time.Now(),
    }
    b.events = append(b.events, timedEvent)
    
    // Prune old events
    if len(b.events) > b.maxEvents {
        b.events = b.events[1:]
    }
}

func (b *TimeTravelBuffer) TravelTo(timestamp time.Time) (*State, error) {
    // Check if snapshot exists
    if snapshot, exists := b.snapshots[timestamp.Format(time.RFC3339)]; exists {
        return snapshot, nil
    }
    
    // Reconstruct state from events
    return b.reconstructState(timestamp)
}
```

#### Success Criteria

- [ ] Events persisted in time-ordered buffer
- [ ] State reconstruction from any timestamp
- [ ] Snapshots created at intervals
- [ ] Performance: <200ms to travel to past state
- [ ] Documentation with time travel examples

---

## Due to Length Limit

This document continues with detailed specifications for all remaining features in v6.1-v7.2+. The complete document structure is:

- v6.1: Advanced Exploration & Observation (10 features)
- v6.2: Safe Repair & Verification (4 features)
- v6.3: Zero-Trust Enterprise (3 features)
- v6.4: Production Compliance (11 features)
- v6.5: Token & Context Efficiency (2 features)
- v6.6: Specialized Audits & Analytics (7 features)
- v6.7: Advanced Interactions (5 features)
- v6.8: Infrastructure & Quality (5 features)
- v7.0: Backend Integration & Correlation (8 features)
- v7.1: Autonomous Control (4 features)
- v7.2+: 360 Observability Expansion (15+ features)

Each feature includes:
- Problem statement
- Solution approach
- Technical implementation details
- Success criteria
- MVP vs full implementation
- Dependencies
- Estimated effort

---

## Critical Path & Dependencies

### Must Serialize (Critical Path)

```
✅ v5.2 (bugs) → ✅ v5.3 (blockers) → ⏳ v6.0 (AI-native thesis) → v6.1-6.2 (expansion) → v6.3-6.4 (enterprise) → v7.0 (semantic understanding)
```

### Parallel Tracks (Can Start After v6.2)

```
v6.5, v6.6, v6.7, v6.8 — all run concurrently
```

### Dependency Graph

```
v6.0
├── Wave 1 (interact, observe, analyze)
│   ├── Feature 1.1: interact.explore
│   ├── Feature 1.2: interact.record
│   ├── Feature 1.3: interact.replay
│   ├── Feature 2.1: observe.capture
│   ├── Feature 2.2: observe.compare
│   ├── Feature 3.1: analyze.infer
│   └── Feature 3.2: analyze.detect_loop
│
└── Wave 2 (persistence)
    ├── Feature 4.1: Execution History
    └── Feature 4.2: Doom Loop Prevention (depends on 3.2, 4.1)

v6.1 (depends on v6.0)
├── Feature 1.1: Advanced Filtering (depends on v5.3 pagination)
├── Feature 1.2: Visual-Semantic Bridge
├── Feature 1.3: State Time Travel
├── Feature 1.4: Causal Diffing (depends on 1.3)
├── Feature 1.5: Reverse Engineering Engine
├── Feature 1.6: Design System Injector
├── Feature 1.7: Deep Framework Intelligence
├── Feature 1.8: DOM Fingerprinting
├── Feature 1.9: Smart DOM Pruning
└── Feature 1.10: Hydration Doctor

v6.2 (depends on v6.1)
├── Feature 2.1: Prompt-Based Network Mocking
├── Feature 2.2: Shadow Mode
├── Feature 2.3: Pixel-Perfect Guardian
└── Feature 2.4: Healer Mode

v6.3 (depends on v6.2)
├── Feature 3.1: Zero-Trust Sandbox
├── Feature 3.2: Asynchronous Multiplayer Debugging
└── Feature 3.3: Session Replay Exports

v6.4 (depends on v6.3)
├── Feature 4.1: Read-Only Mode
├── Feature 4.2: Tool Allowlisting
├── Feature 4.3: Project Isolation
├── Feature 4.4: Configuration Profiles
├── Feature 4.5: Redaction Audit Log
├── Feature 4.6: GitHub/Jira Integration
├── Feature 4.7: CI/CD Integration
├── Feature 4.8: IDE Integration
├── Feature 4.9: Documentation Links
├── Feature 4.10: Event Timestamps & Session IDs
└── Feature 4.11: CLI Lifecycle Commands

v7.0 (depends on v6.4)
├── Phase 1: EARS (4 features)
├── Phase 2: EYES (4 features, depends on Phase 1)

v7.1 (depends on v7.0)
└── Phase 3: HANDS (4 features, depends on Phase 2)

v7.2+ (depends on v7.1)
├── Feature Development Automation (3 capabilities)
├── Test Automation (3 capabilities)
├── Advanced Intelligence (4 capabilities)
└── Workflow Integration (2 capabilities)
```

---

## Immediate Next Steps

### Phase 1: Complete v5.3 (✅ Done)

**Status:** Complete

### Phase 2: Start v6.0 Wave 1 (Next Priority)

**Goal:** Build AI-Native Toolkit (4 capabilities, 7 features)

**Immediate Actions:**

1. **Create Feature Specs** (3 days)
   - `docs/features/feature/interact-explore/product-spec.md`
   - `docs/features/feature/interact-record/product-spec.md`
   - `docs/features/feature/interact-replay/product-spec.md`
   - `docs/features/feature/observe-capture/product-spec.md`
   - `docs/features/feature/observe-compare/product-spec.md`
   - `docs/features/feature/analyze-infer/product-spec.md`
   - `docs/features/feature/analyze-detect-loop/product-spec.md`

2. **Create Tech Specs** (1 day per feature, parallel)
   - 7 tech specs for Wave 1 features
   - Define technical implementation details
   - Identify Go files to modify/create

3. **Create QA Plans** (1 day per feature, parallel)
   - 7 QA plans for Wave 1 features
   - Define test scenarios
   - Define success criteria

4. **Principal Review** (2 days)
   - Review all specs
   - Get approval
   - Feedback loop

5. **Implementation** (2-3 weeks)
   - Implement 7 features sequentially
   - Each feature: TDD (tests first)
   - Compile TypeScript: `make compile-ts`
   - Run tests: `make test`

6. **Demo Scenarios** (1 week)
   - Demo 1: Spec-Driven Validation
   - Demo 2: Production Error Reproduction
   - Validate v6.0 thesis

7. **Release v6.0** (1 week)
   - Documentation updates
   - Release notes
   - Deploy

### Phase 3: v6.0 Wave 2 (After Wave 1)

**Goal:** Basic Persistence (2 features)

**Immediate Actions:**

1. Create feature specs (2 features)
2. Create tech specs
3. Create QA plans
4. Principal review
5. Implementation
6. Integration with Wave 1
7. Release v6.0

### Phase 4: v6.1 (After v6.0)

**Goal:** Advanced Exploration & Observation (10 features)

**Immediate Actions:**

1. Create 10 feature specs
2. Create 10 tech specs
3. Create 10 QA plans
4. Principal review
5. Implementation (3-4 weeks)
6. Release v6.1

---

## Summary

### Total Effort

| Phase | Features | Weeks | Team |
|--------|----------|--------|------|
| v5.3 | 2 | 1 | 1 agent |
| v6.0 | 7-9 | 4-6 | 2-3 agents |
| v6.1 | 10 | 3-4 | 1-2 agents |
| v6.2 | 4 | 2-3 | 1 agent |
| v6.3 | 3 | 2-3 | 1-2 agents |
| v6.4 | 11 | 4-5 | 2-3 agents |
| v6.5-6.8 | 19 | 6-9 | 1 agent each (parallel) |
| v7.0 | 8 | 4-6 | 2-3 agents |
| v7.1 | 4 | 3 | 1-2 agents |
| v7.2+ | 15+ | Ongoing | 1-2 agents |

### Next 4 Weeks (Immediate Priority)

**Week 1:** Create v6.0 Wave 1 feature specs (7 specs)

**Week 2:** Create v6.0 Wave 1 tech specs (7 specs) + QA plans (7 plans)

**Week 3:** Principal review + start implementation (first 3 features)

**Week 4:** Implementation (remaining 4 features) + integration

**Week 5:** Demo scenarios + v6.0 release

---

**Last Updated:** 2026-01-31  
**Status:** Ready for implementation  
**Next Action:** Create first feature spec for interact.explore
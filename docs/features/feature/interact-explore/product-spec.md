---
status: proposed
scope: feature/interact-explore
ai-priority: high
tags: [v6.0, core, ai-native, interaction]
relates-to: [interact-record.md, interact-replay.md, observe-capture.md]
last-verified: 2026-01-31
---

# Product Spec: interact.explore

**Version:** v6.0 (Wave 1: AI-Native Toolkit)  
**Capability:** Explore (interact)  
**Feature ID:** 1.1  
**Status:** Proposed

---

## User Story

As an AI assistant, I need to autonomously explore web applications to understand their behavior, test features, and validate functionality. I want to batch actions together and capture complete state after each action, so I can efficiently explore user flows without human intervention.

### Use Case 1: Spec-Driven Validation

**Given:** A product specification (natural language) and a live web application

**When I want to:** Validate that the application behaves as specified

#### Then I can:
- Batch execute multiple actions (navigate, click, fill, wait, etc.) together
- Capture comprehensive state (console, network, DOM, screenshots) after each action automatically
- Observe the results and compare against the spec
- Fix any bugs I discover autonomously

**So that:** I can validate the entire feature in <3 minutes without human help

### Use Case 2: Production Error Reproduction

**Given:** A recording of user interactions from production where a bug occurred

**When I want to:** Reproduce the bug in development environment and understand why it happened

#### Then I can:
- Replay the recording in development environment
- Capture state after each action
- Compare with original production recording
- Identify differences (e.g., API timeout, configuration mismatch)
- Fix the root cause autonomously

**So that:** I can debug production issues by comparing prod vs dev behavior

---

## User Persona

### Target Users:
- Claude (Anthropic) - AI assistant building/fixing web applications
- GPT-4 - AI assistant debugging web applications
- Other AI coding assistants

### User Goals:
- Autonomously explore and understand web applications
- Efficiently batch actions together
- Capture complete state without manual steps
- Compare states to identify differences
- Iterate and fix bugs automatically

### Pain Points:
- Current tools require one-at-a-time action execution
- No automatic state capture between actions
- Can't batch explore user flows efficiently
- Must manually orchestrate: "click → capture state → click → capture state"
- Difficult to compare before/after states
- Can't record and replay user interactions

---

## Problem Statement

### Current Limitations

#### Problem 1: No Action Batching
Current Gasoline MCP tools require explicit, sequential action execution. For example, to explore a checkout flow, an AI must:
1. Call tool to navigate to checkout
2. Call tool to fill shipping form
3. Call tool to fill billing form
4. Call tool to select payment method
5. Call tool to click "Pay" button
6. Call tool to wait for confirmation

Each tool call is a separate MCP request with its own latency. This is inefficient for exploring multi-step flows.

#### Problem 2: No Automatic State Capture
After each action, the AI must explicitly request state capture. For example:
1. Click button
2. Call `observe.capture` to get console logs
3. Call `observe.capture` to get network waterfall
4. Call `observe.capture` to get DOM snapshot

This is manual and error-prone. If the AI forgets to capture state, critical information is lost.

#### Problem 3: No Recording and Replay
If a user encounters a bug in production, there's no way to:
1. Record the user's interactions leading to the bug
2. Save the recording
3. Replay the recording in development environment to debug
4. Compare production vs development behavior

This makes production debugging difficult and requires manual orchestration by the user.

#### Problem 4: No State Comparison
To compare two application states (before/after, prod/dev), the AI must:
1. Capture state 1
2. Capture state 2
3. Manually compare the results to identify differences
4. Infer what changed and why

This is tedious and error-prone. The AI might miss subtle differences.

---

## Solution Overview

### Capability: interact.explore

**Purpose:** Enable AI to batch execute actions with automatic state capture, record interactions, and replay them for debugging.

### Core Functionality

1. **Batch Action Execution**
   - Execute multiple actions in a single MCP call
   - Actions execute sequentially in the browser
   - Each action can depend on previous actions
   - Return results for all actions

2. **Automatic State Capture**
   - After each action, automatically capture:
     - Console logs
     - Network requests (waterfall)
     - DOM snapshot
     - Screenshot (optional)
   - Include captured state in action result

3. **Session Recording**
   - Start a recording session
   - Capture all user interactions
   - Save state after each interaction
   - Stop recording and save to `.gas` file

4. **Session Replay**
   - Load a recording from `.gas` file
   - Replay all actions in sequence
   - Compare current state to original recording state
   - Return differences between replay and original

5. **Error Handling**
   - Stop execution on first failure (configurable)
   - Continue on failure (configurable)
   - Return clear error messages
   - Include debug information (screenshots, console logs)

---

## Technical Requirements

### MCP Tool: interact.explore

#### Input Parameters:

```javascript
{
  "type": "explore",
  "actions": [
    {
      "method": "goto|click|fill|select|check|uncheck|hover|scroll|wait|wait_for_selector|press_key|upload",
      "selector": "string",  // CSS selector or XPath
      "value": "string",     // For fill, select, press_key
      "options": {...}      // Method-specific options
    },
    ...
  ],
  "capture_after_each": true,  // Automatically capture state after each action
  "capture_types": ["console", "network", "dom", "screenshot"],  // What to capture
  "stop_on_failure": true,     // Stop on first failure
  "continue_on_failure": false   // Continue on failure (overrides stop_on_failure)
}
```

#### Action Methods:

1. **goto** - Navigate to URL
   ```javascript
   {
     "method": "goto",
     "url": "string"  // Absolute or relative URL
   }
   ```

2. **click** - Click element
   ```javascript
   {
     "method": "click",
     "selector": "string",  // CSS selector or XPath
     "options": {
       "wait": 1000,          // Wait ms before click
       "force": false          // Force click even if not visible
       "scroll_into_view": true // Scroll element into view before clicking
     }
   }
   ```

3. **fill** - Fill form field
   ```javascript
   {
     "method": "fill",
     "selector": "string",
     "value": "string",
     "options": {
       "clear": false,     // Clear field before filling
       "delay": 50,         // Delay ms between keystrokes
     }
   }
   ```

4. **select** - Select dropdown option
   ```javascript
   {
     "method": "select",
     "selector": "string",
     "value": "string"
   }
   ```

5. **check** - Check checkbox
   ```javascript
   {
     "method": "check",
     "selector": "string"
   }
   ```

6. **uncheck** - Uncheck checkbox
   ```javascript
   {
     "method": "uncheck",
     "selector": "string"
   }
   ```

7. **hover** - Hover over element
   ```javascript
   {
     "method": "hover",
     "selector": "string",
     "options": {
       "duration": 500  // Hover duration in ms
     }
   }
   ```

8. **scroll** - Scroll page
   ```javascript
   {
     "method": "scroll",
     "selector": "string | null",  // Element or window
     "x": 0,                    // Scroll x
     "y": 100,                  // Scroll y
     "options": {
       "behavior": "smooth" | "auto",
       "block": null
     }
   }
   ```

9. **wait** - Wait for duration
   ```javascript
   {
     "method": "wait",
     "duration": 1000  // Wait time in ms
   }
   ```

10. **wait_for_selector** - Wait for selector
   ```javascript
   {
     "method": "wait_for_selector",
     "selector": "string",
     "state": "visible|hidden|attached|detached",
     "timeout": 5000
   }
   ```

11. **press_key** - Press keyboard key
   ```javascript
   {
     "method": "press_key",
     "selector": "string | null",  // Element or window
     "key": "Enter|Escape|ArrowUp|ArrowDown|...",
     "options": {
       "modifiers": ["Control", "Shift", ...]
     }
   }
   ```

12. **upload** - Upload file
   ```javascript
   {
     "method": "upload",
     "selector": "string",
     "file_path": "string"  // Path to file to upload
   }
   ```

#### Output Response:

```javascript
{
  "execution_id": "explore-abc123",
  "actions_executed": [
    {
      "action": { /* action definition */ },
      "result": "success|failed",
      "duration_ms": 450,
      "captured_state": {
        "console": [...],      // Console logs after action
        "network": [...],     // Network waterfall after action
        "dom": {...},         // DOM snapshot after action
        "screenshot": "base64..."  // Screenshot after action (if requested)
      },
      "error": "string | null"  // Error message if failed
    }
    // ... all actions
  ],
  "final_state": {
    "console": [...],
    "network": [...],
    "dom": {...},
    "screenshot": "base64..."
  },
  "summary": {
    "total_actions": 10,
    "successful_actions": 9,
    "failed_actions": 1,
    "total_duration_ms": 4500
  }
}
```

### MCP Tool: interact.record

#### Input Parameters:

```javascript
{
  "type": "record",
  "name": "string",  // Recording name
  "capture_types": ["console", "network", "dom", "screenshot"]
}
```

#### Output Response:

```javascript
{
  "recording_id": "rec-abc123",
  "status": "recording",  // "recording" | "stopped"
  "actions_recorded": 0,
  "duration_seconds": 0
  "recording_file": null  // Set when stopped
}
```

### MCP Tool: interact.record_stop

#### Input Parameters:

```javascript
{
  "type": "record_stop",
  "recording_id": "rec-abc123"
}
```

#### Output Response:

```javascript
{
  "status": "completed",
  "recording_id": "rec-abc123",
  "actions_recorded": 15,
  "duration_seconds": 45,
  "recording_file": ".gasoline/recordings/rec-abc123.gas"
}
```

#### Recording Format (.gas file):

```json
{
  "recording_id": "rec-abc123",
  "name": "production-bug-reproduction",
  "timestamp": "2026-01-31T10:00:00Z",
  "browser": "Chrome 120",
  "viewport": {"width": 1920, "height": 1080},
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
    }
    // ... all actions
  ],
  "final_state": {
    "console": ["Error: Payment gateway timeout"],
    "network": [{"url": "/api/payment", "status": 504}],
    "dom": "<html>...</html>",
    "screenshot": "base64..."
  }
}
```

### MCP Tool: interact.replay_load

#### Input Parameters:

```javascript
{
  "type": "replay_load",
  "recording_file": ".gasoline/recordings/rec-abc123.gas"
}
```

#### Output Response:

```javascript
{
  "recording_id": "rec-abc123",
  "actions_count": 15,
  "duration_seconds": 45
}
```

### MCP Tool: interact.replay

#### Input Parameters:

```javascript
{
  "type": "replay",
  "recording_id": "rec-abc123",
  "compare_to": "original",  // "original" | "none"
  "capture_differences": true
}
```

#### Output Response:

```javascript
{
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
  },
  "summary": {
    "total_differences": 1,
    "breaking_changes": 0,
    "non_breaking_changes": 1
  }
}
```

---

## Success Criteria

### Functional Requirements

- [ ] All 12 action methods implemented
- [ ] Batch execution works for any number of actions
- [ ] Actions execute sequentially with dependencies
- [ ] State captured automatically after each action when enabled
- [ ] All capture types work (console, network, dom, screenshot)
- [ ] Error handling stops or continues based on configuration
- [ ] Recording starts and captures all interactions
- [ ] Recording stops and saves to `.gas` file
- [ ] Recording can be loaded for replay
- [ ] Replay executes all actions in sequence
- [ ] Replay compares to original when requested
- [ ] Differences detected and reported clearly

### Performance Requirements

- [ ] Each action executes in <1s average
- [ ] State capture adds <100ms per action
- [ ] Batch of 10 actions completes in <10s total
- [ ] Recording doesn't slow down browser (>5% overhead)
- [ ] Replay completes within 2x original duration
- [ ] Memory usage stable (<200MB buffer)

### Integration Requirements

- [ ] Works with existing observe.capture functionality
- [ ] Compatible with observe.compare for state analysis
- [ ] Compatible with analyze.infer for natural language analysis
- [ ] Integrates with execution history for doom loop prevention
- [ ] File storage in `.gasoline/recordings/` directory

### Quality Requirements

- [ ] Actions have clear error messages
- [ ] Debug information included in error state (screenshots, console)
- [ ] Performance metrics tracked (duration per action)
- [ ] Summary statistics provided

### Documentation Requirements

- [ ] All 12 action methods documented with examples
- [ ] MCP tool schema documented
- [ ] Recording format documented
- [ ] Replay comparison documented
- [ ] Integration with other tools documented

---

## Constraints

### Must Have

- **5 MCP Tools Maximum** — Cannot add 6th tool
- **Zero Runtime Dependencies** — No npm packages at runtime
- **TypeScript Strict Mode** — No `any` types
- **Go Implementation** — All code in Go

### Must Not Have

- **No External Services** — Cannot call external APIs for AI
- **No Browser Modifications** — Cannot modify browser behavior
- **No Framework Dependencies** — Works with vanilla browser APIs

---

## Non-Requirements

### Out of Scope

- **Visual Regression Testing** — Use Pixel-Perfect Guardian feature (v6.2)
- **Form Validation** — Basic field filling only
- **Canvas/Video Recording** — Not MVP
- **Advanced Interactions** — Drag & drop, multi-touch (v6.7)
- **Network Request Modification** — Use Prompt-Based Network Mocking (v6.2)

### Deferred to Future

- **Action Recording Macro** — Record sequences of actions as reusable macros
- **Conditional Execution** — If/else branching in action sequences
- **Parallel Execution** — Execute multiple actions concurrently
- **Smart Wait Strategies** — Intelligent waiting for elements (use wait_for_selector)

---

## Dependencies

### Internal Dependencies

- **Extension:** Background service worker, content script for DOM access
- **Server:** Go MCP server, WebSocket communication
- **Capture:** Existing observe.capture functionality
- **Buffers:** Network buffer, console buffer, DOM buffer
- **Session:** Recording and replay storage

### External Dependencies

- **Browser APIs** — DOM, Fetch API, WebSocket, Console
- **Playwright (optional)** — For advanced interactions (v6.7)

---

## Open Questions

1. **Action Method Completeness:** Are there any additional action methods needed beyond the 12 defined?

2. **Error Handling Granularity:** Should stop_on_failure be per-action or global?

3. **Capture Filtering:** Should capture_after_each support filtering (e.g., only network on action 3)?

4. **Recording Metadata:** Should recording include user info, browser version, viewport size?

5. **Replay Environment:** Should replay support environment configuration (dev vs staging vs prod)?

6. **Performance Targets:** Are the performance targets (<1s per action) realistic?

---

## Related Documents

- **Feature Spec (this document)** — Requirements
- **Tech Spec** — `tech-spec.md` — Implementation details
- **QA Plan** — `qa-plan.md` — Test scenarios
- **Related Features:**
  - `interact-record.md` — Session recording
  - `interact-replay.md` — Session replay
  - `observe-capture.md` — State capture
  - `observe-compare.md` — State comparison

---

## Acceptance Criteria

### Definition of Done

This feature is "done" when:

1. **All tools implemented** — 5 MCP tools (explore, record, record_stop, replay_load, replay)
2. **All action methods work** — 12 action methods functional
3. **Automatic capture works** — State captured after each action when enabled
4. **Recording works** — Can record and save sessions
5. **Replay works** — Can replay and compare states
6. **Integration tested** — Works with observe and analyze tools
7. **Documentation complete** — All specs written
8. **Tests passing** — All tests in QA plan pass
9. **Performance met** — Performance targets achieved
10. **Demo validated** — AI can batch actions and capture state automatically

### Sign-Off

- **Principal Review Required** — Before implementation starts
- **Tech Spec Review** — Before coding begins
- **Demo Validation** — Before v6.0 release

---

**Last Updated:** 2026-01-31  
**Next Phase:** Create tech spec
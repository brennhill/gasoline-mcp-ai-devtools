# MCP Tool Descriptions: Protocol-Based LLM Onboarding

**Status:** Implemented
**Priority:** P0
**Author:** Claude + Brenn
**Date:** 2026-01-26
**Implementation:** `cmd/dev-console/tools.go` (lines 586, 745, 863, 1028)
**Test:** `cmd/dev-console/testdata/mcp-tools-list.golden.json`

---

## 1. Executive Summary

LLMs connecting to Gasoline via MCP default to passivity: they ask users to click buttons, fill forms, and navigate instead of using the available `interact` tool. They hand-write tests instead of using `generate`. They ask users for clarification instead of calling `observe` first.

**Root Cause:** Tool descriptions were passive lists of features. They told the LLM "what it does" but not "when/how to use it" or "anti-patterns to avoid."

**Solution:** Redesign all four MCP tool descriptions to be **instructional**. Each description now teaches:
- **Priority/Intent** (START HERE, PERFORM ACTIONS, CREATE ARTIFACTS, CUSTOMIZE SESSION)
- **Explicit Anti-Patterns** (e.g., "Do NOT ask the user to click")
- **Rules & Dependencies** (observe → interact → observe pattern)
- **Concrete Examples** with expected outputs
- **Failure Modes & Recovery** (what to do if prerequisites aren't met)

**Outcome:** When an LLM connects and reads `tools/list`, it understands the pattern **from the protocol itself** — no manual system prompt editing required.

---

## 2. Problem Statement

### 2.1 The Passivity Default

Current behavior when users interact with Gasoline-enabled LLMs:

```
User: "The login form is broken. Can you debug it?"

LLM (Bad):
  "I see there's an error. Could you click the login button to trigger it again?"
  "Could you navigate to the form and try submitting?"
  "Can you fill out the form and tell me what happens?"

Expected (with Gasoline):
  observe({what: 'page'}) → Sees form structure
  observe({what: 'errors'}) → Sees console errors
  interact({action: 'execute_js', script: 'document.querySelector("button").click()'})
  observe({what: 'page'}) → Confirms form submitted
  observe({what: 'errors'}) → Sees new error
  generate({format: 'reproduction'}) → Creates bug reproduction script
```

### 2.2 Root Cause

Tool descriptions in the golden file were **passive feature lists**:

```json
{
  "name": "observe",
  "description": "Observe browser state, analyze captured data, or run security scans.
                  Covers raw data (errors, logs, network, WebSocket, actions, vitals, page info),
                  analysis (performance, API schema, accessibility, changes, timeline), and
                  security auditing."
}
```

This tells the LLM "what it does" but not:
- When to call it (before other tools)
- What NOT to do (don't ask user)
- How to use it (concrete examples)
- How it chains to other tools

### 2.3 Why System Prompt Injection Is Wrong

Alternative approach: Update the system prompt of Claude Code and every other MCP client.

**Problems:**
- Users must manually edit `.mcp.json` or their IDE's system prompt
- Each LLM platform has different system prompt locations
- When Gasoline updates, instructions get out of sync
- Instructions live in documentation users never read
- No guarantee the LLM reads the instructions at all

**Better:** The **protocol teaches the pattern**. When an LLM reads `tools/list`, it learns how to use Gasoline immediately, without external docs.

---

## 3. Solution Design

### 3.1 The Instructional Pattern

Each tool description now includes:

**1. Clear Priority/Intent (Lead Sentence)**
```
✅ "START HERE. Always call observe() first..."
✅ "PERFORM ACTIONS. Do NOT ask the user..."
✅ "CREATE ARTIFACTS. Do NOT write code/tests manually..."
✅ "CUSTOMIZE THE SESSION. Filter noise..."
```

**2. Explicit Anti-Patterns**
```
❌ "Don't ask user to click the button"
✅ "Use interact() to click instead"

❌ "Don't hand-write tests"
✅ "Use generate() to create them from recorded actions"
```

**3. Rules & Dependencies**
```
RULE: Before interact(), call observe() to understand state.
RULE: Before generate(), call observe() to see what data exists.
Pattern: observe() → interact() → observe()
```

**4. Concrete Examples with Expected Output**
```
observe({what:'page'}) → URL & structure
observe({what:'errors'}) → console errors
interact({action:'navigate',url:'https://example.com'}) → navigation
interact({action:'execute_js',script:'document.querySelector("button.submit").click()'}) → click action
generate({format:'test',test_name:'login flow'}) → Playwright test
```

**5. Failure Modes & Recovery**
```
PREREQUISITE: User must enable 'AI Web Pilot' in Gasoline extension popup.
If interact() fails, ask them to verify it's enabled.
```

### 3.2 The Four Tools

#### **observe** — The Context Tool
**Purpose:** Gather truth about browser state.

**Key Changes:**
- Added "START HERE" to lead
- Added explicit rule: "Before interact(), call observe()"
- Added concrete examples with arrow notation (→)
- Emphasized "source of truth"

**Before:**
```
"Observe browser state, analyze captured data, or run security scans..."
```

**After:**
```
"START HERE. Always call observe() first to see what data is available
before taking any action. Use observe to READ the current browser state—
this is your source of truth... RULES: Before interact(), call observe()
to understand state... Examples: observe({what:'page'})→URL & structure,
observe({what:'errors'})→console errors..."
```

#### **interact** — The Action Tool
**Purpose:** Perform browser actions.

**Key Changes:**
- Added explicit anti-pattern: "Do NOT ask the user to click"
- Listed all available actions with → notation
- Added JavaScript examples (the most important)
- Added prerequisite (AI Web Pilot) and failure mode
- Chained back to observe()

**Before:**
```
"Interactive browser control: highlight elements, manage page state snapshots,
execute JavaScript, navigate and control tabs..."
```

**After:**
```
"PERFORM ACTIONS. Do NOT ask the user to click, type, navigate, or fill forms—
use this tool instead... Actions: navigate(url)→go to URL,
execute_js(script)→run JavaScript to click/fill/submit... RULES: After
interact(), always call observe() to confirm the action worked...
Examples: interact({action:'execute_js',script:'document.querySelector(\"button\").click()'}),
interact({action:'execute_js',script:'document.querySelector(\"input\").value=\"test\"'})..."
```

#### **generate** — The Output Tool
**Purpose:** Create production-ready artifacts from captured data.

**Key Changes:**
- Added explicit anti-pattern: "Do NOT write code/tests manually"
- Linked output types to data sources
- Added concrete examples showing each format
- Chained to observe() (need data first)

**Before:**
```
"Generate artifacts from captured data: reproduction scripts, Playwright tests,
PR summaries, SARIF reports, HAR archives, Content-Security-Policy headers,
or SRI hashes."
```

**After:**
```
"CREATE ARTIFACTS. Do NOT write code/tests/docs manually—use this tool instead...
RULES: After observe() captures data, use generate() to create outputs.
Never hand-write tests when you could generate them from recorded actions...
Examples: generate({format:'test',test_name:'login flow'})→Playwright test,
generate({format:'reproduction'})→bug reproduction script..."
```

#### **configure** — The Session Tool
**Purpose:** Customize capture behavior for long sessions.

**Key Changes:**
- Action-focused list with purpose for each
- Concrete examples for the three most-used actions
- Explicit "when to use" guidance
- Positioned as optional (not critical path)

**Before:**
```
"Configure and manage the session: persistent store, noise filtering,
DOM queries, session snapshots, API validation, audit log, server health,
and event streaming."
```

**After:**
```
"CUSTOMIZE THE SESSION. Filter noise, store data, validate APIs, create snapshots...
Actions: noise_rule (add/remove patterns to ignore), store (save persistent data),
diff_sessions (create snapshots & compare)... Use when: isolating signal,
filtering noise, or tracking state across multiple actions."
```

---

## 4. Implementation Details

### 4.1 Code Changes

**File:** `cmd/dev-console/tools.go`

Four description strings updated in the `toolsList()` method:

| Tool | Line | Change Size |
|------|------|-------------|
| observe | 586 | 95 words → 167 words (75% growth) |
| generate | 745 | 28 words → 126 words (350% growth) |
| configure | 863 | 25 words → 139 words (456% growth) |
| interact | 1028 | 29 words → 191 words (559% growth) |

**Pattern:** Each description now ~130-170 words, structured as:
1. Lead + Intent (1 sentence)
2. Available actions/data (1-2 sentences)
3. Rules (2-3 bullet points)
4. Examples (3-5 concrete calls with →)
5. Failure modes (if applicable)

### 4.2 Test Updates

**File:** `cmd/dev-console/testdata/mcp-tools-list.golden.json`

Regenerated via `UPDATE_GOLDEN=1 go test` to capture new descriptions. This is the snapshot that tests verify against — ensures descriptions stay consistent and are tested with every change.

### 4.3 No Additional Infrastructure

This solution requires **zero additional MCP protocol support**:
- No new resources
- No custom headers
- No out-of-band initialization
- Tool descriptions are already part of the MCP spec (`tools/list` response)

The descriptions ride on existing MCP infrastructure.

---

## 5. How LLMs Use This

### 5.1 First Connection Flow

```
1. LLM connects to Gasoline via MCP
2. LLM calls tools/list
3. LLM reads tool descriptions (gets instructional text)
4. LLM understands the observe→interact→generate pattern
5. LLM starts using tools correctly
```

No system prompt editing needed. No documentation lookups required.

### 5.2 Example Behavior Change

**Before:**
```
User: "My form isn't submitting"
LLM: "Please fill out the form and try submitting it again.
      Tell me what error you see."
```

**After:**
```
User: "My form isn't submitting"
LLM:
  1. observe({what:'page'}) → [sees form]
  2. interact({action:'execute_js',script:'document.querySelector("form").submit()'})
  3. observe({what:'errors'}) → [sees the error]
  4. generate({format:'reproduction'}) → [creates script to repro]
  5. "Here's the error and a reproduction script"
```

The LLM reads the tool descriptions once and changes its behavior for the entire session.

---

## 6. Testing Strategy

### 6.1 Golden File Test

`TestMCPToolsListGolden` verifies:
- Tool descriptions are returned in `tools/list` response
- Descriptions match the golden file exactly
- Any change to descriptions is captured in the test

Run with:
```bash
go test ./cmd/dev-console -run TestMCPToolsListGolden
```

Update golden file when descriptions change:
```bash
UPDATE_GOLDEN=1 go test ./cmd/dev-console -run TestMCPToolsListGolden
```

### 6.2 Manual Verification

Test with Claude Code by:
1. Installing Gasoline extension
2. Adding to `.mcp.json`
3. Connecting Claude Code
4. Checking Claude's behavior when user says "click X" or "fill form Y"
5. Verify Claude uses interact() instead of asking

### 6.3 Real-World Testing

Observe LLM behavior over 2-3 weeks:
- Do LLMs still ask users to click buttons?
- Do they use observe() before acting?
- Do they use generate() for tests/reproduction?
- What % of interactions still require user involvement?

---

## 7. Limitations & Future Work

### 7.1 What This Solves

✅ LLMs understand basic usage patterns from descriptions
✅ Anti-patterns are explicit and clear
✅ Examples show concrete syntax
✅ Dependencies are documented
✅ Zero infrastructure overhead

### 7.2 What This Doesn't Solve

❌ LLMs still don't know the `what` enum values for observe()
❌ Complex filtering parameters (limit, status_min, etc.) aren't explained
❌ Error cases aren't documented (what if interact() fails?)
❌ The descriptions are long (LLM context cost is high)

### 7.3 Future Improvements

**Option 1: MCP Resources for Detailed Docs**
Expose a resource `gasoline://docs/observe` with full parameter documentation. LLMs could fetch as needed.

**Option 2: Inline Schema Documentation**
Add detailed descriptions to the `inputSchema` for each parameter (currently minimal).

**Option 3: Few-Shot Examples**
Include a resource `gasoline://examples` with full workflow examples the LLM can reference.

**Option 4: Tiered Descriptions**
- Short (current)
- Medium (with examples)
- Long (with all parameters)

LLMs could request the detail level they want.

---

## 8. Philosophical Alignment

### 8.1 "LLMs Just Work"

Gasoline's core value: AI debugging assistance that "just works" when enabled.

This change advances that philosophy by ensuring **LLMs understand Gasoline's capabilities automatically**, without manual setup or documentation reading.

The protocol teaches the pattern. The product delivers on the promise.

### 8.2 Zero-Dependency Principle

Tool descriptions are strings in Go code — no new dependencies, no infrastructure, no complexity.

---

## 9. Acceptance Criteria

✅ All four tool descriptions updated in tools.go
✅ Golden test file regenerated and passing
✅ Descriptions follow the pattern (Intent → Actions → Rules → Examples → Failure modes)
✅ No additional MCP protocol changes required
✅ Documentation of this spec in docs/specs/
✅ Manual testing shows LLM behavior improves (interact/generate used more, user asks fewer questions)

---

## 10. References

- **Implementation:** `cmd/dev-console/tools.go:580-1060`
- **Test:** `cmd/dev-console/testdata/mcp-tools-list.golden.json`
- **MCP Spec:** [Model Context Protocol](https://modelcontextprotocol.io/)
- **Related:** `docs/architecture.md` (tool design), `docs/product-philosophy.md` (product principles)

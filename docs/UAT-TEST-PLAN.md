# Gasoline MCP - User Acceptance Testing (UAT) Plan

## Overview

This document provides a comprehensive UAT plan for testing all Gasoline MCP functionality. It covers observe modes, interact commands, generate formats, configure actions, and extension behavior.

## Test Environment Setup

### Prerequisites
1. Gasoline MCP server running: `gasoline --port 7890`
2. Chrome browser with Gasoline extension installed
3. Extension reloaded after any TypeScript changes
4. A tab tracked in the extension

### Test Pages (Recommended)

| Purpose | URL | Why |
|---------|-----|-----|
| **Basic HTML** | https://example.com | Simple, fast, predictable |
| **Rich Content** | https://httpbin.org/html | Known HTML structure |
| **Forms** | https://httpbin.org/forms/post | Form submission testing |
| **Network Heavy** | https://www.wikipedia.org | Many resources (CSS, JS, images) |
| **API Testing** | https://jsonplaceholder.typicode.com | REST API endpoints |
| **WebSocket** | https://www.websocket.org/echo.html | WebSocket testing |
| **SPA** | https://react.dev | Single Page Application |
| **E-commerce** | https://demo.opencart.com | Forms, cart, checkout flows |
| **Auth Flow** | https://the-internet.herokuapp.com/login | Login form testing |
| **Complex Forms** | https://demoqa.com/automation-practice-form | Multi-field form |

---

## Pre-UAT: Clean Slate

**IMPORTANT: Run these commands before starting ANY UAT testing to ensure no stale data contaminates results.**

### Clear All Buffers

```bash
# Clear all buffers via configure tool
curl -s -X POST http://localhost:7890/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"configure","arguments":{"action":"clear","buffer":"all"}}}'
```

Or clear individually:
```bash
# Clear logs
curl -s -X POST http://localhost:7890/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"configure","arguments":{"action":"clear","buffer":"logs"}}}'

# Clear network
curl -s -X POST http://localhost:7890/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"configure","arguments":{"action":"clear","buffer":"network"}}}'

# Clear websocket
curl -s -X POST http://localhost:7890/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"configure","arguments":{"action":"clear","buffer":"websocket"}}}'

# Clear actions
curl -s -X POST http://localhost:7890/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"configure","arguments":{"action":"clear","buffer":"actions"}}}'
```

### Verify Clean State

After clearing, verify all buffers are empty:

```bash
# Check logs count
curl -s -X POST http://localhost:7890/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"logs","limit":1}}}' | jq -r '.result.content[0].text' | tail -1 | jq '.count'
# Expected: 0

# Check actions count
curl -s -X POST http://localhost:7890/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"observe","arguments":{"what":"actions","limit":1}}}' | jq -r '.result.content[0].text' | tail -1 | jq '.entries | length'
# Expected: 0

# Check network count
curl -s -X POST http://localhost:7890/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"observe","arguments":{"what":"network_waterfall","limit":1}}}' | jq -r '.result.content[0].text' | tail -1 | jq '.count'
# Expected: 0
```

### Clean Slate Checklist

- [ ] All buffers cleared (`configure({action:"clear", buffer:"all"})`)
- [ ] Logs count = 0
- [ ] Actions count = 0
- [ ] Network count = 0
- [ ] WebSocket count = 0

**Only proceed with UAT after confirming clean state.**

---

## Stub Detection: Before-Action-After Verification Pattern

**⚠️ CRITICAL**: Many MCP functions can return `{"status":"ok"}` without actually doing anything (stub implementations). Every test MUST verify that the action actually happened, not just that it returned "ok".

### The BAA Pattern (Before-Action-After)

For every action that modifies state:

1. **Before**: Record the current state
2. **Action**: Call the function
3. **After**: Verify state actually changed

### Example: Clear Buffer Test

```
❌ WRONG (just checks response):
configure({action: "clear", buffer: "actions"})
→ Returns {"status":"ok"} → PASS

✅ CORRECT (verifies actual effect):
1. BEFORE: observe({what:"actions"}) → count: 5
2. ACTION: configure({action: "clear", buffer: "actions"})
3. AFTER: observe({what:"actions"}) → count: 0
→ Count changed from 5 to 0 → PASS
```

### Example: Save State Test

```
❌ WRONG:
interact({action: "save_state", snapshot_name: "test"})
→ Returns {"status":"ok"} → PASS

✅ CORRECT:
1. ACTION: interact({action: "save_state", snapshot_name: "test"})
2. VERIFY: interact({action: "list_states"})
   → states array contains "test" → PASS
3. VERIFY: interact({action: "load_state", snapshot_name: "test"})
   → Returns actual saved data, not empty → PASS
```

### Functions Requiring BAA Verification

| Tool | Action | Must Verify |
|------|--------|-------------|
| configure | clear | Buffer count → 0 |
| configure | noise_rule add | Rule appears in list |
| configure | noise_rule remove | Rule disappears from list |
| configure | store save | Data retrievable via load |
| interact | save_state | State appears in list_states |
| interact | load_state | State actually restores |
| interact | delete_state | State disappears from list_states |
| interact | highlight | Element visually highlighted (manual check) |
| observe | security_audit | Returns real violations (not empty []) when issues exist |
| observe | third_party_audit | Returns real origins (not empty []) when third parties loaded |

### Red Flags (Likely Stubs)

These responses indicate a stub that needs investigation:

- Returns `{"status":"ok"}` with no other data
- Returns empty arrays `[]` when data should exist
- Returns placeholder message like "not implemented"
- Function claims success but subsequent query shows no change

---

## 0. Installation & Startup Testing

This section tests the full installation flow with a version bump to ensure the npm package installs correctly and the server starts properly.

### 0.1 Pre-Installation State

**Document current state before testing:**

1. Check current running server:
```bash
curl http://localhost:7890/health
# Note the version in response
```

2. Check current npm package version:
```bash
cat server/package.json | grep version
cat VERSION
```

3. Note any running gasoline processes:
```bash
ps aux | grep gasoline
lsof -ti :7890
```

### 0.2 Version Bump

**Bump the version to trigger a fresh install:**

1. Update VERSION file:
```bash
# Increment patch version (e.g., 5.6.6 → 5.6.7)
echo "5.6.7" > VERSION
```

2. Update server/package.json:
```bash
# Edit version field to match
```

3. Build new binary (if testing locally):
```bash
make build
# or: go build -o dist/gasoline-darwin-arm64 ./cmd/dev-console
```

### 0.3 Fresh Installation Test

**Test the npm install flow:**

1. Kill any existing server:
```bash
pkill -9 -f "gasoline --port"
# Verify port is free
lsof -ti :7890  # Should return nothing
```

2. Run npm install:
```bash
cd server && npm install
```

**Expected Output:**
```
Cleaning up old gasoline processes...
Downloading gasoline binary for darwin-arm64...
✓ Verified gasoline version: gasoline v5.6.7
Starting gasoline server on port 7890...
✓ Server started on http://127.0.0.1:7890
gasoline installed successfully!
```

3. Verify server is running:
```bash
curl http://localhost:7890/health
```

**Expected**: `{"status":"ok","version":"5.6.7"}`

### 0.4 Process Cleanup Verification

**Test that old processes are killed during upgrade:**

1. Start a server manually:
```bash
./dist/gasoline-darwin-arm64 --port 7890 &
```

2. Run npm install again:
```bash
cd server && npm install
```

**Expected**:
- Old process killed automatically
- New server started with correct version
- No "port already in use" errors

### 0.5 Extension Reconnection Test

**Verify extension reconnects after server restart:**

1. Open Chrome with Gasoline extension
2. Track a tab (click extension icon → "Track This Tab")
3. Verify connected (green badge on extension icon)
4. Kill and restart server:
```bash
pkill -9 -f "gasoline --port"
./dist/gasoline-darwin-arm64 --port 7890
```

**Expected**:
- Extension shows "!" badge briefly (disconnected)
- Extension auto-reconnects within 5 seconds
- Badge returns to normal
- `observe({what: "pilot"})` shows `extension_connected: true`

### 0.6 Cold Start Feature Verification

**After fresh install, verify basic features work:**

1. **Health check**:
```bash
curl http://localhost:7890/health
```
Expected: `{"status":"ok","version":"X.X.X"}`

2. **MCP tools list**:
```bash
curl -X POST http://localhost:7890/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```
Expected: Lists observe, interact, generate, configure tools

3. **Observe without extension** (should return empty data, not error):
```json
{"what": "logs"}
```
Expected: `{"entries":[],"total":0}` or similar empty response

4. **Extension tracking**:
- Track a tab in extension
- Run `observe({what: "pilot"})`
Expected: `extension_connected: true`, `tracked_tab_id` present

5. **Data capture**:
- Open console on tracked page
- Run: `console.log("UAT test")`
- Query: `observe({what: "logs", limit: 5})`
Expected: Log entry with "UAT test" message

### 0.7 Multi-Client Connection Test (Up to 10 Clients)

**Test --connect mode for multiple MCP clients (up to 10 supported):**

#### 0.7.1 Basic Two-Client Test

1. Start server normally:
```bash
./dist/gasoline-darwin-arm64 --port 7890
```

2. In another terminal, connect as second client:
```bash
./dist/gasoline-darwin-arm64 --connect http://localhost:7890
```

**Expected**:
- Second client connects successfully
- Both clients can query data
- Data is shared between clients

#### 0.7.2 Ten-Client Stress Test

**Setup script** (save as `test-10-clients.sh`):
```bash
#!/bin/bash
# Start primary server
./dist/gasoline-darwin-arm64 --port 7890 &
SERVER_PID=$!
sleep 1

# Start 9 additional clients
PIDS=()
for i in {1..9}; do
  ./dist/gasoline-darwin-arm64 --connect http://localhost:7890 &
  PIDS+=($!)
  echo "Started client $i (PID: $!)"
  sleep 0.5
done

echo "All 10 clients started. Testing..."

# Test that all clients respond
for port in 7890; do
  curl -s http://localhost:$port/health && echo " - Primary OK"
done

# Each --connect client uses stdio, test via server
curl -s -X POST http://localhost:7890/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"pilot"}}}' \
  | jq '.result'

echo "Press Enter to stop all clients..."
read

# Cleanup
kill $SERVER_PID ${PIDS[@]} 2>/dev/null
```

**Manual Test Procedure**:

1. Open 10 terminal windows/tabs

2. In terminal 1, start the primary server:
```bash
./dist/gasoline-darwin-arm64 --port 7890
```

3. In terminals 2-10, connect as additional clients:
```bash
# Terminal 2
./dist/gasoline-darwin-arm64 --connect http://localhost:7890

# Terminal 3
./dist/gasoline-darwin-arm64 --connect http://localhost:7890

# ... repeat for terminals 4-10
```

4. Verify all clients are connected:
```bash
curl http://localhost:7890/health
# Check server logs for "client connected" messages
```

5. Test data sharing - in any client, query:
```json
{"what": "pilot"}
```

**Expected Results**:
- All 10 clients connect without errors
- No "connection refused" or timeout errors
- Server logs show all client connections
- `observe({what: "pilot"})` returns same data from any client
- Server remains responsive under load
- No memory leaks or performance degradation

#### 0.7.3 Client Disconnect/Reconnect

1. With 5+ clients connected, disconnect one client (Ctrl+C)
2. Verify server and other clients continue working
3. Reconnect the disconnected client
4. Verify it receives the same data as others

**Expected**:
- Server handles disconnect gracefully
- No crash or data corruption
- Reconnected client syncs correctly

#### 0.7.4 Concurrent Query Test

With 10 clients connected, run simultaneous queries:

```bash
# Run in parallel from multiple terminals
for i in {1..10}; do
  curl -s -X POST http://localhost:7890/mcp \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":'$i',"method":"tools/call","params":{"name":"observe","arguments":{"what":"logs"}}}' &
done
wait
```

**Expected**:
- All 10 queries complete successfully
- No request timeouts
- Responses are consistent
- Server doesn't crash or deadlock

### 0.8 Graceful Shutdown Test

**Test server stops cleanly:**

1. Start server in foreground:
```bash
./dist/gasoline-darwin-arm64 --port 7890
```

2. Send SIGTERM:
```bash
kill -TERM $(lsof -ti :7890)
```

**Expected**:
- Server logs "shutting down" message
- Port freed immediately
- No zombie processes

### 0.9 Kill and Auto-Restart Verification

**Test that server restarts correctly after being killed.**

This test proves the complete lifecycle: kill → restart → verify.

#### Pre-Release vs Post-Release Testing

**Before npm publish** (local development):
- Use `make build` to create binary in `dist/`
- Start server directly: `./dist/gasoline-darwin-arm64 --port 7890`
- OR: `npm install` will fail download but use local `dist/` binary

**After npm publish** (release verification):
- Use `cd server && npm install` to download and auto-start

#### Step 1: Document Initial State

```bash
# Get current version and PID
curl http://localhost:7890/health
SERVER_PID=$(lsof -ti :7890)
echo "Initial server PID: $SERVER_PID"
```

#### Step 2: Kill the Server

```bash
# Force kill the running server
pkill -9 -f "gasoline --port"
# OR: kill -9 $SERVER_PID

# Verify server is dead
curl http://localhost:7890/health 2>&1 | grep -q "Connection refused" && echo "Server killed successfully"
```

#### Step 3: Restart the Server

**Option A: Pre-release (local binary)**
```bash
# Build if needed
make build

# Start directly from local binary
./dist/gasoline-darwin-arm64 --port 7890 &
```

**Option B: Post-release (npm install)**
```bash
# Run npm install - downloads and auto-starts the server
cd server && npm install
```

**Expected Output (npm install):**
```
Cleaning up old gasoline processes...
Downloading gasoline binary for darwin-arm64...
✓ Verified gasoline version: gasoline vX.X.X
Starting gasoline server on port 7890...
✓ Server started on http://127.0.0.1:7890
gasoline installed successfully!
```

#### Step 4: Verify New Server is Running

```bash
# Check health
curl http://localhost:7890/health
# Should return: {"status":"ok","version":"X.X.X"}

# Get new PID
NEW_PID=$(lsof -ti :7890)
echo "New server PID: $NEW_PID"

# Verify it's a different process
[ "$NEW_PID" != "$SERVER_PID" ] && echo "✓ New server process started"
```

#### Step 5: Verify Functionality After Restart

```bash
# Test MCP is working
curl -s -X POST http://localhost:7890/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq '.result.tools | length'
# Should return: 4 (observe, interact, generate, configure)

# Test observe works
curl -s -X POST http://localhost:7890/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"observe","arguments":{"what":"pilot"}}}' | jq '.result'
```

#### Complete Kill-Restart Script

Save as `test-kill-restart.sh`:
```bash
#!/bin/bash
set -e

# Usage: ./test-kill-restart.sh [--local]
# --local: Use local binary (pre-release testing)
# No flag: Use npm install (post-release testing)

USE_LOCAL=false
if [ "$1" = "--local" ]; then
  USE_LOCAL=true
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BINARY="$PROJECT_ROOT/dist/gasoline-darwin-arm64"

echo "=== Kill and Restart Verification Test ==="
echo "Mode: $([ "$USE_LOCAL" = true ] && echo 'LOCAL BINARY' || echo 'NPM INSTALL')"

# Step 1: Document initial state
echo ""
echo "Step 1: Checking initial server..."
INITIAL_HEALTH=$(curl -s http://localhost:7890/health || echo "not running")
INITIAL_PID=$(lsof -ti :7890 || echo "none")
echo "Initial health: $INITIAL_HEALTH"
echo "Initial PID: $INITIAL_PID"

# Step 2: Kill the server
echo ""
echo "Step 2: Killing server..."
pkill -9 -f "gasoline --port" || true
sleep 1

# Verify dead
if curl -s http://localhost:7890/health >/dev/null 2>&1; then
  echo "FAIL: Server still running after kill"
  exit 1
fi
echo "✓ Server killed successfully"

# Step 3: Restart
echo ""
if [ "$USE_LOCAL" = true ]; then
  echo "Step 3: Starting local binary..."
  if [ ! -f "$BINARY" ]; then
    echo "Binary not found. Building..."
    cd "$PROJECT_ROOT" && make build
  fi
  "$BINARY" --port 7890 &
  sleep 2
else
  echo "Step 3: Running npm install..."
  cd "$PROJECT_ROOT/server" && npm install
  cd - >/dev/null
  sleep 2
fi

# Step 4: Verify restart
echo ""
echo "Step 4: Verifying new server..."
NEW_HEALTH=$(curl -s http://localhost:7890/health)
NEW_PID=$(lsof -ti :7890)
echo "New health: $NEW_HEALTH"
echo "New PID: $NEW_PID"

if [ -z "$NEW_PID" ]; then
  echo "FAIL: No server running after restart"
  exit 1
fi

if [ "$NEW_PID" = "$INITIAL_PID" ] && [ "$INITIAL_PID" != "none" ]; then
  echo "FAIL: Same PID - server was not restarted"
  exit 1
fi
echo "✓ New server process running"

# Step 5: Verify functionality
echo ""
echo "Step 5: Testing MCP functionality..."
TOOLS_COUNT=$(curl -s -X POST http://localhost:7890/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq '.result.tools | length')

if [ "$TOOLS_COUNT" -ge 4 ]; then
  echo "✓ MCP tools available: $TOOLS_COUNT"
else
  echo "FAIL: Expected 4+ tools, got $TOOLS_COUNT"
  exit 1
fi

echo ""
echo "=== All Kill-Restart Tests Passed ==="
```

**Expected Results:**
- [ ] Server is killed successfully (health returns "Connection refused")
- [ ] npm install starts a new server automatically
- [ ] New server has a different PID than the killed one
- [ ] Health endpoint returns OK with correct version
- [ ] MCP tools/list returns all 4 tools
- [ ] Observe tool works after restart

### 0.10 AI-Executed MCP Verification

**Have the AI assistant execute actual MCP commands to verify functionality.**

This section requires the AI (Claude Code with Gasoline MCP) to run live tool calls.

#### 0.10.1 Basic Connectivity Check

Ask the AI to run:
```
observe({what: "pilot"})
```

**Expected**: AI returns response with `extension_connected` status

#### 0.10.2 Observe Tool Smoke Test

Ask the AI to run each observe mode:
```
observe({what: "logs", limit: 5})
observe({what: "errors", limit: 5})
observe({what: "page"})
observe({what: "tabs"})
observe({what: "pilot"})
observe({what: "network_waterfall", limit: 10})
observe({what: "actions", limit: 5})
```

**Expected**: Each returns valid JSON (empty arrays are OK if no data)

#### 0.10.3 Interact Tool Test (requires AI Web Pilot enabled)

Ask the AI to execute:
```
interact({action: "navigate", url: "https://example.com"})
```

Wait for navigation, then:
```
observe({what: "page"})
```

**Expected**: Page URL shows `https://example.com`

#### 0.10.4 Generate Tool Test

After navigating to a resource-heavy page (Wikipedia):
```
interact({action: "navigate", url: "https://www.wikipedia.org"})
```

Wait 5 seconds, then:
```
generate({format: "csp", mode: "moderate"})
```

**Expected**: Returns CSP policy with multiple directives

#### 0.10.5 Configure Tool Test

```
configure({action: "health"})
configure({action: "clear", buffer: "logs"})
```

**Expected**: Health returns OK, clear succeeds

#### 0.10.6 Full AI Verification Script

**Instructions for AI assistant:**

"Execute the following MCP verification sequence and report results:

1. Check pilot status
2. Navigate to https://example.com
3. Check page info
4. Check for any console logs
5. Navigate to https://www.wikipedia.org
6. Wait 5 seconds
7. Generate CSP policy
8. Report: extension connected, page URL, CSP directive count"

**Expected Output Format:**
```
✓ Extension connected: true/false
✓ Page URL: https://www.wikipedia.org
✓ Page title: Wikipedia
✓ CSP directives: 5+ (or unavailable with reason)
✓ All MCP tools responding
```

### 0.11 Startup Checklist

Complete this checklist for each release:

- [ ] VERSION file updated
- [ ] server/package.json version updated
- [ ] Old processes killed during install
- [ ] New server starts automatically
- [ ] Health endpoint returns correct version
- [ ] Extension reconnects after restart
- [ ] MCP tools/list works
- [ ] Basic observe works (logs, errors, page)
- [ ] Data flows from extension to observe
- [ ] Multi-client mode works
- [ ] Graceful shutdown works

---

## 1. OBSERVE Tool Tests

### 1.1 Console Logs (`observe({what: "logs"})`)

**IMPORTANT: Clear logs buffer first to avoid stale data!**

```json
{"action": "clear", "buffer": "logs"}
```

**Setup**: Open browser console, execute log commands with UNIQUE markers:

```javascript
// In tracked page console - use unique timestamp marker:
const marker = `UAT_${Date.now()}`;
console.log(`Test log message ${marker}`);
console.warn(`Test warning ${marker}`);
console.error(`Test error ${marker}`);
console.info(`Test info ${marker}`);
console.debug(`Test debug ${marker}`);
```

**Verification**:
```json
{"what": "logs", "limit": 10}
```

**MUST VERIFY**: The response contains logs with your SPECIFIC `UAT_xxxxx` marker.

- ❌ FAIL if marker not found (stale data)
- ✅ PASS only if your unique marker appears in results

### 1.2 Errors (`observe({what: "errors"})`)

**IMPORTANT: Clear first, use unique markers!**

```json
{"action": "clear", "buffer": "logs"}
```

**Setup**: Trigger JavaScript errors with UNIQUE markers:

```javascript
// In tracked page console - use unique marker:
const marker = `UAT_ERR_${Date.now()}`;
throw new Error(`Test uncaught error ${marker}`);
```

**Verification**:
```json
{"what": "errors", "limit": 10}
```

**MUST VERIFY**: Response contains your SPECIFIC `UAT_ERR_xxxxx` marker.

- ❌ FAIL if marker not found
- ✅ PASS only if your unique error message appears

### 1.3 Extension Logs (`observe({what: "extension_logs"})`)

**Expected**: Internal extension debug logs visible

### 1.4 Network Waterfall (`observe({what: "network_waterfall"})`)

**IMPORTANT: Clear network buffer first!**

```json
{"action": "clear", "buffer": "network"}
```

**Setup**: Navigate to a FRESH page after clearing:

```json
{"action": "navigate", "url": "https://www.wikipedia.org"}
```

Wait 5 seconds for resources to load.

**Verification**:
```json
{"what": "network_waterfall", "limit": 50}
```

**MUST VERIFY**: Response contains URLs from wikipedia.org domain.

- ❌ FAIL if count = 0 after navigating to Wikipedia (no capture)
- ❌ FAIL if URLs don't match the page you just navigated to
- ✅ PASS if fresh Wikipedia resources appear

### 1.5 Network Bodies (`observe({what: "network_bodies"})`)

**IMPORTANT: Clear network buffer first!**

```json
{"action": "clear", "buffer": "network"}
```

**Setup**: Navigate to test page and make a UNIQUE fetch request.

```javascript
// In tracked page console - use unique query param:
const marker = Date.now();
fetch(`https://jsonplaceholder.typicode.com/posts/1?uat=${marker}`)
  .then(r => r.json())
  .then(console.log);
```

**Verification**:
```json
{"what": "network_bodies", "limit": 10}
```

**MUST VERIFY**: Response contains URL with your `?uat=xxxxx` marker.

- ❌ FAIL if your unique URL not found
- ✅ PASS if your specific fetch request appears

### 1.6 WebSocket Events (`observe({what: "websocket_events"})`)

**IMPORTANT: Clear websocket buffer first!**

```json
{"action": "clear", "buffer": "websocket"}
```

**Setup**: Visit https://www.websocket.org/echo.html, then:

1. Click "Connect"
2. Send a UNIQUE message: `UAT_WS_[timestamp]`
3. Click "Disconnect"

**Verification**:
```json
{"what": "websocket_events", "limit": 20}
```

**MUST VERIFY**: Response contains your unique `UAT_WS_xxxxx` message.

- ❌ FAIL if your message not found
- ✅ PASS if open, your message, and close events appear

### 1.7 WebSocket Status (`observe({what: "websocket_status"})`)

**Expected**: Current connection states for active WebSockets

### 1.8 Actions (`observe({what: "actions"})`)

**IMPORTANT: Clear actions buffer first!**

```json
{"action": "clear", "buffer": "actions"}
```

**Verify count = 0 before proceeding using BAA pattern.**

**Setup**: Navigate to a page with a form (e.g., `https://the-internet.herokuapp.com/login`):

1. **AI Action first** (creates timestamp marker):

   ```json
   {"action": "navigate", "url": "https://the-internet.herokuapp.com/login"}
   ```

2. **Human Actions** (do these manually AFTER the AI navigation):
   - Click the username field
   - Type something
   - Click the password field

**Verification**:
```json
{"what": "actions", "limit": 20}
```

**MUST VERIFY**:

- The FIRST action should be `navigate` with `source: "ai"` (your AI marker)
- Subsequent actions should be `click`/`input` with `source: "human"`
- ❌ FAIL if you see actions from BEFORE your AI navigation timestamp
- ✅ PASS if actions are chronological starting from your navigate

### 1.9 Web Vitals (`observe({what: "vitals"})`)

**Expected**: Core Web Vitals (LCP, FCP, CLS) when available

### 1.10 Page Info (`observe({what: "page"})`)

**Expected**:
- `url`: Current tracked page URL
- `title`: Page title (not blank)

### 1.11 Tabs (`observe({what: "tabs"})`)

**Expected**: List of all open browser tabs

### 1.12 Pilot Status (`observe({what: "pilot"})`)

**Expected**:
- `enabled`: AI Web Pilot toggle state
- `extension_connected`: true when extension syncing

### 1.13 Performance (`observe({what: "performance"})`)

**Expected**: Navigation timing, resource timing data

### 1.14 API Schema (`observe({what: "api"})`)

**Setup**: Make various API calls from tracked page

**Expected**: Detected API endpoints with request/response shapes

### 1.15 Accessibility (`observe({what: "accessibility"})`)

**Expected**: Accessibility audit results (axe-core)

### 1.16 Changes (`observe({what: "changes"})`)

**Expected**: DOM/state changes since last check

### 1.17 Timeline (`observe({what: "timeline"})`)

**Expected**: Chronological event stream

### 1.18 Error Clusters (`observe({what: "error_clusters"})`)

**Setup**: Trigger same error multiple times

**Expected**: Grouped/deduplicated errors

### 1.19 History (`observe({what: "history"})`)

**Expected**: Browser navigation history for tracked tab

### 1.20 Security Audit (`observe({what: "security_audit"})`)

**Expected**: Security findings (headers, cookies, transport)

### 1.21 Third Party Audit (`observe({what: "third_party_audit"})`)

**Expected**: Third-party resource analysis

### 1.22 Security Diff (`observe({what: "security_diff"})`)

**Expected**: Security changes between snapshots

### 1.23 Command Result (`observe({what: "command_result", correlation_id: "..."})`)

**Setup**: Execute async command, note correlation_id

**Expected**: Result of async command execution

### 1.24 Pending Commands (`observe({what: "pending_commands"})`)

**Expected**: Commands waiting for extension execution

### 1.25 Failed Commands (`observe({what: "failed_commands"})`)

**Expected**: Commands that failed to execute

---

## 2. INTERACT Tool Tests

### 2.1 Navigate (`interact({action: "navigate", url: "..."})`)

**Test**:
```json
{"action": "navigate", "url": "https://example.com"}
```

**Expected**:
- Status: "queued" with correlation_id
- AI action recorded with `source: "ai"`
- Page navigates (verify via observe page)

### 2.2 Refresh (`interact({action: "refresh"})`)

**Test**:
```json
{"action": "refresh"}
```

**Expected**: Page reloads, action recorded

### 2.3 Back (`interact({action: "back"})`)

**Setup**: Navigate to multiple pages first

**Test**:
```json
{"action": "back"}
```

**Expected**: Browser navigates back

### 2.4 Forward (`interact({action: "forward"})`)

**Setup**: Use back first

**Test**:
```json
{"action": "forward"}
```

**Expected**: Browser navigates forward

### 2.5 New Tab (`interact({action: "new_tab", url: "..."})`)

**Test**:
```json
{"action": "new_tab", "url": "https://example.com"}
```

**Expected**: New tab opens with URL

### 2.6 Execute JS (`interact({action: "execute_js", script: "..."})`)

**Test**:
```json
{"action": "execute_js", "script": "return document.title"}
```

**Expected**:
- Script executes in page context
- Result returned via command_result

### 2.7 Highlight (`interact({action: "highlight", selector: "..."})`)

**⚠️ KNOWN STUB**: This function may return `{"status":"ok"}` without highlighting.

**Setup**: Navigate to a page with an h1 element

**Test with BAA Pattern**:

1. **BEFORE**: Visually observe the h1 element (no highlight)

2. **ACTION**:
```json
{"action": "highlight", "selector": "h1", "duration_ms": 5000}
```

3. **AFTER**:
   - ❌ FAIL if response is just `{"status":"ok"}` with no other data
   - ❌ FAIL if h1 element shows no visual change (no colored border/background)
   - ✅ PASS only if element is VISUALLY highlighted for ~5 seconds

**Manual Verification Required**: This test REQUIRES human observation of the browser.

### 2.8 State Management

**⚠️ KNOWN STUBS**: These functions may return `{"status":"ok"}` without actually saving/loading state.

#### 2.8.1 Save and Verify State Exists

**Test with BAA Pattern**:

1. **BEFORE**: List states and note which exist
```json
{"action": "list_states"}
```
Record the states array.

2. **ACTION**: Save a new state with unique name
```json
{"action": "save_state", "snapshot_name": "UAT_STATE_[timestamp]"}
```

3. **AFTER**: List states again
```json
{"action": "list_states"}
```

**Verification**:
- ❌ FAIL if `list_states` returns empty `{"states":[]}` (stub!)
- ❌ FAIL if your `UAT_STATE_[timestamp]` is not in the states array
- ✅ PASS only if your new state name appears in the list

#### 2.8.2 Load State Returns Data

**Test**:
```json
{"action": "load_state", "snapshot_name": "UAT_STATE_[timestamp]"}
```

**Verification**:
- ❌ FAIL if response is just `{"status":"ok"}` with no state data
- ❌ FAIL if response contains no URL/cookies/localStorage information
- ✅ PASS only if response includes actual state data (URL, at minimum)

#### 2.8.3 Delete State Actually Removes It

**Test with BAA Pattern**:

1. **BEFORE**: Verify state exists
```json
{"action": "list_states"}
```
Confirm `UAT_STATE_[timestamp]` is in the list.

2. **ACTION**: Delete the state
```json
{"action": "delete_state", "snapshot_name": "UAT_STATE_[timestamp]"}
```

3. **AFTER**: List states again
```json
{"action": "list_states"}
```

**Verification**:
- ❌ FAIL if deleted state still appears in list (stub!)
- ✅ PASS only if state is actually removed from the list

---

## 3. GENERATE Tool Tests

### 3.1 CSP Policy (`generate({format: "csp"})`)

**Setup**: Navigate to page with external resources first

**Test without data**:
```json
{"format": "csp"}
```
**Expected**: `status: "unavailable"` with reason and hint

**Test with data** (after loading Wikipedia):
```json
{"format": "csp"}
```
**Expected**: Real CSP policy with directives

### 3.2 Reproduction Script (`generate({format: "reproduction"})`)

**Setup**: Perform actions on page

**Test**:
```json
{"format": "reproduction"}
```

**Expected**: Playwright-compatible script

### 3.3 Test Generation (`generate({format: "test"})`)

**Test**:
```json
{"format": "test", "test_name": "my-test"}
```

**Expected**: Playwright test file

### 3.4 PR Summary (`generate({format: "pr_summary"})`)

**Expected**: Pull request summary from session

### 3.5 SARIF Export (`generate({format: "sarif"})`)

**Expected**: SARIF format security findings

### 3.6 HAR Export (`generate({format: "har"})`)

**Expected**: HTTP Archive format

### 3.7 SRI Hashes (`generate({format: "sri"})`)

**Expected**: Subresource Integrity hashes

---

## 4. CONFIGURE Tool Tests

### 4.1 Noise Rules

**⚠️ KNOWN STUBS**: These functions may return fake counts without actually managing rules.

#### 4.1.1 Add and Verify Rule Exists

**Test with BAA Pattern**:

1. **BEFORE**: List existing rules
```json
{"action": "noise_rule", "noise_action": "list"}
```
Note the `totalRules` count.

2. **ACTION**: Add a rule with unique identifier
```json
{"action": "noise_rule", "noise_action": "add", "rules": [{"category": "console", "match_spec": {"message_regex": "UAT_NOISE_[timestamp]"}}], "reason": "UAT test rule"}
```

3. **AFTER**: List rules again
```json
{"action": "noise_rule", "noise_action": "list"}
```

**Verification**:
- ❌ FAIL if `totalRules` did not increase
- ❌ FAIL if rules array doesn't contain your UAT_NOISE pattern
- ✅ PASS only if rule count increased AND your rule is in the list

#### 4.1.2 Remove Rule Actually Removes It

**Test with BAA Pattern**:

1. **BEFORE**: List rules and get a rule_id
```json
{"action": "noise_rule", "noise_action": "list"}
```

2. **ACTION**: Remove the rule
```json
{"action": "noise_rule", "noise_action": "remove", "rule_id": "[id from step 1]"}
```

3. **AFTER**: List rules again

**Verification**:
- ❌ FAIL if rule still exists in list
- ✅ PASS only if rule is actually removed

### 4.2 Clear Buffers

**Test with BAA Pattern** (already fixed - verify it works):

#### 4.2.1 Clear Logs

1. **BEFORE**: Generate logs and check count
```javascript
// In browser console with unique marker
console.log("UAT_CLEAR_TEST_" + Date.now());
```
```json
{"what": "logs", "limit": 1}
```
Note the count (should be > 0).

2. **ACTION**: Clear logs
```json
{"action": "clear", "buffer": "logs"}
```

3. **AFTER**: Check count again
```json
{"what": "logs", "limit": 1}
```

**Verification**:
- Response should include `"cleared": {"logs": N}` where N > 0
- After-count should be 0
- ❌ FAIL if count unchanged
- ✅ PASS if count went to 0

#### 4.2.2 Clear Actions

Same pattern - verify count goes from N to 0.

#### 4.2.3 Clear Network

Same pattern - verify count goes from N to 0.

#### 4.2.4 Clear All

Verify ALL buffers go to 0 in one call.

### 4.3 Store/Load Data

**⚠️ KNOWN STUBS**: These functions may return "ok" without actually storing data.

#### 4.3.1 Save and Retrieve Data

**Test with BAA Pattern**:

1. **ACTION**: Save data with unique key
```json
{"action": "store", "store_action": "save", "namespace": "uat", "key": "UAT_KEY_[timestamp]", "data": {"test": "value", "number": 42}}
```

2. **VERIFY**: Load the same data back
```json
{"action": "store", "store_action": "load", "namespace": "uat", "key": "UAT_KEY_[timestamp]"}
```

**Verification**:
- ❌ FAIL if load returns empty or just `{"status":"ok"}`
- ❌ FAIL if loaded data doesn't match saved data (`{"test": "value", "number": 42}`)
- ✅ PASS only if exact data is returned

#### 4.3.2 List Stored Keys

```json
{"action": "store", "store_action": "list", "namespace": "uat"}
```

**Verification**:
- Should show your saved key `UAT_KEY_[timestamp]`
- ❌ FAIL if always returns empty list

#### 4.3.3 Delete Stored Data

**Test with BAA Pattern**:

1. **BEFORE**: Verify key exists via list
2. **ACTION**: Delete the key
```json
{"action": "store", "store_action": "delete", "namespace": "uat", "key": "UAT_KEY_[timestamp]"}
```
3. **AFTER**: Verify key is gone via list and load returns not-found

### 4.4 Health Check

```json
{"action": "health"}
```

**Expected**: Server health status with actual metrics (not just `{"status":"ok"}`)

### 4.5 Query DOM

**⚠️ KNOWN STUB**: May return empty `{"matches":[]}` without querying.

**Test with BAA Pattern**:

1. **SETUP**: Navigate to a page with known elements (e.g., Wikipedia has `h1`, `p`, `a` elements)

2. **ACTION**: Query for elements that definitely exist
```json
{"action": "query_dom", "selector": "h1"}
```

**Verification**:
- ❌ FAIL if returns `{"matches":[]}` on a page with h1 elements
- ✅ PASS only if matches array contains actual element data

### 4.6 Security Audit

**⚠️ KNOWN STUB**: May return empty `{"violations":[]}` without auditing.

**Test with BAA Pattern**:

1. **SETUP**: Navigate to a page that loads resources over HTTP or has known security issues

2. **ACTION**: Run security audit
```json
{"action": "security_audit", "checks": ["transport", "headers"]}
```

**Verification**:
- On a page with mixed content (HTTP resources on HTTPS page), should find violations
- ❌ FAIL if always returns empty violations regardless of page
- ✅ PASS if returns real violations when issues exist

### 4.7 Third-Party Audit

**⚠️ KNOWN STUB**: May return empty `{"third_parties":[]}` without auditing.

**Test with BAA Pattern**:

1. **SETUP**: Navigate to a page that loads third-party resources (e.g., Wikipedia loads from wikimedia.org, Google Analytics sites)

2. **ACTION**: Run third-party audit
```json
{"action": "third_party_audit", "first_party_origins": ["https://www.wikipedia.org"]}
```

**Verification**:
- On Wikipedia, should detect wikimedia.org as third-party
- ❌ FAIL if always returns empty third_parties regardless of page
- ✅ PASS if returns real third-party origins when they exist

---

## 5. Extension Behavior Tests

### 5.1 Track/Untrack Tab

1. Click extension icon
2. Click "Track This Tab" button
3. Verify button changes to "Stop Tracking"
4. Verify tracked URL shows in popup
5. Click "Stop Tracking"
6. Verify favicon flicker stops on that tab

### 5.2 AI Web Pilot Toggle

1. Enable AI Web Pilot in popup
2. Verify interact commands work
3. Disable AI Web Pilot
4. Verify interact commands return "pilot disabled" error

### 5.3 Connection Status

1. Start server, verify "Connected" badge
2. Stop server, verify "!" badge appears
3. Restart server, verify auto-reconnect

### 5.4 Settings Persistence

1. Change settings in extension options
2. Reload extension
3. Verify settings persisted

---

## 6. Form Interaction Tests

### Test Page: https://the-internet.herokuapp.com/login

### 6.1 Fill Form Fields

```json
{"action": "execute_js", "script": "document.querySelector('#username').value = 'tomsmith'"}
{"action": "execute_js", "script": "document.querySelector('#password').value = 'SuperSecretPassword!'"}
```

### 6.2 Submit Form

```json
{"action": "execute_js", "script": "document.querySelector('button[type=\"submit\"]').click()"}
```

### 6.3 Verify Success

```json
{"what": "page"}
```

Check URL changed to /secure

### Test Page: https://demoqa.com/automation-practice-form

Test complex form with:
- Text inputs
- Radio buttons
- Checkboxes
- Date pickers
- File uploads
- Dropdowns

---

## 7. Screenshot Tests

### 7.1 Full Page Screenshot

```json
{"action": "execute_js", "script": "/* screenshot logic */"}
```

### 7.2 Element Screenshot

```json
{"action": "execute_js", "script": "/* element screenshot */"}
```

---

## 8. Edge Cases & Error Handling

### 8.1 Empty Data Scenarios

| Observe Mode | Expected when empty |
|--------------|---------------------|
| logs | `[]` |
| errors | `[]` |
| actions | `[]` |
| network_waterfall | `[]` |
| network_bodies | `[]` |
| websocket_events | `[]` |

### 8.2 Invalid Parameters

Test each tool with:
- Missing required parameters
- Invalid parameter values
- Malformed JSON

**Expected**: Structured error with helpful message

### 8.3 Disconnected Extension

Test observe/interact when:
- Extension not installed
- Extension disabled
- No tab tracked
- Pilot disabled (for interact)

### 8.4 Large Data Sets

Test with:
- 1000+ log entries
- Large network responses
- Many actions

---

## 9. Traffic Generation Scripts

### 9.1 Generate Console Logs

```javascript
// Run in tracked page console
for (let i = 0; i < 50; i++) {
  console.log(`Log entry ${i}`);
  if (i % 5 === 0) console.warn(`Warning ${i}`);
  if (i % 10 === 0) console.error(`Error ${i}`);
}
```

### 9.2 Generate Network Traffic

```javascript
// Run in tracked page console
const urls = [
  'https://jsonplaceholder.typicode.com/posts/1',
  'https://jsonplaceholder.typicode.com/posts/2',
  'https://jsonplaceholder.typicode.com/users/1',
  'https://jsonplaceholder.typicode.com/comments?postId=1'
];
urls.forEach(url => fetch(url).then(r => r.json()).then(console.log));
```

### 9.3 Generate User Actions

```javascript
// Run in tracked page console - creates synthetic events
['click', 'scroll', 'input'].forEach((type, i) => {
  setTimeout(() => {
    const event = new Event(type, { bubbles: true });
    document.body.dispatchEvent(event);
  }, i * 100);
});
```

### 9.4 Generate WebSocket Traffic

Visit https://www.websocket.org/echo.html and:
1. Click "Connect"
2. Type messages and click "Send"
3. Click "Disconnect"

---

## 10. Human vs AI Action Testing

This section validates that actions are correctly attributed to their source.

**⚠️ CRITICAL: Stale data can contaminate this test!**

The actions buffer may contain old test data. You MUST verify a clean slate.

### 10.1 Prerequisite Setup

1. Track a page with interactive elements (e.g., `https://the-internet.herokuapp.com/login`)

2. Clear the actions buffer:

   ```json
   {"action": "clear", "buffer": "actions"}
   ```

3. **VERIFY** the buffer is empty:

   ```json
   {"what": "actions", "limit": 1}
   ```

   - If count > 0, run clear again and verify
   - Only proceed when actions count = 0

4. Enable AI Web Pilot in extension popup

5. Note the current timestamp: `Date.now()` - all actions should be AFTER this

### 10.2 Record Human Actions (Requires Manual Testing)

**Step 1**: Perform manual interactions on the tracked page:
- Click the username input field
- Type "testuser"
- Click the password field
- Type "testpass"
- Click the Login button
- Scroll the page up and down

**Step 2**: Query actions:
```json
{"what": "actions", "limit": 20}
```

**Expected Results**:
- All actions have `"source": "human"`
- Action types include: `click`, `input`, `scroll`
- Selectors identify the interacted elements
- Timestamps are chronological

### 10.3 Record AI Actions

**Step 1**: Execute AI-driven actions via interact tool:
```json
{"action": "navigate", "url": "https://example.com"}
{"action": "execute_js", "script": "document.title"}
{"action": "refresh"}
{"action": "highlight", "selector": "h1"}
```

**Step 2**: Query actions:
```json
{"what": "actions", "limit": 20}
```

**Expected Results**:
- AI actions have `"source": "ai"`
- Action types include: `navigate`, `execute_js`, `refresh`, `highlight`
- Each action has appropriate metadata (URL, script, selector)

### 10.4 Mixed Human + AI Actions

**Step 1**: Perform this sequence:
1. **Human**: Click a link on the page
2. **AI**: `{"action": "navigate", "url": "https://httpbin.org/html"}`
3. **Human**: Scroll down
4. **AI**: `{"action": "execute_js", "script": "return document.body.innerText.length"}`
5. **Human**: Select some text

**Step 2**: Query actions and verify:
```json
{"what": "actions", "limit": 10}
```

**Expected Results**:
- Actions interleaved with correct source attribution
- Human actions: `source: "human"`
- AI actions: `source: "ai"`
- Chronological ordering preserved

### 10.5 Action Source Verification Checklist

| Action | Expected Source | How to Trigger |
|--------|-----------------|----------------|
| Click | `human` | Manual click on element |
| Input | `human` | Manual typing in field |
| Scroll | `human` | Manual scroll |
| Select | `human` | Manual text selection |
| navigate | `ai` | `interact({action: "navigate"})` |
| refresh | `ai` | `interact({action: "refresh"})` |
| back | `ai` | `interact({action: "back"})` |
| forward | `ai` | `interact({action: "forward"})` |
| execute_js | `ai` | `interact({action: "execute_js"})` |
| highlight | `ai` | `interact({action: "highlight"})` |

---

## 11. CSP Generation Success Testing

This section validates that CSP (Content-Security-Policy) is correctly generated from observed network traffic.

### 11.1 Understanding CSP Directives

CSP controls which resources can load. Key directives to verify:

| Directive | What it controls | Example origins |
|-----------|------------------|-----------------|
| `script-src` | JavaScript files | CDNs, analytics |
| `style-src` | CSS stylesheets | CDNs, fonts |
| `font-src` | Web fonts | Google Fonts, Adobe |
| `img-src` | Images | CDNs, avatars |
| `connect-src` | XHR/fetch/WebSocket | APIs, analytics |
| `media-src` | Audio/video | CDNs, streaming |
| `frame-src` | Iframes | Embeds, widgets |
| `default-src` | Fallback for all | Usually 'self' |

### 11.2 Test Page: Wikipedia (Comprehensive Resources)

**Setup**: Navigate to https://www.wikipedia.org and interact

**Step 1**: Track the Wikipedia page
**Step 2**: Wait for page to fully load (5+ seconds)
**Step 3**: Generate CSP:
```json
{"format": "csp", "mode": "moderate"}
```

**Expected Results**:
- `status: "success"` (not "unavailable")
- Policy contains multiple directives
- Origins include:
  - `'self'` (same origin)
  - `*.wikimedia.org` (assets)
  - Possibly analytics domains

### 11.3 Test Page: Google Fonts + CDN Heavy

**Setup**: Visit https://fonts.google.com

**Expected CSP directives**:
- `font-src`: `fonts.gstatic.com`
- `style-src`: `fonts.googleapis.com`
- `script-src`: Various Google domains

### 11.4 Test Page: API-Heavy (connect-src)

**Setup**: Visit https://jsonplaceholder.typicode.com and run:
```javascript
// In console - make API requests
fetch('/posts').then(r => r.json());
fetch('/users').then(r => r.json());
fetch('/comments?postId=1').then(r => r.json());
```

**Generate CSP**:
```json
{"format": "csp"}
```

**Expected Results**:
- `connect-src` includes `jsonplaceholder.typicode.com`
- Policy reflects the actual API endpoints observed

### 11.5 Test Page: Media Content

**Setup**: Visit YouTube or a video streaming site

**Expected CSP directives**:
- `media-src`: Video CDN origins
- `img-src`: Thumbnail origins
- `frame-src`: Embedded player origins

### 11.6 CSP Mode Comparison

Test the same page with all three modes:

**Strict Mode**:
```json
{"format": "csp", "mode": "strict"}
```
- Most restrictive
- May include nonces/hashes
- Minimal `unsafe-*` directives

**Moderate Mode** (default):
```json
{"format": "csp", "mode": "moderate"}
```
- Balanced security
- Allows common patterns

**Report-Only Mode**:
```json
{"format": "csp", "mode": "report_only"}
```
- Non-blocking
- Includes `report-uri` directive

### 11.7 CSP Validation Checklist

After generating CSP, verify:

- [ ] Policy is non-empty string
- [ ] Contains `default-src` directive
- [ ] Contains at least 3 different directives
- [ ] Origins are real domains (not placeholders)
- [ ] Mode is reflected correctly
- [ ] No syntax errors (validate at https://csp-evaluator.withgoogle.com/)

### 11.8 CSP Unavailable Scenarios

Test these should return `status: "unavailable"`:

1. **Fresh session, no navigation**:
```json
{"format": "csp"}
```
Expected: `status: "unavailable"`, `reason` explains why

2. **Static HTML page with no external resources**:
Navigate to `data:text/html,<h1>Hello</h1>` then generate CSP

3. **After clearing network buffer**:
```json
{"action": "clear", "buffer": "network"}
```
Then immediately:
```json
{"format": "csp"}
```

### 11.9 Full CSP Success Test Procedure

**Complete Test Flow**:

1. Start fresh (clear all buffers)
2. Navigate to https://www.wikipedia.org
3. Wait 10 seconds for full page load
4. Click several links (human actions)
5. Generate CSP:
```json
{"format": "csp", "mode": "moderate"}
```

6. **Verify output contains**:
   - `status: "success"` OR actual policy string
   - Multiple origins from different domains
   - At least: `default-src`, `script-src`, `style-src`, `img-src`

7. **Validate the policy**:
   - Copy the generated policy
   - Paste into https://csp-evaluator.withgoogle.com/
   - Should parse without syntax errors
   - Should identify the resource origins correctly

---

## 12. Checklist Summary

### Pre-Test Setup (MUST DO FIRST)

- [ ] Server running (`curl http://localhost:7890/health`)
- [ ] Extension loaded in Chrome
- [ ] Tab tracked (check popup)
- [ ] AI Web Pilot enabled (for interact tests)
- [ ] **CLEAN SLATE**: All buffers cleared (`configure({action:"clear", buffer:"all"})`)
- [ ] **VERIFIED**: Logs count = 0
- [ ] **VERIFIED**: Actions count = 0
- [ ] **VERIFIED**: Network count = 0

### Installation & Startup Tests (Section 0)
- [ ] VERSION file updated with new version
- [ ] server/package.json version matches
- [ ] Old processes killed during npm install
- [ ] New server starts automatically after install
- [ ] Health endpoint returns correct version
- [ ] Extension auto-reconnects after server restart
- [ ] MCP tools/list returns expected tools
- [ ] Basic observe works without extension (empty data, no error)
- [ ] Data flows from extension to observe after tracking
- [ ] Multi-client: 2 clients work correctly
- [ ] Multi-client: 10 clients connect without errors
- [ ] Multi-client: concurrent queries from 10 clients succeed
- [ ] Multi-client: disconnect/reconnect handled gracefully
- [ ] Graceful shutdown (SIGTERM) works cleanly
- [ ] Kill-restart: server killed successfully
- [ ] Kill-restart: npm install auto-starts new server
- [ ] Kill-restart: new server has different PID
- [ ] Kill-restart: MCP functionality works after restart
- [ ] AI-executed: observe(pilot) returns valid response
- [ ] AI-executed: observe smoke test (logs, errors, page, tabs)
- [ ] AI-executed: interact(navigate) works
- [ ] AI-executed: generate(csp) returns policy or unavailable
- [ ] AI-executed: configure(health) returns OK

### Observe Tests (25 modes)
- [ ] logs
- [ ] errors
- [ ] extension_logs
- [ ] network_waterfall
- [ ] network_bodies
- [ ] websocket_events
- [ ] websocket_status
- [ ] actions (verify source field)
- [ ] vitals
- [ ] page (verify title not blank)
- [ ] tabs
- [ ] pilot
- [ ] performance
- [ ] api
- [ ] accessibility
- [ ] changes
- [ ] timeline
- [ ] error_clusters
- [ ] history
- [ ] security_audit
- [ ] third_party_audit
- [ ] security_diff
- [ ] command_result
- [ ] pending_commands
- [ ] failed_commands

### Interact Tests (11 actions)
- [ ] navigate
- [ ] refresh
- [ ] back
- [ ] forward
- [ ] new_tab
- [ ] execute_js
- [ ] highlight
- [ ] save_state
- [ ] load_state
- [ ] list_states
- [ ] delete_state

### Generate Tests (7 formats)
- [ ] csp (with/without data)
- [ ] reproduction
- [ ] test
- [ ] pr_summary
- [ ] sarif
- [ ] har
- [ ] sri

### Extension Tests
- [ ] Track/Untrack tab
- [ ] Favicon flicker stops on untrack
- [ ] Page title captured
- [ ] AI Web Pilot toggle
- [ ] Connection badge updates
- [ ] Settings persistence

### Human vs AI Action Tests (Section 10)
- [ ] Human actions have `source: "human"`
- [ ] AI actions have `source: "ai"`
- [ ] Mixed human+AI actions correctly attributed
- [ ] All human action types captured (click, input, scroll, select)
- [ ] All AI action types captured (navigate, refresh, back, forward, execute_js, highlight)

### CSP Success Tests (Section 11)
- [ ] CSP returns "unavailable" when no network data
- [ ] CSP generates real policy after page load
- [ ] Wikipedia test: multiple directives present
- [ ] API test: connect-src populated
- [ ] All three modes work (strict, moderate, report_only)
- [ ] Generated policy validates at csp-evaluator.withgoogle.com

### Stub Detection Tests (CRITICAL)

**These tests specifically verify functions actually work, not just return `{"status":"ok"}`.**

#### Interact Tool State Management
- [ ] `save_state`: After save, `list_states` shows the new state name
- [ ] `load_state`: Returns actual state data (URL, localStorage), not just `{"status":"ok"}`
- [ ] `delete_state`: After delete, `list_states` no longer shows the state
- [ ] `highlight`: Element is VISUALLY highlighted (manual verification required)

#### Configure Tool
- [ ] `clear`: After clear, buffer count is actually 0 (not just "ok" response)
- [ ] `noise_rule add`: After add, `list` shows the new rule
- [ ] `noise_rule remove`: After remove, `list` no longer shows the rule
- [ ] `store save`: After save, `store load` returns the exact data saved
- [ ] `store delete`: After delete, `store load` returns not-found
- [ ] `query_dom`: On page with h1, returns actual element data (not empty `[]`)

#### Observe Tool (Security)
- [ ] `security_audit`: On mixed-content page, returns actual violations (not empty `[]`)
- [ ] `third_party_audit`: On Wikipedia, returns third-party origins (not empty `[]`)

#### Known Stubs (Expected to Fail Until Implemented)

These are documented as stubs and should return "not_implemented":

- [ ] `observe({what: "api"})` - Returns "not_implemented" (v6.0)
- [ ] `observe({what: "changes"})` - Returns "not_implemented" (v6.0)

---

## Version History

| Date | Version | Changes |
|------|---------|---------|
| 2026-02-05 | 1.8 | Added stub detection: BAA verification pattern, updated all tests to verify actual state changes, added stub detection checklist |
| 2026-02-05 | 1.7 | Added unique markers to all observe tests to detect stale data; documented actions clear bug |
| 2026-02-05 | 1.6 | Added Pre-UAT Clean Slate section (mandatory buffer clear before testing) |
| 2026-02-05 | 1.5 | Added AI-executed MCP verification (Section 0.10) |
| 2026-02-05 | 1.4 | Added kill and auto-restart verification (Section 0.9) |
| 2026-02-05 | 1.3 | Expanded multi-client testing to 10 clients (Section 0.7) |
| 2026-02-05 | 1.2 | Added Installation & Startup testing (Section 0) |
| 2026-02-05 | 1.1 | Added Human vs AI action testing (Section 10), CSP success testing (Section 11) |
| 2026-02-05 | 1.0 | Initial UAT plan |

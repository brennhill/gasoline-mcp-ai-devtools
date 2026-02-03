# Post-Refactor Verification Checklist

> **How to be 100% sure everything works after major refactoring**

After splitting god files and major architectural changes, use this checklist to verify **nothing** broke.

---

## 🤖 Automated Verification

**Run the comprehensive verification script:**

```bash
bash scripts/verify-refactor.sh
```

This runs 7 levels of verification:
1. ✅ Compilation & Static Analysis
2. ✅ Unit Tests (all packages)
3. ✅ Integration Tests
4. ✅ Performance Benchmarks
5. ✅ Critical Path Verification (file existence, method existence)
6. ✅ Quality Standards (file length, linting)
7. ✅ Smoke Tests (binary builds and runs)

**Expected output:** `✅ All verifications passed`

If any fail, review the detailed output and fix issues before proceeding.

---

## 🧪 Manual Verification (Belt & Suspenders)

Even with automated checks, manually verify these critical scenarios:

### Level 1: Can the Server Start?

```bash
# Build the binary
go build -o /tmp/gasoline ./cmd/dev-console

# Start server
/tmp/gasoline --server --port 7890 &
SERVER_PID=$!

# Wait for startup
sleep 2

# Check health
curl http://127.0.0.1:7890/health

# Should return: {"status":"ok"}
```

**Expected:** Server starts **without** errors, health check returns 200 OK

**Cleanup:**
```bash
kill $SERVER_PID
```

---

### Level 2: Do MCP Tools Work?

**Test each of the 4 main tools:**

**1. Observe Tool:**
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"logs"}}}' | \
  /tmp/gasoline --connect --port 7890
```

**Expected:** Returns JSON with logs (or empty array if no logs)

**2. Generate Tool:**
```bash
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"generate","arguments":{"format":"csp"}}}' | \
  /tmp/gasoline --connect --port 7890
```

**Expected:** Returns CSP policy (or stub response)

**3. Configure Tool:**
```bash
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"configure","arguments":{"action":"health"}}}' | \
  /tmp/gasoline --connect --port 7890
```

**Expected:** Returns server health status

**4. Interact Tool (AI Web Pilot):**
```bash
echo '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"interact","arguments":{"action":"navigate","url":"https://example.com"}}}' | \
  /tmp/gasoline --connect --port 7890
```

**Expected:** Returns correlation_id (command queued)

---

### Level 3: Do HTTP Endpoints Work?

```bash
# Start server (if not already running)
/tmp/gasoline --server --port 7890 &
sleep 2

# Test each endpoint
curl http://127.0.0.1:7890/health              # Should: 200 OK
curl http://127.0.0.1:7890/diagnostics         # Should: JSON with server state
curl -X POST http://127.0.0.1:7890/clients     # Should: 200 or 405
curl http://127.0.0.1:7890/mcp/sse             # Should: 200 (SSE stream)

# Cleanup
pkill -f "gasoline --server"
```

**Expected:** **All** endpoints respond (**no** 500 errors, **no** panics in logs)

---

### Level 4: Does the Extension Connect?

**Prerequisites:** Chrome with Gasoline extension installed

**Steps:**
1. Start server: `/tmp/gasoline --server --port 7890`
2. Open Chrome DevTools → Console
3. Check for: `[Gasoline] Connected to server`
4. Open extension popup - should show "Connected" status
5. Navigate to a page - check server logs for captured events

**Expected:** Extension connects, captures events, server receives them

---

### Level 5: Critical Functionality Tests

**Test the most important features:**

#### A. WebSocket Capture
1. Navigate to websocket.org/echo.html
2. Open WebSocket connection
3. Send message
4. Run: `curl http://127.0.0.1:7890/websocket/events`
5. **Expected:** JSON array with captured WebSocket events

#### B. Network Body Capture
1. Navigate to any page with API calls (e.g., github.com)
2. Wait for network activity
3. Run: `curl 'http://127.0.0.1:7890/network/bodies?limit=10'`
4. **Expected:** JSON array with request/response bodies

#### C. User Action Capture
1. Click some elements on a page
2. Run: `curl 'http://127.0.0.1:7890/actions?limit=10'`
3. **Expected:** JSON array with click events

#### D. Performance Capture
1. Navigate to any page
2. Run: `curl http://127.0.0.1:7890/vitals`
3. **Expected:** JSON with Web Vitals (LCP, FID, CLS)

#### E. AI Web Pilot (Async Queue Pattern - **Critical**!)

1. Enable AI Web Pilot in extension popup
2. Via MCP, send navigate command
3. Extension should poll and execute
4. Command should complete within 10 seconds
5. **Expected:** Page navigates, result returned

This is the most **critical** test - verifies async queue **wasn't** broken by refactoring.

---

## 🔍 What to Look For

### Signs **Everything** Works ✅

- **No** panics in server logs
- **All** HTTP endpoints return valid responses
- Extension connects and stays connected
- Events are captured in real-time
- MCP tools return valid JSON
- Tests **all** pass
- Benchmarks run **without** errors

### Signs **Something** Broke ❌

- Panics or crashes on startup
- HTTP endpoints return 500 errors
- Extension shows "Disconnected"
- **No** events captured despite user activity
- MCP tools timeout or return errors
- Tests fail or skip unexpectedly
- Benchmarks compile errors

---

## 🎯 Comprehensive Verification Commands

**Run all at once:**

```bash
# Clean build
make clean
make compile-ts

# Full quality gate
make quality-gate

# Comprehensive verification
bash scripts/verify-refactor.sh

# Integration smoke test
go build -o /tmp/gasoline ./cmd/dev-console
/tmp/gasoline --version
/tmp/gasoline --help
/tmp/gasoline --server --port 7890 &
sleep 2
curl http://127.0.0.1:7890/health
pkill -f "gasoline --server"
```

If **all** pass: Refactoring is safe ✅

---

## 🚨 If Something Fails

### Debugging Steps:

1. **Check compilation errors:**
   ```bash
   go build ./... 2>&1 | tee build-errors.txt
   ```

2. **Check test failures:**
   ```bash
   go test ./... -v 2>&1 | tee test-failures.txt
   grep "^--- FAIL:" test-failures.txt
   ```

3. **Check for missing methods:**
   ```bash
   # Example: Find where toolObserve is defined
   grep -rn "func.*toolObserve" cmd/dev-console/
   ```

4. **Check for import issues:**
   ```bash
   # Missing imports after file splits?
   go build ./cmd/dev-console 2>&1 | grep "undefined:"
   ```

5. **Compare with last working commit:**
   ```bash
   git diff HEAD~1 cmd/dev-console/
   ```

---

## 📊 Success Criteria

### Required (Must All Pass)

- [x] ✅ All Go packages compile
- [x] ✅ All TypeScript compiles
- [x] ✅ go vet returns 0 issues
- [x] ✅ ESLint returns 0 errors
- [x] ✅ All unit tests pass
- [x] ✅ All benchmarks run
- [x] ✅ Critical files exist
- [x] ✅ Critical methods exist
- [x] ✅ Binary builds
- [x] ✅ Binary runs (--help, --version)

### Recommended (Should Pass)

- [ ] ✅ All files under 800 lines (or justified)
- [ ] ✅ Integration tests pass
- [ ] ✅ Server starts without errors
- [ ] ✅ Extension connects successfully
- [ ] ✅ Events captured correctly
- [ ] ✅ MCP tools return valid responses

### Nice to Have (Manual Verification)

- [ ] Test all 4 MCP tools manually
- [ ] Test AI Web Pilot (async queue)
- [ ] Test with real Chrome extension
- [ ] Compare performance benchmarks with baseline
- [ ] Review server logs for warnings

---

## 🔄 Regression Testing

**Before refactoring, capture baseline:**

```bash
# Save current benchmarks
go test -bench=. -benchmem ./... > benchmarks-before.txt

# Save test output
go test ./... -v > tests-before.txt
```

**After refactoring, compare:**

```bash
# Run benchmarks again
go test -bench=. -benchmem ./... > benchmarks-after.txt

# Compare (look for >20% regressions)
# Use benchcmp or manual review
diff benchmarks-before.txt benchmarks-after.txt

# Run tests again
go test ./... -v > tests-after.txt

# Compare (should have same or more passing tests)
diff tests-before.txt tests-after.txt
```

---

## 🎯 Confidence Levels

### 99% Confidence (Automated Only)
- Run `scripts/verify-refactor.sh`
- All checks pass
- No manual testing

**Risk:** Edge cases not covered by tests might be broken

### 99.9% Confidence (Automated + Smoke Tests)
- Run `scripts/verify-refactor.sh`
- Start server and test endpoints
- Test binary flags
- Quick extension connection test

**Risk:** Complex user workflows might have issues

### 100% Confidence (Full Manual Verification)
- Run `scripts/verify-refactor.sh`
- Start server and extension
- Test all 4 MCP tools manually
- Test AI Web Pilot (async queue)
- Test WebSocket capture
- Test network capture
- Test action capture
- Test all HTTP endpoints
- Review server logs for any warnings/errors

**Risk:** None - everything verified

---

## 📋 Post-Refactor Checklist

Use this after ANY major refactoring:

**Automated Verification:**
- [ ] `bash scripts/verify-refactor.sh` passes
- [ ] `make quality-gate` passes
- [ ] `make ci-local` passes

**Manual Verification:**
- [ ] Server starts without errors
- [ ] Extension connects successfully
- [ ] At least 1 event captured (WebSocket/network/action)
- [ ] MCP tool call works (any tool)
- [ ] Binary flags work (--help, --version)

**Code Review:**
- [ ] All moved code is accounted for (nothing lost)
- [ ] All imports updated correctly
- [ ] No duplicate code created
- [ ] Comments/docs moved with code
- [ ] Tests still test the right things

**Performance:**
- [ ] Benchmarks run without errors
- [ ] No regressions >20% in any benchmark
- [ ] Memory usage similar to before

If **all** pass: Safe to merge ✅

---

## 🚀 Quick Verification (30 seconds)

**Minimum viable verification:**

```bash
# 1. Does it compile?
go build ./cmd/dev-console && echo "✅ Compiles"

# 2. Do tests pass?
go test ./cmd/dev-console -v && echo "✅ Tests pass"

# 3. Can it run?
./gasoline --version && echo "✅ Runs"
```

If **all** 3 pass, you have high confidence the refactoring worked.

---

## 💡 Pro Tips

### Before Large Refactors

1. **Tag the current state:**
   ```bash
   git tag before-refactor-$(date +%Y%m%d)
   ```

2. **Run and save baseline:**
   ```bash
   bash scripts/verify-refactor.sh > verification-before.txt
   ```

3. **Document the plan:**
   - What files will be split?
   - What will move where?
   - What could break?

### During Refactoring

1. **Commit frequently** (every file split)
2. **Test after each commit**
3. **Keep old files** until new ones compile
4. **Use git stash** to try risky changes

### After Refactoring

1. **Run verification script**
2. **Manual smoke test** key features
3. **Compare benchmarks** with baseline
4. **Review git diff** for anything unexpected
5. **Ask colleague** to code review

---

## 📝 Example: Verifying tools.go Split

**The refactoring we just did:**
- Split `tools.go` (2396 lines) → 7 files

**How we verify it worked:**

```bash
# 1. Automated
bash scripts/verify-refactor.sh

# 2. Check all tool methods exist
grep "func (h \*ToolHandler) tool" cmd/dev-console/tools_*.go | wc -l
# Should be: ~40+ methods

# 3. Check dispatch works
go test ./cmd/dev-console -v -run TestHandleToolCall

# 4. Start server and test
/tmp/gasoline --server --port 7890 &
sleep 2

# Test observe
curl -X POST http://127.0.0.1:7890/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"logs"}}}'

# Should return valid MCP response

pkill -f gasoline
```

**Result:** If **all** pass, the split was successful and safe.

---

## ✅ Final Checklist

Before marking refactoring as complete:

**Automated:**
- [ ] `scripts/verify-refactor.sh` exits 0
- [ ] `make quality-gate` passes
- [ ] All tests pass (no skips except integration)
- [ ] All benchmarks run
- [ ] go vet clean
- [ ] ESLint 0 errors

**Manual:**
- [ ] Server starts and responds to health check
- [ ] At least one MCP tool tested successfully
- [ ] Binary flags work (--help, --version)
- [ ] No panics in server logs
- [ ] Git diff reviewed (no accidental deletions)

**Performance:**
- [ ] Benchmarks run without errors
- [ ] No major regressions (>20%)
- [ ] Memory usage reasonable

**Documentation:**
- [ ] This verification checklist completed
- [ ] Commit message documents what was refactored
- [ ] Any breaking changes documented

**If all checked:** ✅ Refactoring verified and safe to merge

---

## 🎯 Bottom Line

**For 100% confidence after refactoring:**

1. **Run:** `bash scripts/verify-refactor.sh`
2. **Test:** Start server, test MCP tool, check extension
3. **Review:** Git diff, check for missing code
4. **Compare:** Benchmarks before/after

**If all pass:** You have **100% confidence** everything works! 🎉

---

**Last updated:** 2026-02-02
**Used for:** tools.go/main.go split verification
**Status:** All verifications passed ✅

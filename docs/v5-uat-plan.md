# v5 UAT Plan

User Acceptance Testing plan for v5 features. All tests must pass before release.

---

## Pre-UAT Checklist

- [ ] `go vet ./cmd/dev-console/` passes
- [ ] `make test` passes (all Go tests)
- [ ] `node --test extension-tests/*.test.js` passes
- [ ] No version label violations in code comments
- [ ] All file headers present and correct

---

## P4: AI Web Pilot

### Safety Toggle (Critical Path)

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 1 | Toggle defaults to OFF | Fresh install, open popup | "AI Web Pilot" checkbox unchecked |
| 2 | Toggle persists | Enable toggle, close/reopen popup | Toggle remains checked |
| 3 | Toggle blocks highlight | Toggle OFF, call `highlight_element` | Error: "ai_web_pilot_disabled" |
| 4 | Toggle blocks state | Toggle OFF, call `manage_state` | Error: "ai_web_pilot_disabled" |
| 5 | Toggle blocks execute | Toggle OFF, call `execute_javascript` | Error: "ai_web_pilot_disabled" |
| 6 | Toggle enables all | Toggle ON, call any pilot tool | Tool executes successfully |
| 7 | AI cannot enable toggle | No MCP tool can modify toggle state | N/A (verify no such tool exists) |
| 8 | Toggle syncs across tabs | Enable in one tab, check another | Both show enabled |

### highlight_element

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 9 | Basic highlight | Call with valid selector | Red overlay appears on element |
| 10 | Highlight positioning | Scroll page, observe highlight | Overlay follows element position |
| 11 | Auto-remove | Wait for duration | Overlay disappears after timeout |
| 12 | Replace highlight | Highlight A, then B | A removed, B shown |
| 13 | Invalid selector | Call with non-existent selector | Error: "element_not_found" |
| 14 | Return bounds | Call with valid selector | Response includes x, y, width, height |
| 15 | Custom duration | Call with duration_ms: 1000 | Overlay disappears after 1s |

### manage_state

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 16 | Save captures all | Set localStorage, sessionStorage, cookie, save | Snapshot includes all three |
| 17 | Load restores all | Load snapshot | All three storage types restored |
| 18 | Load clears first | Have existing data, load snapshot | Old data replaced, not merged |
| 19 | List shows metadata | Save 2 snapshots, call list | Returns both with url, timestamp, size |
| 20 | Delete removes | Delete snapshot, call list | Deleted snapshot not in list |
| 21 | Round-trip integrity | Save → clear all storage → load | Original values restored exactly |
| 22 | include_url navigation | Save on /page-a, load with include_url:true | Navigates to /page-a |
| 23 | include_url skip | Load with include_url:false | Stays on current page |
| 24 | Large state | Save 1MB of localStorage | Saves and loads without error |

### execute_javascript

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 25 | Simple expression | `1 + 1` | `{ success: true, result: 2 }` |
| 26 | Access window | `window.location.hostname` | Returns current hostname |
| 27 | Access DOM | `document.title` | Returns page title |
| 28 | Object return | `({ a: 1, b: [2, 3] })` | Properly serialized object |
| 29 | Function execution | `(() => 42)()` | `{ result: 42 }` |
| 30 | Error handling | `throw new Error('test')` | `{ success: false, error: 'execution_error', stack: '...' }` |
| 31 | Syntax error | `{{{` | Error response with message |
| 32 | Promise resolution | `Promise.resolve(42)` | `{ result: 42 }` |
| 33 | Promise rejection | `Promise.reject(new Error('fail'))` | Error response |
| 34 | Timeout | Script with infinite loop | Timeout error after 5s |
| 35 | Custom timeout | `timeout_ms: 1000` with 2s delay | Timeout after 1s |
| 36 | Circular reference | Object with circular ref | Serializes with [Circular] marker |
| 37 | DOM node return | `document.body` | Descriptive string, not crash |
| 38 | Redux store | `window.__REDUX_STORE__?.getState()` | Returns store state or undefined |
| 39 | Next.js data | `window.__NEXT_DATA__` | Returns Next.js payload if present |

---

## P5: Nice-to-Have

### Binary Format Detection

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 40 | MessagePack detection | Capture MessagePack body | `binary_format: "messagepack"` in response |
| 41 | Protobuf detection | Capture protobuf body | `binary_format: "protobuf"` |
| 42 | Unknown binary | Capture random binary | No binary_format field or null |
| 43 | Text not detected | Capture JSON/text | No binary_format field |
| 44 | WebSocket binary | Binary WS message | Format detected in WS event |

### Network Body E2E

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 45 | Large body truncation | Fetch 1MB response | Body truncated at limit, no crash |
| 46 | Binary preservation | Fetch binary data | Binary intact, not corrupted |
| 47 | Auth header stripped | Request with Authorization | Header not in captured data |
| 48 | POST body captured | POST with JSON body | Request body in capture |
| 49 | Error body captured | 500 response with body | Error body captured |

### Reproduction Enhancements

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 50 | Screenshot insertion | Generate with include_screenshots:true | Script has screenshot calls |
| 51 | Fixture generation | Generate with generate_fixtures:true | fixtures/ file created |
| 52 | Visual assertions | Generate with visual_assertions:true | toHaveScreenshot calls in script |
| 53 | Options default off | Generate with no options | No screenshots, fixtures, or visual assertions |

---

## Integration Tests

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 54 | Full pilot workflow | Enable toggle → highlight → save state → execute JS | All work in sequence |
| 55 | Toggle disable mid-session | Enable, use tools, disable, try again | Tools fail after disable |
| 56 | Multiple tabs | Open 2 tabs, use pilot in each | Both work independently |
| 57 | Extension reload | Use pilot, reload extension, use again | Works after reload (toggle persists) |
| 58 | Server restart | Use pilot, restart Go server, use again | Works after restart |

---

## Performance Tests

| # | Test Case | Steps | Expected |
|---|-----------|-------|----------|
| 59 | Highlight latency | Time from call to overlay visible | < 100ms |
| 60 | Execute latency | Time for simple expression | < 50ms |
| 61 | State save latency | Save 100KB state | < 200ms |
| 62 | State load latency | Load 100KB state | < 200ms |

---

## Sign-off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Safety Toggle | | | |
| highlight_element | | | |
| manage_state | | | |
| execute_javascript | | | |
| Binary Detection | | | |
| Network E2E | | | |
| Reproduction | | | |
| Integration | | | |
| Performance | | | |

**Release Approved:** [ ] Yes / [ ] No

**Approved By:** ______________________ **Date:** __________

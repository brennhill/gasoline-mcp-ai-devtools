# Test Generation v2 (Feature 6) - Technical Review

**Reviewer:** Principal Engineer
**Spec:** `/docs/generate-test-specification.md`
**Date:** 2026-01-26

---

## Executive Summary

The `generate_test` specification proposes a compelling "witness not recorder" approach that transforms passive browser observation into assertion-rich Playwright/Cypress tests. The design is architecturally sound for the stated performance budgets, but the implementation underestimates complexity in three critical areas: (1) the causal window correlation algorithm has O(n*m) time complexity that will exceed the 50ms budget on busy sessions, (2) the spec lacks memory accounting for the string-heavy test generation pipeline that can easily exceed 50KB when combining network bodies with assertions, and (3) the `assertions` object introduces a wide API surface with 13 independent boolean/array fields that creates combinatorial testing burden. The implementation should be staged: ship with a narrower API surface first, then expand based on usage patterns.

---

## 1. Performance Analysis

### 1.1 Timeline Correlation Complexity

**Section:** "Correlation strategy" (lines 124-130)

The spec describes a 5-step correlation that attributes events to actions within a 2-second causal window:

```
For each action, identify network requests that occurred within a 2-second causal window
```

**Problem:** This implies an O(A * (N + C + W)) algorithm where A=actions, N=network, C=console, W=websocket events. With buffer limits of 50 actions, 100 network bodies, 1000 console entries, and 500 websocket events, worst case is 50 * 1600 = 80,000 comparisons.

**Existing Code Reference:** The current `GetSessionTimeline` in `codegen.go` (lines 256-384) uses insertion sort with O(n^2) worst case:
```go
for i := 1; i < len(entries); i++ {
    for j := i; j > 0 && entries[j].Timestamp < entries[j-1].Timestamp; j-- {
        entries[j], entries[j-1] = entries[j-1], entries[j]
    }
}
```

**Recommendation:** Pre-sort all buffers once (O(n log n)), then use binary search to find the causal window boundaries for each action. This reduces correlation to O(A * log(N+C+W)) and stays well within 50ms budget.

### 1.2 String Generation Memory Pressure

**Section:** "Performance Budget" (lines 427-434)

The spec budgets 50KB output and < 100ms for script generation. However, the generation pipeline allocates multiple intermediate strings:

1. Network body extraction from `capture.networkBodies` (each up to 16KB)
2. Response shape extraction via `extractResponseShape` (allocates new map)
3. Assertion string building per step
4. Framework-specific template expansion
5. Final script concatenation

**Existing Code Reference:** The current `generatePlaywrightScript` in `codegen.go` (line 104-108) does cap output:
```go
if len(script) > 51200 {
    script = script[:51200]
}
```

But this happens AFTER all allocations are complete. For comprehensive mode with 50 actions, each with network assertions including response shapes, the intermediate allocations can easily hit 200-500KB before truncation.

**Recommendation:**
1. Add early-out size estimation before generating each step
2. Use `strings.Builder` with pre-allocated capacity
3. Consider streaming generation that caps mid-generation rather than post-hoc truncation

### 1.3 GC Pressure from Response Shape Extraction

**Section:** "Shape extraction logic" (lines 312-328)

The spec shows extracting top-level properties from JSON responses. The existing `extractResponseShape` in `codegen.go` (lines 512-547) allocates recursively:

```go
func extractShape(val interface{}, depth int) interface{} {
    switch v := val.(type) {
    case map[string]interface{}:
        result := make(map[string]interface{})  // allocation per level
        for key, value := range v {
            result[key] = extractShape(value, depth+1)  // recursive allocation
        }
        return result
    // ...
    }
}
```

For 100 network bodies with average 10 fields each, this creates ~1000+ map allocations during a single tool invocation.

**Recommendation:** For top-level-only assertions (the default), skip recursive descent entirely. Parse JSON once, extract only root keys, discard the parsed structure immediately.

---

## 2. Concurrency Analysis

### 2.1 Lock Contention During Generation

**Section:** "Data Sources" (lines 104-121)

The spec states generation reads from four server-side buffers. The existing codebase uses a single `sync.RWMutex` on `Capture`:

```go
type Capture struct {
    mu sync.RWMutex
    // ...wsEvents, networkBodies, enhancedActions, etc.
}
```

**Problem:** The generation pipeline will hold a read lock while:
1. Copying all relevant buffer data
2. Correlating events (potentially 80,000 comparisons)
3. Building output strings

During this time, the extension cannot POST new events because `HandleNetworkBodies`, `HandleWebSocketEvents`, etc. all need write locks.

**Existing Pattern Reference:** The current tool handlers do the right thing - copy under lock, process outside:
```go
// From tools.go lines 1565-1568
h.capture.mu.RLock()
bodies := make([]NetworkBody, len(h.capture.networkBodies))
copy(bodies, h.capture.networkBodies)
h.capture.mu.RUnlock()
```

**Recommendation:** Ensure `correlateSessionData` and `generatePlaywrightTest` operate ONLY on copied slices, never holding the capture lock during computation. The spec's proposed function signature is correct:
```go
func correlateSessionData(actions []EnhancedAction, network []NetworkBody, ...) []CorrelatedStep
```
Just ensure the caller copies everything before passing.

### 2.2 No Goroutine Lifecycle in Spec

**Section:** "Server-Side Implementation" (lines 437-527)

The spec does not mention any background processing. This is correct for the MCP request-response model. However, if future iterations add async features (e.g., streaming test generation, background fixture caching), the spec should establish lifecycle patterns.

**Recommendation:** Add a non-goal statement: "Test generation is synchronous within the MCP request. No background goroutines, no caching of generated tests, no streaming output."

---

## 3. Data Contract Analysis

### 3.1 Wide API Surface

**Section:** "assertions Object" (lines 66-93)

The nested assertions object has 13 independent configuration fields:
- `network.enabled`, `network.status_codes`, `network.response_shape`, `network.timing_budget_ms`, `network.exclude_urls`
- `console.enabled`, `console.fail_on_errors`, `console.fail_on_warnings`, `console.ignore_patterns`
- `dom.enabled`, `dom.visibility`, `dom.text_content`, `dom.element_count`
- `websocket.enabled`, `websocket.connection_lifecycle`, `websocket.message_shape`

**Problem:** This creates 2^13 = 8,192 potential configuration combinations. Testing all meaningful combinations is impractical. Real-world usage will likely cluster around 3-5 patterns.

**Recommendation:** Ship v1 with only the `style` parameter (`comprehensive`, `smoke`, `minimal`). This maps to 3 fixed assertion profiles:

```go
var assertionProfiles = map[string]AssertionConfig{
    "comprehensive": {Network: {...all enabled...}, ...},
    "smoke":         {Network: {...status only...}, ...},
    "minimal":       {/* no assertions */},
}
```

Add fine-grained `assertions` object in v2 only if users request specific overrides.

### 3.2 Breaking Change Risk

**Section:** "Tool Name" (line 49)

The spec uses the existing tool name `generate_test`. The current implementation in `codegen.go` (lines 638-668) has a different parameter schema:

```go
// Current implementation
var arguments struct {
    TestName            string `json:"test_name"`
    AssertNetwork       bool   `json:"assert_network"`
    AssertNoErrors      bool   `json:"assert_no_errors"`
    AssertResponseShape bool   `json:"assert_response_shape"`
    BaseURL             string `json:"base_url"`
}
```

The new spec adds `framework`, `style`, `assertions`, `url_filter`, `last_n_actions` and changes behavior of existing params.

**Problem:** Existing Claude Code prompts that call `generate_test` will break or behave unexpectedly.

**Recommendation:** Either:
1. Version the tool: `generate_test_v2` (preferred for TDD workflow - can ship incrementally)
2. Make ALL new parameters additive with backwards-compatible defaults
3. Add schema version field that changes output format

### 3.3 Type Safety for JSON Parameters

**Section:** "assertions Object" (lines 66-93)

The `timing_budget_ms` is typed as `*int` (nullable), `exclude_urls` as `[]string`, `ignore_patterns` as `[]string`. JSON unmarshaling of these complex nested objects is error-prone.

**Existing Code Reference:** The codebase uses flat argument structs with direct unmarshal:
```go
var arguments struct {
    TestName string `json:"test_name"`
    // ...flat fields...
}
_ = json.Unmarshal(args, &arguments)
```

**Recommendation:** Define explicit Go types for the full config hierarchy as shown in the spec (lines 476-527). Add validation functions that return meaningful errors, not silent zero-value defaults.

---

## 4. Error Handling Analysis

### 4.1 Buffer Overflow Handling

**Section:** "Edge Cases & Limitations" (lines 399-403)

The spec acknowledges buffer overflow:
> Enhanced actions buffer: 50 actions max. If the session exceeds this, only the most recent 50 are available.

**Problem:** The spec says "Test name includes a warning comment" but doesn't specify the format or how the AI/user is notified that the test is incomplete.

**Recommendation:** Add structured metadata to the output:

```go
type TestGenerationResult struct {
    Script       string `json:"script"`
    Warnings     []string `json:"warnings,omitempty"`
    BufferStatus struct {
        ActionsOverflow  bool `json:"actions_overflow"`
        NetworkOverflow  bool `json:"network_overflow"`
        ConsoleOverflow  bool `json:"console_overflow"`
    } `json:"buffer_status"`
}
```

Return this as JSON when any buffer overflowed, allowing the AI to warn the user.

### 4.2 Correlation Failure Modes

**Section:** "Causal Window" (lines 133-137)

The 2-second window is arbitrary and will fail for:
- Long-running API calls (file uploads, AI model inference)
- Debounced inputs (search-as-you-type with 500ms debounce + network)
- WebSocket messages that arrive asynchronously

**Problem:** The spec says "comment notes uncorrelated network activity" but doesn't define what "uncorrelated" means or how to handle it.

**Recommendation:** Define explicit correlation states:
1. `correlated` - Event within window of an action
2. `orphaned` - Event before first action or after last action
3. `ambiguous` - Event in overlapping windows of multiple actions

For `ambiguous`, attribute to the FIRST action in the overlap (most likely cause).

### 4.3 Silent Failures in Shape Extraction

**Section:** "Shape extraction logic" (lines 312-328)

The existing `extractResponseShape` returns `nil` on parse error:
```go
if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
    return nil
}
```

**Problem:** Invalid JSON in captured network bodies will silently produce no assertions for that response, with no indication to the user.

**Recommendation:** Track parse failures and include in output:
```go
// Generated test includes comment:
// Note: Response body for POST /api/users was not valid JSON (skipped shape assertions)
```

---

## 5. Security Analysis

### 5.1 Sensitive Data in Generated Tests

**Section:** "Password Fields" (lines 421-425)

The spec correctly notes password redaction:
> Generated test uses `'test-password'` placeholder with a comment.

**Problem:** Other sensitive fields are not addressed:
- API keys in headers (already stripped by existing code)
- Tokens in URL query parameters
- PII in request/response bodies (names, emails, addresses)

**Existing Code Reference:** The codebase has `RedactionEngine` (`tools.go` line 152) but it's applied to tool RESPONSES, not generated test content.

**Recommendation:** Apply the same `RedactionEngine` patterns to:
1. Generated fixture data (response bodies)
2. Request bodies that appear in test scripts
3. URL query parameters in assertions

### 5.2 XSS in Generated Test Output

**Section:** "Output Format: Playwright" (lines 159-211)

The generated Playwright code uses `escapeJSString` for values:
```go
func escapeJSString(s string) string {
    s = strings.ReplaceAll(s, "\\", "\\\\")
    s = strings.ReplaceAll(s, "'", "\\'")
    s = strings.ReplaceAll(s, "\n", "\\n")
    s = strings.ReplaceAll(s, "\r", "\\r")
    return s
}
```

**Problem:** This doesn't escape `</script>` sequences. If a captured network body contains `</script><script>evil()`, it could break out of generated code when viewed in certain contexts.

**Risk Level:** Low - the test file is saved to disk and executed by Playwright, not rendered in a browser. But defense-in-depth applies.

**Recommendation:** Add:
```go
s = strings.ReplaceAll(s, "</", "<\\/")
```

### 5.3 File Path Injection

**Section:** "MCP Tool Interface" (line 62)

The `base_url` parameter is used in `replaceOrigin`:
```go
func replaceOrigin(original, baseURL string) string {
    // ...
    return base + path
}
```

**Problem:** If `baseURL` is not a valid URL, this could produce malformed output. More critically, if the generated fixtures are saved with `save_to` (mentioned in "Future Iterations"), path traversal is a risk.

**Recommendation:** Validate `base_url` is a well-formed URL with scheme and host before using. For any file-writing features, use `filepath.Clean` and restrict to allowed directories.

---

## 6. Maintainability Analysis

### 6.1 Dual Framework Support

**Section:** "Output Format: Playwright" and "Output Format: Cypress" (lines 159-293)

Supporting both Playwright and Cypress doubles the code surface:
- `generatePlaywrightTest`
- `generateCypressTest`
- `generateNetworkAssertions` needs `framework string` parameter
- `generateConsoleAssertions` needs `framework string` parameter
- etc.

**Problem:** Every assertion strategy requires two implementations that must be kept in sync. Feature drift between frameworks is likely.

**Recommendation:** Consider template-based generation:
```go
type TestTemplate struct {
    Import           string
    TestWrapper      func(name, body string) string
    ConsoleSetup     string
    NetworkAssertion func(url string, status int) string
    // ...
}

var playwrightTemplate = TestTemplate{...}
var cypressTemplate = TestTemplate{...}
```

This isolates framework differences to a single configuration object.

### 6.2 Testing Surface

**Section:** "TDD test cases for all assertion strategies" (line 445)

The spec mentions adding tests to `v4_test.go`. The combinatorial complexity of:
- 3 frameworks (Playwright, Cypress, future)
- 3 styles (comprehensive, smoke, minimal)
- 4 event types (network, console, DOM, WebSocket)
- 2 outcomes each (present, absent)

...creates 3 * 3 * 4 * 2 = 72 test cases for basic coverage, not counting edge cases.

**Recommendation:** Use table-driven tests with fixture files:
```go
func TestGenerateTest(t *testing.T) {
    cases := []struct {
        name     string
        input    string // path to input fixture
        expected string // path to golden output
    }{
        {"playwright_comprehensive_all_events", "testdata/full-session.json", "testdata/playwright-comprehensive.golden.ts"},
        // ...
    }
}
```

This scales better than inline test cases and makes it easy to regenerate goldens when output format changes intentionally.

### 6.3 Future Extension Points

**Section:** "Future Iterations" (lines 537-559)

The spec lists v2 and v3 features that will require architectural changes:
- `response_shape_depth` - needs recursive depth tracking
- Visual snapshot integration - needs Playwright config output
- Multi-tab/window - needs tab ID correlation
- AI-enhanced assertion selection - needs Claude API integration

**Problem:** None of these have extension points in the v1 design.

**Recommendation:** Add unexported struct fields and interfaces now that v2/v3 can hook into:

```go
type GenerateTestOpts struct {
    // v1 fields...

    // Extension points (unexported, for future versions)
    responseShapeExtractor func(body string, depth int) interface{}
    assertionRanker        func(assertions []Assertion) []Assertion
}
```

---

## 7. Implementation Roadmap

Based on the analysis above, I recommend the following staged implementation:

### Phase 1: Core Infrastructure (2-3 days)
1. Define all Go types from spec (lines 476-527) in `types.go`
2. Implement `correlateSessionData` with O(n log n) sorting + binary search
3. Add unit tests for correlation with edge cases (empty buffers, single action, overlapping windows)
4. Verify timeline generation stays under 50ms with worst-case data

### Phase 2: Playwright Generation (2-3 days)
1. Implement `generatePlaywrightTest` with only `style` parameter (no fine-grained `assertions`)
2. Add network status assertions for `comprehensive` style
3. Add console error tracking setup
4. Implement response shape extraction (top-level only)
5. Add golden file tests for each style mode

### Phase 3: Integration & Hardening (1-2 days)
1. Wire up MCP tool handler (`toolGenerateTest` replacement)
2. Add buffer overflow warnings to output
3. Apply `RedactionEngine` to generated fixture data
4. Add output size capping with early-exit
5. Test with real captured sessions from demo app

### Phase 4: Cypress Support (1-2 days)
1. Implement Cypress template generation
2. Mirror all Playwright tests for Cypress
3. Ensure framework-specific idioms are correct (cy.intercept vs page.waitForResponse)

### Phase 5: Documentation & Polish (1 day)
1. Update MCP tool schema with new parameters
2. Add inline help text for each style mode
3. Document breaking changes from current `generate_test`
4. Add troubleshooting guide for common issues

### Deferred to v2
- Fine-grained `assertions` object (wait for user feedback on style modes)
- `timing_budget_ms` assertions
- DOM assertions beyond navigation
- WebSocket assertions

---

## 8. Critical Issues (Must Fix Before Implementation)

| # | Issue | Section | Severity | Recommendation |
|---|-------|---------|----------|----------------|
| 1 | O(n*m) correlation complexity | 1.1 | High | Use sorted buffers + binary search |
| 2 | Wide API surface (13 params) | 3.1 | High | Ship with `style` only, defer `assertions` object |
| 3 | Breaking changes to existing tool | 3.2 | High | Version as `generate_test_v2` or ensure backwards compatibility |
| 4 | No structured error output | 4.1 | Medium | Return JSON with warnings and buffer status |
| 5 | PII in generated fixtures | 5.1 | Medium | Apply RedactionEngine to output |

---

## 9. Recommendations (Should Consider)

| # | Recommendation | Section | Effort | Impact |
|---|----------------|---------|--------|--------|
| 1 | Template-based framework generation | 6.1 | Medium | Reduces maintenance burden |
| 2 | Table-driven golden file tests | 6.2 | Low | Scales test coverage |
| 3 | Pre-allocated strings.Builder | 1.2 | Low | Reduces GC pressure |
| 4 | Structured correlation states | 4.2 | Medium | Better error messaging |
| 5 | Extension points for v2 | 6.3 | Low | Future-proofs design |

---

## Appendix: Existing Code References

| File | Function | Relevance |
|------|----------|-----------|
| `codegen.go:21-109` | `generatePlaywrightScript` | Current reproduction script generator |
| `codegen.go:256-384` | `GetSessionTimeline` | Timeline correlation logic |
| `codegen.go:512-547` | `extractResponseShape` | JSON shape extraction |
| `codegen.go:638-668` | `toolGenerateTest` | Current MCP handler |
| `reproduction.go` | `generateEnhancedPlaywrightScript` | Enhanced script with fixtures |
| `types.go:321-344` | Buffer constants | `maxEnhancedActions=50`, etc. |
| `memory.go` | Memory enforcement | Eviction policies |
| `tools.go:152` | `redactionEngine` | Sensitive data scrubbing |

---

*Review complete. Ready for implementation planning discussion.*

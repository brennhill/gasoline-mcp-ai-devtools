# QA Plan: Dynamic Tool Exposure

> QA plan for the Dynamic Tool Exposure feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Gasoline runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | `_meta.data_counts` reveals volume of user activity | Verify data counts expose aggregate buffer sizes (e.g., `errors: 3`), not specific content or identifiable patterns | medium |
| DL-2 | Available modes reveal which features are in use | Verify enum filtering exposes generic capability names (e.g., "network", "errors"), not specific URL patterns or app state | low |
| DL-3 | Progressive disclosure state leaks session progress | Verify the "first observation" flag does not expose when the session started or how active the user has been | low |
| DL-4 | Tool descriptions in `tools/list` contain sensitive defaults | Verify tool descriptions and parameter schemas do not embed example data from actual user sessions | medium |
| DL-5 | `_meta` field exposes internal server state | Verify `_meta` contains only data counts and available modes, not internal struct field names or memory addresses | medium |
| DL-6 | Enum values in filtered lists reveal data absence | Verify that missing enum values (modes not available) do not allow inferring what the user has NOT done | low |

### Negative Tests (must NOT leak)
- [ ] No specific URLs, error messages, or user data in `tools/list` response
- [ ] No internal Go struct names or implementation details in `_meta` field
- [ ] No session timing information (when did observation happen) in tool list
- [ ] No user activity patterns derivable from data count changes over time
- [ ] No private tool names or hidden features exposed in filtered lists

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Missing tools = not available yet, not broken | AI understands tools missing from `tools/list` are hidden due to no data, not removed or erroring | [ ] |
| CL-2 | Progressive disclosure is transparent | AI understands "only observe and query_dom available" means "call observe first to unlock other tools" | [ ] |
| CL-3 | `_meta.data_counts` meaning | AI uses data counts to prioritize which modes to call (e.g., `errors: 3` means 3 errors to review) | [ ] |
| CL-4 | Available modes vs all possible modes | AI understands `observe.what` enum showing only `["errors", "logs", "network"]` means other modes have no data yet | [ ] |
| CL-5 | `_meta` absence for configure tool | AI understands `configure` with no `_meta` means counts are not applicable, not that something is wrong | [ ] |
| CL-6 | First observation semantics | AI understands it needs to make at least one `observe` call before `analyze`, `generate`, `configure` appear | [ ] |
| CL-7 | "always available" modes | AI understands `errors`, `logs`, `page` are always in `observe` enum even with no data (data may arrive at any time) | [ ] |
| CL-8 | Data counts change between calls | AI understands `tools/list` data counts reflect current state, not a fixed snapshot | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might think missing tools are permanently unavailable -- verify progressive disclosure is explained
- [ ] AI might call `tools/list` repeatedly expecting it to unlock tools -- verify only `observe` calls unlock
- [ ] AI might assume `data_counts` are cumulative across sessions -- verify they are current buffer sizes
- [ ] AI might not realize `configure` is always available once disclosure lifts -- verify consistency
- [ ] AI might treat `_meta` as a required field and error when absent -- verify it is optional

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low (transparent to users)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Discover available tools | 1 step: `tools/list` (MCP standard) | No -- already the MCP standard |
| Unlock all tools | 1 step: make any `observe` call | No -- already minimal |
| Check data availability | 0 steps: `_meta.data_counts` included automatically | No -- zero-config |
| Use a specific tool mode | 1 step: call the tool (if mode is available) | No -- standard MCP flow |

### Default Behavior Verification
- [ ] Fresh server shows only `observe` and `query_dom` (progressive disclosure active)
- [ ] After first `observe` call, all tools with data appear automatically
- [ ] `configure` always shows all modes once progressive disclosure lifts
- [ ] `_meta` field appears automatically where applicable (no opt-in)
- [ ] `tools/list` call itself does NOT count as an observation
- [ ] Server restart resets progressive disclosure flag

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Fresh server tools/list | No data in any buffer | Only `observe` (errors, logs, page) and `query_dom` in response | must |
| UT-2 | Progressive disclosure: first observe call | Call `observe(what: "errors")` | `toolHandler.hasObserved` set to true | must |
| UT-3 | Post-observe tools/list | After first observe, some buffers populated | `observe`, `analyze`, `generate`, `configure`, `query_dom` all present | must |
| UT-4 | tools/list call does not count as observe | Only call `tools/list` | Progressive disclosure still active (only 2 tools) | must |
| UT-5 | Observe enum filtering - network available | `capture.networkBodies` has entries | `observe.what` enum includes "network" | must |
| UT-6 | Observe enum filtering - network unavailable | `capture.networkBodies` is empty | `observe.what` enum excludes "network" | must |
| UT-7 | Observe enum filtering - always-available modes | No data at all | `observe.what` enum still includes "errors", "logs", "page" | must |
| UT-8 | Analyze enum filtering - performance available | `capture.perf.snapshots` non-empty | `analyze.target` enum includes "performance" | must |
| UT-9 | Analyze enum filtering - api available | `capture.schemaStore` has endpoints | `analyze.target` enum includes "api" | must |
| UT-10 | Analyze enum filtering - accessibility always available | No a11y data cached | `analyze.target` enum still includes "accessibility" (live trigger) | must |
| UT-11 | Generate enum filtering - reproduction available | `capture.enhancedActions` non-empty | `generate.format` enum includes "reproduction" and "test" | must |
| UT-12 | Generate enum filtering - har available | `capture.networkBodies` non-empty | `generate.format` enum includes "har" | must |
| UT-13 | Generate enum filtering - sarif always available | No a11y data | `generate.format` enum still includes "sarif" (live trigger) | must |
| UT-14 | Configure always shows all modes | After progressive disclosure lifted | `configure` has full mode list regardless of data | must |
| UT-15 | _meta data counts - errors | 5 error entries in server.entries | `_meta.data_counts.errors: 5` | must |
| UT-16 | _meta data counts - network | 12 network bodies | `_meta.data_counts.network: 12` | must |
| UT-17 | _meta data counts - actions | 8 enhanced actions | `_meta.data_counts.actions: 8` | must |
| UT-18 | _meta absent for configure | configure tool in tools/list | No `_meta` field on configure tool | must |
| UT-19 | _meta data counts reflect current state | Add 3 more network bodies, call tools/list again | `_meta.data_counts.network` increased by 3 | must |
| UT-20 | Concurrency: tools/list under load | tools/list called while data is being written to buffers | No deadlock, consistent response (read locks on server.mu and capture.mu) | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | MCP protocol flow: list -> observe -> list | MCP client sends tools/list, then observe, then tools/list | Second tools/list has more tools than first | must |
| IT-2 | Extension data arrival updates tool availability | Extension sends network data via WS | Next tools/list includes "network" mode | must |
| IT-3 | Server restart resets progressive disclosure | tools/list with all tools -> restart -> tools/list | Only observe + query_dom visible | must |
| IT-4 | Multiple MCP clients see same tool list | Two MCP clients connected | Both see identical tools/list results | should |
| IT-5 | Data counts update in real-time | Extension sends 5 errors, then tools/list | `_meta.data_counts.errors: 5` reflects current count | must |
| IT-6 | Full progressive disclosure lifecycle | fresh -> observe -> all tools visible -> data arrives -> specific modes appear | Complete lifecycle works end-to-end | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | tools/list response time | Time to compute filtered tool list with data counts | < 1ms | must |
| PT-2 | No unnecessary allocations | Memory allocated by tools/list | Only response construction, no buffer copies | must |
| PT-3 | Lock contention | Impact on concurrent observe/tools/list calls | Negligible (tools/list uses read locks only) | must |
| PT-4 | Repeated tools/list calls | 100 tools/list calls in 1 second | All respond in < 1ms, no degradation | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | All observe modes empty (except always-available) | Fresh server, no data | `observe` shows only errors, logs, page; other tools hidden | must |
| EC-2 | Progressive disclosure active, 2 tools only | No observe call made yet | tools/list returns exactly observe + query_dom | must |
| EC-3 | Data arrives but no observe call yet | Network bodies arrive from extension | tools/list still shows only 2 tools (disclosure not lifted) | must |
| EC-4 | observe call with empty result | `observe(what: "errors")` returns 0 errors | Progressive disclosure still lifted (call was made, not "had data") | must |
| EC-5 | All buffers populated | Every buffer type has data | All enum values present for all tools | must |
| EC-6 | Buffer emptied between tools/list calls | Network bodies buffer cleared | Next tools/list excludes "network" from observe enum | must |
| EC-7 | Concurrent buffer writes during tools/list | Data arriving while tools/list computes | Read locks prevent inconsistent state; response is atomic snapshot | must |
| EC-8 | Zero data counts | observe called but buffers empty | `_meta.data_counts` shows zeros for non-always-available modes | should |
| EC-9 | Very large data counts | 10000 entries in a buffer | `_meta.data_counts` shows 10000 accurately | should |
| EC-10 | Tool with zero available modes | hypothetical: all modes require data, none available | Tool omitted entirely from tools/list response | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Gasoline server running: `./dist/gasoline --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web app generating various types of browser data (errors, network, actions, performance)
- [ ] MCP client capable of calling `tools/list`

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Start fresh Gasoline server | No browser data captured yet | Server starts clean | [ ] |
| UAT-2 | Call `tools/list` via MCP | AI receives tool list | Only 2 tools: `observe` (with errors, logs, page) and `query_dom` | [ ] |
| UAT-3 | `{"tool": "observe", "arguments": {"what": "errors"}}` | AI makes first observation | Response received (possibly empty errors) | [ ] |
| UAT-4 | Call `tools/list` again | AI receives expanded tool list | `observe`, `analyze`, `generate`, `configure`, `query_dom` all present | [ ] |
| UAT-5 | Navigate to test app, generate network traffic | Network tab shows API calls | Extension captures network data | [ ] |
| UAT-6 | Call `tools/list` | AI receives tool list | `observe.what` enum now includes "network"; `generate.format` includes "har" | [ ] |
| UAT-7 | Verify `_meta.data_counts` | Inspect `_meta` field on `observe` tool | Counts match: `errors: N`, `network: M`, etc. reflecting actual buffer sizes | [ ] |
| UAT-8 | Click around the test app to generate user actions | Human interacts with the app | Enhanced actions captured by extension | [ ] |
| UAT-9 | Call `tools/list` | AI receives updated tool list | `observe.what` includes "actions"; `analyze.target` includes "timeline"; `generate.format` includes "reproduction", "test" | [ ] |
| UAT-10 | Trigger a performance snapshot (page load) | Page loads completely | Performance data captured | [ ] |
| UAT-11 | Call `tools/list` | AI receives tool list | `observe.what` includes "vitals"; `analyze.target` includes "performance" | [ ] |
| UAT-12 | Verify `configure` tool completeness | Check `configure` in tool list | All modes present, no `_meta` field | [ ] |
| UAT-13 | Restart server, call `tools/list` | Human restarts Gasoline | Only 2 tools visible again (progressive disclosure reset) | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | No user data in tools/list | Inspect full tools/list response | Only tool schemas, enum values, and counts -- no URLs, errors, or content | [ ] |
| DL-UAT-2 | _meta contains only counts | Inspect `_meta.data_counts` values | Integer counts only, no strings, URLs, or identifiable data | [ ] |
| DL-UAT-3 | Tool descriptions are static | Compare tool descriptions across sessions | Descriptions do not change based on user data (no dynamic examples) | [ ] |
| DL-UAT-4 | Missing modes do not reveal specifics | Note which modes are absent | Absence only indicates "no data of this type", not what specific data is missing | [ ] |

### Regression Checks
- [ ] MCP clients that ignore `_meta` field still work correctly
- [ ] AI agents that call `tools/list` once at startup still discover tools via errors when calling unavailable modes
- [ ] Existing tool call behavior unchanged (only tools/list response changes)
- [ ] No performance degradation on tools/list vs previous static response
- [ ] `configure` tool functionality unaffected by dynamic exposure logic

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

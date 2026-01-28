# QA Plan: Configuration Profiles

> QA plan for the Configuration Profiles feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Configuration profiles bundle capture settings and are persisted to disk. The primary risks involve profile files leaking sensitive configuration, path traversal via profile names, profile settings exposing internal server state, and profiles weakening security posture.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Path traversal via profile name | Profile names like `../../etc/passwd` or `../meta` could read/write outside `.gasoline/profiles/` -- verify `validateStoreInput` prevents this | critical |
| DL-2 | Profile files on disk contain sensitive data | Custom profile files in `.gasoline/profiles/` should contain only capture setting key-value pairs, never credentials or tokens | critical |
| DL-3 | Built-in profile overwrite by name collision | An agent saves a custom profile named "debug" to override the built-in -- verify built-in names are protected | critical |
| DL-4 | Profile settings expose redaction configuration | The `restricted` and `paranoid` built-in profiles include redaction patterns (SSN, credit card regex) -- verify exposing these patterns in list/get responses is acceptable | high |
| DL-5 | Profile export includes resolved settings | Export includes fully-merged settings which could reveal inherited redaction rules from parent profiles the agent was not aware of | medium |
| DL-6 | Custom profile description field as free-text | Description is user-provided free-text stored to disk -- verify no injection risk (JSON file, not executed) | medium |
| DL-7 | Profile locking bypass via runtime overrides | When `GASOLINE_PROFILE_LOCKED=true`, runtime overrides are still allowed -- verify overrides cannot weaken security-critical settings (redaction level, tool gating) | high |
| DL-8 | Corrupted profile file read leaks file content | If a profile JSON file is malformed, verify the error message does not include raw file content in the MCP response | medium |
| DL-9 | Profile inheritance chain reveals internal structure | Deep inheritance (e.g., custom -> paranoid -> restricted -> default) reveals the profile hierarchy -- verify this is acceptable | low |
| DL-10 | Previous overrides leaked in load response | The `previous_overrides` field in load response shows what settings were active before -- verify this is intentional and not sensitive | medium |

### Negative Tests (must NOT leak)
- [ ] Profile name `../../etc/passwd` is rejected by name validation
- [ ] Profile name `../meta` is rejected as path traversal
- [ ] Profile name with path separator `/` or `\` is rejected
- [ ] Saving a profile named `debug`, `performance`, or `minimal` (built-in names) returns an error
- [ ] Profile files on disk contain only JSON with capture setting keys, no process environment or credentials
- [ ] When `GASOLINE_PROFILE_LOCKED=true`, attempts to activate a different profile return an error
- [ ] Export response does not include server-internal fields (mutex state, cache pointers, etc.)
- [ ] Corrupted profile file errors do not include raw file content in MCP error messages

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Profile action vs. profile_action distinction | AI understands `action: "profile"` selects the profile feature, and `profile_action` selects the sub-operation (save/load/list/get/delete) | [ ] |
| CL-2 | Built-in vs. custom profile distinction | `builtin: true/false` field clearly indicates immutability | [ ] |
| CL-3 | "loaded" vs. "saved" status semantics | AI understands "loaded" means settings applied, "saved" means profile persisted to disk | [ ] |
| CL-4 | `previous_overrides` in load response | AI understands these are the overrides that were replaced, useful for understanding what changed | [ ] |
| CL-5 | Partial profile settings | AI understands a profile may specify only some of the 5 capture settings; unspecified settings are unchanged | [ ] |
| CL-6 | Active profile cleared on manual change | AI understands that calling `configure(action: "capture")` after loading a profile clears the active profile name | [ ] |
| CL-7 | Profile not persisting active state on restart | AI understands the active profile is session-scoped; after restart, no profile is active and defaults apply | [ ] |
| CL-8 | Error messages include remediation hints | Error messages like "Profile not found" include suggestion to call `list` action | [ ] |
| CL-9 | Performance vs. minimal profile distinction | AI understands these have identical settings today but different intended use cases | [ ] |
| CL-10 | `persisted: true` in save response | AI understands the profile was written to disk and survives restart (vs. session-only) | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might think loading a profile modifies the profile definition -- verify response clarifies loading applies settings, does not change the profile
- [ ] AI might think deleting an active profile removes the current capture settings -- verify response explains overrides remain but active profile name is cleared
- [ ] AI might confuse the PRODUCT_SPEC (5 capture settings) with the TECH_SPEC (broader settings including buffers, redaction, tools) -- verify the profile scope is clearly bounded in responses
- [ ] AI might try to save a profile with no settings and no active overrides -- verify error message explains both paths (explicit settings or snapshot overrides)
- [ ] AI might think `performance` and `minimal` profiles behave differently -- verify list response includes descriptions that explain the semantic difference

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Activate a built-in profile | 1 step: `configure(action: "profile", profile_action: "load", name: "debug")` | No -- already minimal |
| Save current settings as profile | 1 step: `configure(action: "profile", profile_action: "save", name: "my-profile")` | No -- already minimal |
| Save profile with explicit settings | 1 step: same call with `settings` object | No -- already minimal |
| List all profiles | 1 step: `configure(action: "profile", profile_action: "list")` | No -- already minimal |
| Get profile details | 1 step: `configure(action: "profile", profile_action: "get", name: "debug")` | No -- already minimal |
| Delete a custom profile | 1 step: `configure(action: "profile", profile_action: "delete", name: "my-profile")` | No -- already minimal |
| Full "apply debug settings" workflow | 1 step: load the debug profile | Yes, this is the entire value proposition -- replaces 5 individual capture setting changes |
| Switch from debug to minimal | 1 step: load the minimal profile | No -- already minimal |

### Default Behavior Verification
- [ ] Feature works with zero configuration -- built-in profiles are available immediately
- [ ] No profile is active by default (agent must explicitly load one)
- [ ] Without loading a profile, all capture settings use their normal defaults
- [ ] Built-in profiles (debug, performance, minimal) are always listed

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Built-in profiles exist on init | Create ProfileManager | `debug`, `performance`, `minimal` profiles present | must |
| UT-2 | Save custom profile | `Save("my-profile", settings, desc)` | Profile stored, `builtin: false`, `persisted: true` | must |
| UT-3 | Save profile with built-in name rejected | `Save("debug", ...)` | Error: "Cannot overwrite built-in profile: debug" | must |
| UT-4 | Load existing profile | `Load("debug")` | Settings applied as capture overrides, response includes `settings_applied` | must |
| UT-5 | Load non-existent profile | `Load("nonexistent")` | Error: "Profile not found: nonexistent" with hint to use list | must |
| UT-6 | List returns all profiles | `List()` | Returns built-in and custom profiles with names, descriptions, builtin flag | must |
| UT-7 | List shows active profile | Load "debug", then `List()` | Response `active` field = "debug" | must |
| UT-8 | Get built-in profile details | `Get("debug")` | Returns full profile definition with all 5 settings | must |
| UT-9 | Delete custom profile | `Delete("my-profile")` | Profile removed, not in list | must |
| UT-10 | Delete built-in profile rejected | `Delete("debug")` | Error: "Cannot delete built-in profile: debug" | must |
| UT-11 | Delete non-existent profile | `Delete("nonexistent")` | Error: "Profile not found: nonexistent" | must |
| UT-12 | Profile name validation -- valid | Names: "debug", "my-profile", "test_123" | All accepted | must |
| UT-13 | Profile name validation -- path traversal | Names: "../hack", "../../etc", "foo/bar" | All rejected with appropriate error | must |
| UT-14 | Profile name validation -- too long | Name with 51+ characters | Rejected: max 50 characters | must |
| UT-15 | Profile name validation -- empty | Name: "" | Rejected | must |
| UT-16 | Profile name validation -- special chars | Names with spaces, `@`, `#`, etc. | Rejected: alphanumeric, hyphens, underscores only | must |
| UT-17 | Save without settings snapshots current overrides | Set capture overrides, then `Save("snapshot", nil, "")` | Profile saved with current override values | should |
| UT-18 | Save without settings and no overrides fails | No overrides active, `Save("empty", nil, "")` | Error: "No settings provided and no active capture overrides to snapshot" | should |
| UT-19 | Load profile replaces current overrides | Set overrides A, load profile with overrides B | Active overrides are now B, response shows `previous_overrides` = A | must |
| UT-20 | Manual capture change clears active profile | Load "debug", then change log_level manually | Active profile name cleared (null) | should |
| UT-21 | Reset clears active profile | Load "debug", then reset capture | Active profile name cleared, all overrides removed | should |
| UT-22 | Rate limit on profile load | Load two profiles within 1 second | Second load rejected with rate limit error | must |
| UT-23 | Profile load generates audit event | Load "debug" | Audit log contains `profile_load` event with profile name and settings | must |
| UT-24 | Partial profile load | Profile with only `log_level` and `ws_mode` | Only those two settings applied as overrides, others unchanged | must |
| UT-25 | Delete active profile keeps overrides | Load "my-profile", then delete it | Overrides remain active, active profile name cleared | should |
| UT-26 | Concurrent profile operations | Multiple goroutines loading/saving | Mutex prevents race conditions | must |
| UT-27 | Maximum 50 custom profiles | Try to save 51st custom profile | Error: maximum custom profiles reached | should |
| UT-28 | Profile settings validation | Save profile with invalid log_level value | Error: invalid settings value | must |
| UT-29 | Profile file size limit | Save profile with description > 4KB | Error or truncation (max 4KB per file) | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Profile save persists to disk | ProfileManager + SessionStore + filesystem | `.gasoline/profiles/my-profile.json` created with correct JSON | must |
| IT-2 | Profile survives server restart | ProfileManager + SessionStore | Save profile, restart server, profile appears in list | must |
| IT-3 | MCP tool call round-trip: save then load | MCP dispatcher + ProfileManager + CaptureOverrides | Save via MCP, load via MCP, capture settings match | must |
| IT-4 | MCP tool call: list profiles | MCP dispatcher + ProfileManager | Returns JSON array with built-in and custom profiles | must |
| IT-5 | Profile load changes capture behavior | ProfileManager + observe tool | Load "debug" profile, observe tool returns verbose data | must |
| IT-6 | Corrupted profile file on startup | ProfileManager + filesystem | Place invalid JSON in `.gasoline/profiles/bad.json`, start server, server runs without crash, bad profile not in list | must |
| IT-7 | `.gasoline/profiles/` directory auto-created | ProfileManager + filesystem | First save creates directory if it does not exist | must |
| IT-8 | Session store not initialized | ProfileManager without SessionStore | Built-in profiles work, custom save/delete return error | should |
| IT-9 | Audit log captures profile operations | ProfileManager + audit log | Profile save, load, delete all generate audit entries | should |
| IT-10 | Profile load respects rate limit | MCP dispatcher + rate limiter | Two loads within 1 second: second returns rate limit error | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Profile load (apply settings) | Latency | < 1ms | must |
| PT-2 | Profile save (write to disk) | Latency | < 5ms | must |
| PT-3 | Profile list (read all profiles, 50 custom + 3 built-in) | Latency | < 10ms | must |
| PT-4 | Profile delete | Latency | < 5ms | must |
| PT-5 | Memory per profile (in-memory) | Memory | < 1KB per profile | should |
| PT-6 | Profile file size on disk | File size | < 4KB per profile | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Save profile with empty settings object | `settings: {}` and overrides active | Snapshots current overrides | should |
| EC-2 | Load profile after server restart (no active profile) | Restart server, check active profile | No active profile, defaults apply | must |
| EC-3 | Profile with all 5 settings | Save profile specifying all 5 capture settings | All 5 applied on load | must |
| EC-4 | Profile with 1 setting | Save profile with only `log_level: "all"` | Only log_level applied, others unchanged | must |
| EC-5 | Concurrent save and load | Goroutine A saves, goroutine B loads | Mutex serializes, no corruption | must |
| EC-6 | Load profile when rate limited | Load within 1 second of previous load | Rate limit error returned | must |
| EC-7 | Save over existing custom profile | Save "my-profile" twice with different settings | Second save overwrites first | should |
| EC-8 | Profile file manually edited on disk | Edit `.gasoline/profiles/my-profile.json` externally | Server picks up changes on next list/load (or on restart) | should |
| EC-9 | Delete profile while another agent uses it | Agent A has profile loaded, agent B deletes it | Agent A's overrides remain, active name cleared | should |
| EC-10 | Profile name with only hyphens | Name: "---" | Valid (matches alphanumeric + hyphens rule) | should |
| EC-11 | Profile name with unicode characters | Name: "profil-" | Rejected: alphanumeric + hyphens + underscores only | must |
| EC-12 | Maximum profile file size exceeded | Save profile with extremely long description | Rejected or truncated at 4KB | should |
| EC-13 | `.gasoline/profiles/` read-only directory | Save profile when directory is read-only | Error returned, server continues | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Gasoline server running: `./dist/gasoline --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web application running locally
- [ ] Write access to the project directory for profile persistence

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "configure", "arguments": {"action": "profile", "profile_action": "list"}}` | Review profile list | Response contains 3 built-in profiles: `debug`, `performance`, `minimal` with descriptions and `builtin: true`; `active` is null | [ ] |
| UAT-2 | `{"tool": "configure", "arguments": {"action": "profile", "profile_action": "get", "name": "debug"}}` | Review profile details | Response shows all 5 capture settings: log_level=all, ws_mode=messages, network_bodies=true, screenshot_on_error=true, action_replay=true | [ ] |
| UAT-3 | `{"tool": "configure", "arguments": {"action": "profile", "profile_action": "load", "name": "debug"}}` | Check capture behavior changes | Response shows `status: "loaded"`, `settings_applied` with all 5 debug settings, `previous_overrides` showing what was active before | [ ] |
| UAT-4 | `{"tool": "observe", "arguments": {"action": "get_console_logs"}}` | Compare verbosity to before profile load | Console logs now include all levels (debug profile enables log_level=all) | [ ] |
| UAT-5 | `{"tool": "configure", "arguments": {"action": "profile", "profile_action": "load", "name": "minimal"}}` | Check capture behavior changes | Response shows `status: "loaded"`, minimal settings applied (log_level=error, ws_mode=off, etc.) | [ ] |
| UAT-6 | `{"tool": "configure", "arguments": {"action": "profile", "profile_action": "save", "name": "my-test", "description": "Custom test profile", "settings": {"log_level": "warn", "network_bodies": "true"}}}` | Check `.gasoline/profiles/` directory | File `my-test.json` created in `.gasoline/profiles/` | [ ] |
| UAT-7 | `{"tool": "configure", "arguments": {"action": "profile", "profile_action": "list"}}` | Review updated list | List now includes `my-test` with `builtin: false` | [ ] |
| UAT-8 | `{"tool": "configure", "arguments": {"action": "profile", "profile_action": "load", "name": "my-test"}}` | Verify partial settings applied | Only log_level and network_bodies changed; other settings retain their current values | [ ] |
| UAT-9 | `{"tool": "configure", "arguments": {"action": "profile", "profile_action": "save", "name": "debug"}}` | None | Error returned: "Cannot overwrite built-in profile: debug" | [ ] |
| UAT-10 | `{"tool": "configure", "arguments": {"action": "profile", "profile_action": "delete", "name": "debug"}}` | None | Error returned: "Cannot delete built-in profile: debug" | [ ] |
| UAT-11 | `{"tool": "configure", "arguments": {"action": "profile", "profile_action": "delete", "name": "my-test"}}` | Check `.gasoline/profiles/` directory | File `my-test.json` removed; response shows `status: "deleted"` | [ ] |
| UAT-12 | `{"tool": "configure", "arguments": {"action": "profile", "profile_action": "load", "name": "nonexistent"}}` | None | Error: "Profile not found: nonexistent" with hint to use list action | [ ] |
| UAT-13 | `{"tool": "configure", "arguments": {"action": "profile", "profile_action": "save", "name": "../../etc/hack"}}` | None | Error: profile name contains path traversal sequence | [ ] |
| UAT-14 | Restart the Gasoline server, then call `{"tool": "configure", "arguments": {"action": "profile", "profile_action": "list"}}` | Verify persistence | Built-in profiles present; previously saved custom profiles (if any were re-saved) present; no active profile | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Path traversal prevented | Try saving profile with name `../meta` | Rejected with path traversal error | [ ] |
| DL-UAT-2 | Profile file contains only settings | Open `.gasoline/profiles/*.json` manually | File contains only name, description, settings, timestamps -- no credentials or server state | [ ] |
| DL-UAT-3 | Built-in profiles immutable | Try to save with name "debug", "performance", "minimal" | All rejected | [ ] |
| DL-UAT-4 | Profile export has no internal state | Call get on a profile | Response contains only profile definition fields, no mutex state or cache pointers | [ ] |
| DL-UAT-5 | Corrupted file does not leak content | Place malformed JSON in `.gasoline/profiles/bad.json`, restart server | Server starts, bad profile not listed, error log does not contain raw file content | [ ] |

### Regression Checks
- [ ] Existing `configure(action: "capture")` functionality works without profiles loaded
- [ ] Loading a profile does not break subsequent manual capture setting changes
- [ ] Profile feature does not interfere with noise filtering, TTL, or memory enforcement
- [ ] Server startup time not significantly affected by loading profile files from disk
- [ ] Existing capture settings are preserved when profile feature code is present but no profile is loaded

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

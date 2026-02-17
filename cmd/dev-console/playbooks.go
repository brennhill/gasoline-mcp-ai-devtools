// playbooks.go — MCP resource content: capability index, playbooks, guide, quickstart, and demo scripts.
package main

import "strings"

// capabilityIndex is the token-efficient capability discovery resource.
var capabilityIndex = `# Gasoline Capability Index (Token-Efficient)

Use this index for discovery. Load detailed guidance only when task intent matches.

## Routing Rule

1. Identify intent from user request.
2. If intent matches a capability below, read that playbook URI.
3. Prefer quick playbook first; load full playbook only when needed.

## Capability Map

| Capability | When to Use | Playbook URI |
|---|---|---|
| performance | Regressions, slow pages, bottlenecks, Core Web Vitals | gasoline://playbook/performance/quick |
| accessibility | WCAG/axe issues, semantic/contrast/navigation checks | gasoline://playbook/accessibility/quick |
| security | Credential leaks, CSP/cookie/header risks, third-party origin risk | gasoline://playbook/security/quick |
| api_validation | Contract mismatches, malformed responses, endpoint drift | See gasoline://guide |

## Available Playbook Variants

- gasoline://playbook/performance/quick
- gasoline://playbook/performance/full
- gasoline://playbook/accessibility/quick
- gasoline://playbook/accessibility/full
- gasoline://playbook/security/quick
- gasoline://playbook/security/full

## Notes

- Keep this index small; do not inline full workflows here.
- Add future playbooks under gasoline://playbook/{capability}/{quick|full}.
`

// playbooks maps "{capability}/{level}" keys to markdown playbook content.
var playbooks = map[string]string{
	"performance/quick": `# Playbook: Performance Analysis (Quick)

Use when a page feels slow or performance regressed.
If you only need a single metric (e.g. LCP), call observe(what:"vitals") directly.

## Preconditions

- Extension connected and tracked tab confirmed.
- Target URL known.

## Steps

1. {"tool":"configure","arguments":{"action":"health"}}
2. {"tool":"interact","arguments":{"action":"navigate","url":"<target-url>"}}
3. {"tool":"observe","arguments":{"what":"vitals"}}
4. {"tool":"observe","arguments":{"what":"network_waterfall","status_min":400}}
5. {"tool":"observe","arguments":{"what":"actions","last_n":30}}

## Output Format

- Top 3 bottlenecks
- Evidence (metric/request/action references)
- Lowest-risk first fixes
`,
	"performance/full": `# Playbook: Performance Analysis (Full)

Use for deep profiling and remediation planning.

## When To Use

- Perf regression after a change
- Slow initial load or interaction lag
- Need actionable fix plan with evidence

## Preconditions

- Extension connected
- Correct tracked tab
- Reproducible URL/workflow

## Steps

1. Baseline health:
   {"tool":"configure","arguments":{"action":"health"}}
   {"tool":"observe","arguments":{"what":"page"}}
2. Capture navigation perf diff:
   {"tool":"interact","arguments":{"action":"navigate","url":"<target-url>","analyze":true}}
3. Collect web vitals:
   {"tool":"observe","arguments":{"what":"vitals"}}
4. Collect network hotspots:
   {"tool":"observe","arguments":{"what":"network_waterfall","limit":200}}
5. Collect runtime signals:
   {"tool":"observe","arguments":{"what":"actions","last_n":100}}
   {"tool":"observe","arguments":{"what":"logs","min_level":"warn","last_n":200}}
6. Optional active analysis:
   {"tool":"analyze","arguments":{"what":"performance"}}
   {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from analyze>"}}

## Failure Modes

- extension_disconnected: reconnect/track tab, rerun
- no_perf_diff: ensure navigate/refresh or interact with analyze=true
- sparse_data: increase observe limits and repeat flow

## Output Format

- Summary: regression/no-regression with confidence
- Bottlenecks: ranked with concrete evidence
- Fixes: prioritized quick wins then deeper refactors
- Validation plan: exact checks to verify improvement
`,
	"accessibility/quick": `# Playbook: Accessibility Audit (Quick)

Use when you need a fast WCAG issue snapshot.
For a single element check, call analyze(what:"dom", selector:"...") directly.

## Steps

1. {"tool":"configure","arguments":{"action":"health"}}
2. {"tool":"analyze","arguments":{"what":"accessibility"}}
3. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from analyze>"}}

## Output Format

- Top accessibility blockers
- WCAG tags impacted
- Quick remediation suggestions
`,
	"accessibility/full": `# Playbook: Accessibility Audit (Full)

Use for triage plus implementation-ready fixes.

## Preconditions

- Extension connected
- Correct tracked tab

## Steps

1. {"tool":"configure","arguments":{"action":"health"}}
2. {"tool":"observe","arguments":{"what":"page"}}
3. {"tool":"analyze","arguments":{"what":"accessibility"}}
4. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from analyze>"}}
5. {"tool":"analyze","arguments":{"what":"dom","selector":"main, [role='main'], form, nav"}}
6. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from dom analyze>"}}

## Failure Modes

- extension_disconnected: reconnect and retry
- timeout: retry once, then narrow DOM scope

## Output Format

- Findings by severity
- Affected selectors/components
- Concrete code-level fix guidance
- Validation checklist
`,
	"security/quick": `# Playbook: Security Audit (Quick)

Use for fast risk screening.
For a single header/cookie check, call analyze(what:"security_audit") directly.

## Steps

1. {"tool":"configure","arguments":{"action":"health"}}
2. {"tool":"analyze","arguments":{"what":"security_audit"}}
3. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from analyze>"}}

## Output Format

- High-risk findings first
- Evidence location (header/URL/request/etc.)
- Immediate mitigations
`,
	"security/full": `# Playbook: Security Audit (Full)

Use for comprehensive browser-surface security review.

## Preconditions

- Extension connected
- Representative app flow loaded

## Steps

1. {"tool":"configure","arguments":{"action":"health"}}
2. {"tool":"observe","arguments":{"what":"network_waterfall","limit":200}}
3. {"tool":"analyze","arguments":{"what":"security_audit","severity_min":"medium"}}
4. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from security audit>"}}
5. {"tool":"analyze","arguments":{"what":"third_party_audit","first_party_origins":["<origin>"]}}
6. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from third_party_audit>"}}

## Failure Modes

- missing baseline traffic: exercise critical user flow and rerun
- noisy false positives: tighten first_party_origins

## Output Format

- Risks ranked by severity and exploitability
- Evidence and affected endpoints/origins
- Prioritized fix plan
- Verification steps
`,
}

// guideContent is the full usage guide resource.
var guideContent = `# Gasoline MCP Tools

Browser observability for AI coding agents. 5 tools for real-time browser telemetry.

## Quick Reference

| Tool | Purpose | Key Parameters |
|------|---------|----------------|
| observe | Read passive browser buffers | what: errors, logs, extension_logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, pilot, timeline, error_bundles, screenshot, command_result, pending_commands, failed_commands, saved_videos, recordings, recording_actions, log_diff_report |
| analyze | Trigger active analysis (async) | what: dom, accessibility, performance, security_audit, third_party_audit, link_health, link_validation, page_summary, error_clusters, history, api_validation, annotations, annotation_detail, draw_history, draw_session |
| generate | Create artifacts from captured data | format: test, reproduction, pr_summary, sarif, har, csp, sri, visual_test, annotation_report, annotation_issues, test_from_context, test_heal, test_classify |
| configure | Session settings and utilities | action: health, store, load, noise_rule, clear, streaming, test_boundary_start, test_boundary_end, recording_start, recording_stop, playback, log_diff |
| interact | Browser automation (needs AI Web Pilot) | action: click, type, select, check, navigate, refresh, execute_js, highlight, subtitle, key_press, scroll_to, wait_for, get_text, get_value, get_attribute, set_attribute, focus, list_interactive, save_state, load_state, list_states, delete_state, record_start, record_stop, upload, draw_mode_start, back, forward, new_tab, screenshot (alias of observe what=screenshot) |

## Key Patterns

### Check Extension Status First
Always verify the extension is connected before debugging:
  {"tool":"configure","arguments":{"action":"health"}}
If extension_connected is false, ask the user to click "Track This Tab" in the extension popup.

### Async Commands (analyze tool)
analyze dispatches queries to the extension asynchronously. Poll for results:
  1. {"tool":"analyze","arguments":{"what":"accessibility"}}  -> returns correlation_id
  2. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"..."}}

### Pagination (observe tool)
Responses include cursors in metadata. Pass back for next page:
  {"tool":"observe","arguments":{"what":"logs","after_cursor":"...","limit":50}}
Use restart_on_eviction=true if a cursor expires.

## Common Workflows

  // See errors with surrounding context (network + actions + logs)
  {"tool":"observe","arguments":{"what":"error_bundles"}}

  // Check failed network requests
  {"tool":"observe","arguments":{"what":"network_waterfall","status_min":400}}

  // Run accessibility audit (async)
  {"tool":"analyze","arguments":{"what":"accessibility"}}

  // Query DOM elements (async)
  {"tool":"analyze","arguments":{"what":"dom","selector":".error-message"}}

  // Generate Playwright test from session
  {"tool":"generate","arguments":{"format":"test","test_name":"user_login"}}

  // Check Web Vitals (LCP, CLS, INP, FCP)
  {"tool":"observe","arguments":{"what":"vitals"}}

  // Navigate and measure performance (auto perf_diff)
  {"tool":"interact","arguments":{"action":"navigate","url":"https://example.com"}}

  // Suppress noisy console errors
  {"tool":"configure","arguments":{"action":"noise_rule","noise_action":"auto_detect"}}

## Tips

- Start with configure(action:"health") to verify extension is connected
- Use observe(what:"error_bundles") instead of raw errors — includes surrounding context
- Use observe(what:"page") to confirm which URL the browser is on
- interact actions require the AI Web Pilot extension feature to be enabled
- interact navigate and refresh automatically include performance diff metrics
- Data comes from the active tracked browser tab
`

// quickstartContent is the short quickstart resource.
var quickstartContent = `# Gasoline MCP Quickstart

## 1. Health Check
{"tool":"configure","arguments":{"action":"health"}}

## 2. Confirm Tracked Page
{"tool":"observe","arguments":{"what":"page"}}

## 3. Collect Errors + Context
{"tool":"observe","arguments":{"what":"error_bundles"}}

## 4. Network Failures
{"tool":"observe","arguments":{"what":"network_waterfall","status_min":400}}

## 5. WebSocket Status
{"tool":"observe","arguments":{"what":"websocket_status"}}

## 6. Accessibility Audit (Async)
{"tool":"analyze","arguments":{"what":"accessibility"}}
{"tool":"observe","arguments":{"what":"command_result","correlation_id":"..."}}

## 7. DOM Query (Async)
{"tool":"analyze","arguments":{"what":"dom","selector":".error-message"}}
{"tool":"observe","arguments":{"what":"command_result","correlation_id":"..."}}

## 8. Performance Check
{"tool":"interact","arguments":{"action":"navigate","url":"https://example.com"}}

## 9. Start Recording
{"tool":"configure","arguments":{"action":"recording_start","name":"demo-run"}}

## 10. Stop Recording
{"tool":"configure","arguments":{"action":"recording_stop","recording_id":"..."}}
`

// demoScripts maps demo names to markdown demo script content.
var demoScripts = map[string]string{
	"ws": `# Demo: WebSocket Debugging

Goal: show mismatched message format and where to fix it.

Steps:
1. {"tool":"observe","arguments":{"what":"websocket_status"}}
2. {"tool":"observe","arguments":{"what":"websocket_events","limit":20}}
3. {"tool":"analyze","arguments":{"what":"api_validation","operation":"analyze","ignore_endpoints":["/socket"]}}

Expected:
- Connection OK, but message schema warnings
- Identify client-side parsing path for fix
`,
	"annotations": `# Demo: Usability Annotations

Goal: highlight a layout issue and collect feedback.

Steps:
1. {"tool":"interact","arguments":{"action":"draw_mode_start","session":"demo-ux"}}
2. Ask user to annotate oversized image and desired size.
3. {"tool":"analyze","arguments":{"what":"annotations","session":"demo-ux","wait":true}}

Expected:
- Annotation list with coordinates and notes
`,
	"recording": `# Demo: Flow Recording

Goal: show record → action → stop workflow.

Steps:
1. {"tool":"configure","arguments":{"action":"recording_start","name":"demo-flow"}}
2. {"tool":"interact","arguments":{"action":"navigate","url":"http://localhost:xxxx"}}
3. {"tool":"configure","arguments":{"action":"recording_stop","recording_id":"..."}}

Expected:
- Saved recording ID and playback instructions
`,
	"dependencies": `# Demo: Dependency Vetting

Goal: identify unexpected third-party origins.

Steps:
1. {"tool":"analyze","arguments":{"what":"third_party_audit","first_party_origins":["http://localhost:xxxx"]}}
2. {"tool":"observe","arguments":{"what":"network_waterfall","limit":50}}

Expected:
- Highlight unexpected origins for review
`,
}

// canonicalPlaybookCapability normalizes capability aliases to canonical playbook keys.
func canonicalPlaybookCapability(capability string) string {
	switch strings.ToLower(strings.TrimSpace(capability)) {
	case "performance", "performance_analysis":
		return "performance"
	case "accessibility", "accessibility_audit":
		return "accessibility"
	case "security", "security_audit":
		return "security"
	default:
		return ""
	}
}

// resolvePlaybookKey resolves "{capability}/{level}" and bare "{capability}" to canonical keys.
func resolvePlaybookKey(raw string) string {
	trimmed := strings.Trim(strings.ToLower(strings.TrimSpace(raw)), "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	switch len(parts) {
	case 1:
		capability := canonicalPlaybookCapability(parts[0])
		if capability == "" {
			return ""
		}
		return capability + "/quick"
	case 2:
		capability := canonicalPlaybookCapability(parts[0])
		level := strings.TrimSpace(parts[1])
		if capability == "" || level == "" {
			return ""
		}
		return capability + "/" + level
	default:
		return ""
	}
}

// resolveResourceContent resolves a gasoline resource URI into canonical URI + markdown.
func resolveResourceContent(uri string) (string, string, bool) {
	switch {
	case uri == "gasoline://capabilities":
		return uri, capabilityIndex, true
	case uri == "gasoline://guide":
		return uri, guideContent, true
	case uri == "gasoline://quickstart":
		return uri, quickstartContent, true
	case strings.HasPrefix(uri, "gasoline://playbook/"):
		key := resolvePlaybookKey(strings.TrimPrefix(uri, "gasoline://playbook/"))
		text, ok := playbooks[key]
		if !ok {
			return "", "", false
		}
		return "gasoline://playbook/" + key, text, true
	case strings.HasPrefix(uri, "gasoline://demo/"):
		name := strings.TrimPrefix(uri, "gasoline://demo/")
		text, ok := demoScripts[name]
		if !ok {
			return "", "", false
		}
		return uri, text, true
	default:
		return "", "", false
	}
}

// Purpose: Embeds the capability index, usage guide, quickstart, and on-demand playbook content served as MCP resources.
// Why: Provides token-efficient documentation that MCP clients can read without external network access.

package main

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
| automation | Navigate sites, fill forms, click buttons, post content, any browser task | gasoline://playbook/automation/quick |
| api_validation | Contract mismatches, malformed responses, endpoint drift | See gasoline://guide |

## Available Playbook Variants

- gasoline://playbook/performance/quick
- gasoline://playbook/performance/full
- gasoline://playbook/accessibility/quick
- gasoline://playbook/accessibility/full
- gasoline://playbook/security/quick
- gasoline://playbook/security/full
- gasoline://playbook/automation/quick
- gasoline://playbook/automation/full

## Runtime Discovery

When unsure which params a mode accepts, use per-mode filtering:
- configure(what:"describe_capabilities", tool:"observe", mode:"errors") → returns only params relevant to observe/errors
- configure(what:"describe_capabilities", tool:"interact") → returns all interact modes with their per-mode params

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

1. {"tool":"configure","arguments":{"what":"health"}}
2. {"tool":"interact","arguments":{"what":"navigate","url":"<target-url>"}}
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
   {"tool":"configure","arguments":{"what":"health"}}
   {"tool":"observe","arguments":{"what":"page"}}
2. Capture navigation perf diff:
   {"tool":"interact","arguments":{"what":"navigate","url":"<target-url>","analyze":true}}
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

1. {"tool":"configure","arguments":{"what":"health"}}
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

1. {"tool":"configure","arguments":{"what":"health"}}
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

1. {"tool":"configure","arguments":{"what":"health"}}
2. {"tool":"analyze","arguments":{"what":"security_audit"}}
3. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from analyze>"}}

## Output Format

- High-risk findings first
- Evidence location (header/URL/request/etc.)
- Immediate mitigations
`,
	"automation/quick": `# Playbook: Browser Automation (Quick)

Use when you need to interact with any web page: navigate, fill forms, click buttons, post content, or complete multi-step workflows.

## Preconditions

- Extension connected and tracked tab confirmed.

## Steps

1. {"tool":"configure","arguments":{"what":"health"}}
2. {"tool":"interact","arguments":{"what":"navigate","url":"<target-url>"}}
3. {"tool":"observe","arguments":{"what":"screenshot"}}
4. {"tool":"interact","arguments":{"what":"click","selector":"<button-or-link>"}}
5. {"tool":"interact","arguments":{"what":"type","selector":"<input-or-textarea>","text":"<content>"}}
6. {"tool":"observe","arguments":{"what":"screenshot"}}

## Tips

- Always take a screenshot after navigation to understand the page layout.
- Take a screenshot before irreversible actions (submit, post, delete) to verify state.
- Use text=<visible text> selectors when CSS selectors are unknown.
- Use interact(what:"list_interactive") to discover clickable elements on the page.
- For rich text editors, type will handle content insertion automatically.
`,
	"automation/full": `# Playbook: Browser Automation (Full)

Use for complex multi-step browser workflows: form filling, multi-page navigation, content posting, or any task requiring sequential browser interactions.

## Preconditions

- Extension connected
- Correct tracked tab

## Steps

1. Verify connection:
   {"tool":"configure","arguments":{"what":"health"}}
2. Navigate to target:
   {"tool":"interact","arguments":{"what":"navigate","url":"<target-url>"}}
3. Screenshot to understand layout:
   {"tool":"observe","arguments":{"what":"screenshot"}}
4. Discover interactive elements (if selectors unknown):
   {"tool":"interact","arguments":{"what":"list_interactive","scope_selector":"<container>"}}
5. Perform actions (click, type, select, etc.):
   {"tool":"interact","arguments":{"what":"click","selector":"<element>"}}
   {"tool":"interact","arguments":{"what":"type","selector":"<input>","text":"<content>"}}
6. Verify result with screenshot:
   {"tool":"observe","arguments":{"what":"screenshot"}}
7. Continue or submit:
   {"tool":"interact","arguments":{"what":"click","selector":"<submit-button>"}}

## Example Workflows

### Fill and submit a form
  navigate → screenshot → type fields → click submit → screenshot

### Post content on a website
  navigate → click "new post" → type content → screenshot to verify → click post

### Multi-page checkout
  navigate → fill form → click next → fill form → screenshot → click submit

## Failure Modes

- element_not_found: use list_interactive to discover elements, retry with element_id
- ambiguous_target: narrow with scope_selector or scope_rect
- stale_element_id: refresh list_interactive, reacquire element_id
- blocked_by_overlay: run interact({what:"dismiss_top_overlay"}) then retry
- page_changed_unexpectedly: take screenshot, reassess

## Tips

- Screenshot before and after critical actions
- Use observe(what:"page") to confirm current URL
- interact navigate and refresh auto-include performance metrics
- For file uploads use interact(what:"upload")
`,
	"security/full": `# Playbook: Security Audit (Full)

Use for comprehensive browser-surface security review.

## Preconditions

- Extension connected
- Representative app flow loaded

## Steps

1. {"tool":"configure","arguments":{"what":"health"}}
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

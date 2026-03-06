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
var playbooks = mergePlaybookSets(
	playbookSetPerformance,
	playbookSetAccessibility,
	playbookSetSecurity,
	playbookSetAutomation,
)

func mergePlaybookSets(sets ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, set := range sets {
		for key, value := range set {
			if _, exists := merged[key]; exists {
				panic("duplicate playbook key: " + key)
			}
			merged[key] = value
		}
	}
	return merged
}

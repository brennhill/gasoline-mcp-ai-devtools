// Purpose: Accessibility playbook content.
// Why: Keeps capability-specific playbooks modular and easier to update.

package main

var playbookSetAccessibility = map[string]string{
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
}

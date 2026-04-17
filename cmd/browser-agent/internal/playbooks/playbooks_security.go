// playbooks_security.go — Security playbook content.
// Why: Keeps capability-specific playbooks modular and easier to update.

package playbooks

var playbookSetSecurity = map[string]string{
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

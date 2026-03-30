// playbooks_performance.go — Performance playbook content.
// Why: Keeps capability-specific playbooks modular and easier to update.

package playbooks

var playbookSetPerformance = map[string]string{
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
}

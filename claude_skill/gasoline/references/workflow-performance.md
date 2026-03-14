# Performance Workflow

Use this workflow for performance triage, regression checks, and optimization validation.

## Inputs

- Feature or page URL
- Baseline branch or expected performance budget
- Critical user journey steps
- Success budget (LCP, TTI, API latency, etc.)

## Step 1: Define Scenario

List the exact navigation and interaction sequence to measure. Keep it deterministic.

```bash
bash gasoline-browser/scripts/ensure-daemon.sh

# Clear existing data for clean measurement
bash gasoline-browser/scripts/gasoline-call.sh configure '{"what":"clear","buffer":"all"}'
```

## Step 2: Capture Baseline

```bash
# Navigate to target page
bash gasoline-browser/scripts/gasoline-call.sh interact '{"what":"navigate","url":"<page_url>","wait_for_stable":true,"analyze":true}'

# Collect web vitals
bash gasoline-browser/scripts/gasoline-call.sh observe '{"what":"vitals"}'

# Capture network waterfall
bash gasoline-browser/scripts/gasoline-call.sh observe '{"what":"network_waterfall","summary":true}'

# Timeline of all events
bash gasoline-browser/scripts/gasoline-call.sh observe '{"what":"timeline"}'

# Performance analysis
bash gasoline-browser/scripts/gasoline-call.sh analyze '{"what":"performance"}'

# Save baseline snapshot
bash gasoline-browser/scripts/gasoline-call.sh configure '{"what":"diff_sessions","verif_session_action":"capture","name":"perf_baseline"}'
```

## Step 3: Execute User Journey

Run the critical interaction sequence with performance profiling enabled:

```bash
# Each interaction with analyze:true for perf profiling
bash gasoline-browser/scripts/gasoline-call.sh interact '{"what":"click","selector":"<cta>","analyze":true,"wait_for_stable":true}'

# Capture vitals after each significant interaction
bash gasoline-browser/scripts/gasoline-call.sh observe '{"what":"vitals"}'
```

## Step 4: Capture Candidate Measurement

Repeat the same sequence on the candidate branch/build:

```bash
# Clear and re-run
bash gasoline-browser/scripts/gasoline-call.sh configure '{"what":"clear","buffer":"all"}'

# ... repeat exact same sequence ...

# Capture candidate snapshot
bash gasoline-browser/scripts/gasoline-call.sh configure '{"what":"diff_sessions","verif_session_action":"capture","name":"perf_candidate"}'
```

## Step 5: Compare Deltas

```bash
# Session comparison
bash gasoline-browser/scripts/gasoline-call.sh configure '{"what":"diff_sessions","verif_session_action":"compare","compare_a":"perf_baseline","compare_b":"perf_candidate"}'

# Network body comparison for API response times
bash gasoline-browser/scripts/gasoline-call.sh observe '{"what":"network_bodies","summary":true}'

# Third-party impact
bash gasoline-browser/scripts/gasoline-call.sh analyze '{"what":"third_party_audit","summary":true}'
```

## Step 6: Identify Bottlenecks

| Signal | Tool Call |
|--------|-----------|
| Slow API responses | `observe(network_bodies, status_min=200)` — check response times |
| Large payloads | `observe(network_waterfall, summary=true)` — check transfer sizes |
| Render blocking | `analyze(performance)` — check blocking resources |
| Third-party scripts | `analyze(third_party_audit)` — identify heavy external scripts |
| DOM complexity | `analyze(dom)` — check node count and depth |
| Layout thrashing | `observe(vitals)` — check CLS metric |

## Step 7: Report

Return:
- `scenario` — exact steps measured
- `baseline` — metrics before change
- `candidate` — metrics after change
- `regressions` — largest deltas by metric and by request
- `top_bottleneck` — single biggest performance issue
- `recommended_fix` — lowest-risk optimization
- `budget_status` — pass/fail against success budgets

## Troubleshooting

- **Inconsistent measurements:** Clear buffers between runs. Use `wait_for_stable` to ensure page is settled.
- **Missing vitals:** Some metrics (LCP, FID) require user interaction to trigger. Ensure the journey includes real clicks.
- **Third-party noise:** Use `noise_rule` to filter out known third-party requests from comparison.
- **Need visual comparison:** Use `visual_baseline` and `visual_diff` to compare rendered output.

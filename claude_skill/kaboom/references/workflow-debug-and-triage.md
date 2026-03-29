# Debug and Triage Workflow

Use this workflow when something is broken and you need root cause with proof.
Merges: debug, debug-triage, regression-test.

## Inputs

- Target URL
- Expected vs actual behavior
- Repro steps or user action sequence
- Test layer preference (for regression test: unit, integration, e2e)

## Step 1: Establish Baseline

Capture current state before reproducing:

```bash
# Check connection health
bash scripts/ensure-daemon.sh

# Capture baseline state
bash scripts/kaboom-call.sh observe '{"what":"tabs"}'
bash scripts/kaboom-call.sh observe '{"what":"page"}'
bash scripts/kaboom-call.sh observe '{"what":"errors","limit":20}'
bash scripts/kaboom-call.sh observe '{"what":"network_waterfall","limit":20,"summary":true}'
```

## Step 2: Reproduce Deliberately

Use small interact actions, one step at a time. Keep a correlation ID per high-level step.

```bash
# Navigate to the page
bash scripts/kaboom-call.sh interact '{"what":"navigate","url":"<target_url>","wait_for_stable":true}'

# Execute repro steps one at a time
bash scripts/kaboom-call.sh interact '{"what":"click","selector":"<selector>","correlation_id":"step1"}'
bash scripts/kaboom-call.sh interact '{"what":"type","selector":"<selector>","text":"<input>","correlation_id":"step2"}'
```

## Step 3: Capture Failure Evidence

Collect evidence at the failure point. Start broad, then narrow.

```bash
# Pre-assembled error context (best starting point)
bash scripts/kaboom-call.sh observe '{"what":"error_bundles","window_seconds":5}'

# Detailed console errors
bash scripts/kaboom-call.sh observe '{"what":"errors","limit":50}'

# Network failures
bash scripts/kaboom-call.sh observe '{"what":"network_bodies","status_min":400,"limit":20}'

# Console logs around failure
bash scripts/kaboom-call.sh observe '{"what":"logs","min_level":"warn","limit":30}'

# Async command results if relevant
bash scripts/kaboom-call.sh observe '{"what":"command_result","correlation_id":"<id>"}'
```

## Step 4: Deep Analysis (if needed)

Only use analyze when evidence from observe suggests a specific mismatch.

```bash
# DOM state inspection
bash scripts/kaboom-call.sh analyze '{"what":"dom","selector":"<problem_area>"}'

# Computed styles (layout issues)
bash scripts/kaboom-call.sh analyze '{"what":"computed_styles","selector":"<element>"}'

# API contract validation
bash scripts/kaboom-call.sh analyze '{"what":"api_validation"}'

# Visual screenshot for comparison
bash scripts/kaboom-call.sh observe '{"what":"screenshot","save_to":"evidence.png"}'
```

## Step 5: Classify Root Cause

Pick one primary source:

| Class | Signals |
|-------|---------|
| Frontend runtime | JS errors, undefined references, React/Vue errors |
| Backend/API | 4xx/5xx responses, schema mismatches, missing fields |
| Auth/session | 401/403, missing cookies/tokens, expired sessions |
| CSP/extension bridge | CSP violations, extension disconnected, command timeouts |
| Timing/race | Intermittent failures, stale data, out-of-order responses |
| Third-party | External script errors, CDN failures |

## Step 6: Produce Fix-Oriented Output

Return:
- `root_cause` — single sentence
- `evidence` — exact log lines, network traces, DOM state
- `minimal_fix` — smallest change that addresses root cause
- `verification_step` — one action to confirm the fix works

## Step 7: Create Regression Test (optional)

After the bug is confirmed and fixed:

```bash
# Generate test from captured session
bash scripts/kaboom-call.sh generate '{"what":"test","test_name":"regression_<bug_id>"}'

# Or generate from specific error context
bash scripts/kaboom-call.sh generate '{"what":"test_from_context","context":"error","error_id":"<id>"}'

# Generate reproduction script
bash scripts/kaboom-call.sh generate '{"what":"reproduction","output_format":"playwright","save_to":"repro.spec.ts"}'
```

Regression test checklist:
- Test fails on pre-fix behavior
- Test passes after fix
- Guard assertions prevent overfitting
- Run multiple times to verify no flakiness

## Troubleshooting

- **No errors captured:** Check `observe(tabs)` for tracked tab. Ensure extension is active on the page.
- **Errors too old:** Use `since_cursor` to get only recent entries, or `clear` buffers and reproduce fresh.
- **Async command timeout:** Set `background:true` on analyze calls, then poll with `observe(command_result)`.
- **Intermittent failure:** Use `event_recording_start`/`stop` to capture a full session, then replay with `playback`.

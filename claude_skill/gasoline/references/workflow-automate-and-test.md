# Automate and Test Workflow

Use this workflow for browser automation, demo creation, test generation, coverage improvement, and release readiness.
Merges: automate, demo, reliability, test-coverage, release-readiness.

## Inputs

- Target workflow or feature
- Start URL and auth preconditions
- Success condition
- Test/coverage targets (for testing flows)
- Release branch/commit and required gates (for release readiness)

---

## Automation

### Step 1: Validate Preconditions

```bash
bash scripts/ensure-daemon.sh
bash scripts/gasoline-call.sh observe '{"what":"tabs"}'
bash scripts/gasoline-call.sh observe '{"what":"page"}'

# Check auth state
bash scripts/gasoline-call.sh observe '{"what":"storage","storage_type":"cookies"}'
```

### Step 2: Plan Robust Selectors

Prefer semantic selectors and stable attributes over brittle CSS chains.

```bash
# Discover interactive elements
bash scripts/gasoline-call.sh interact '{"what":"list_interactive","visible_only":true}'

# Use element_id from list_interactive for stable targeting
bash scripts/gasoline-call.sh interact '{"what":"click","element_id":"<stable_id>"}'
```

Selector priority: `element_id` > semantic (`text=Submit`, `role=button`) > CSS selector > coordinates.

### Step 3: Execute in Small Steps

After each interact call, verify the result.

```bash
# Navigate
bash scripts/gasoline-call.sh interact '{"what":"navigate","url":"<url>","wait_for_stable":true}'

# Interact with action_diff for mutation tracking
bash scripts/gasoline-call.sh interact '{"what":"click","selector":"button.submit","action_diff":true}'

# Fill and submit forms
bash scripts/gasoline-call.sh interact '{"what":"fill_form_and_submit","fields":[{"selector":"#email","value":"user@example.com"},{"selector":"#password","value":"pass"}],"submit_selector":"button[type=submit]"}'

# Wait for expected state
bash scripts/gasoline-call.sh interact '{"what":"wait_for","selector":".success-message","timeout_ms":5000}'
```

### Step 4: Bounded Recovery

If a step fails, retry once with alternate selector or wait strategy.

```bash
# Try alternate selector
bash scripts/gasoline-call.sh interact '{"what":"click","selector":"text=Submit","timeout_ms":10000}'

# Or wait for stability then retry
bash scripts/gasoline-call.sh interact '{"what":"wait_for_stable","stability_ms":1000}'
bash scripts/gasoline-call.sh interact '{"what":"click","selector":"button.submit"}'
```

### Step 5: Batch Execution

For multi-step sequences, use batch mode:

```bash
bash scripts/gasoline-call.sh interact '{"what":"batch","steps":[{"what":"navigate","url":"https://example.com"},{"what":"click","selector":"#login"},{"what":"type","selector":"#email","text":"user@example.com"},{"what":"click","selector":"button[type=submit]"}],"continue_on_error":false}'
```

### Step 6: Save Reusable Sequences

```bash
# Save a sequence for reuse
bash scripts/gasoline-call.sh configure '{"what":"save_sequence","name":"login_flow","steps":[{"what":"navigate","url":"https://example.com/login"},{"what":"type","selector":"#email","text":"user@example.com"},{"what":"click","selector":"button[type=submit]"}]}'

# Replay it later
bash scripts/gasoline-call.sh configure '{"what":"replay_sequence","name":"login_flow"}'
```

---

## Demo Creation

### Build Demo Script

```bash
# Start recording
bash scripts/gasoline-call.sh interact '{"what":"screen_recording_start","name":"demo_feature_x"}'

# Execute steps with subtitles for narration
bash scripts/gasoline-call.sh interact '{"what":"navigate","url":"<demo_url>","subtitle":"Opening the dashboard"}'
bash scripts/gasoline-call.sh interact '{"what":"click","selector":"<feature_cta>","subtitle":"Launching Feature X"}'

# Capture proof screenshots
bash scripts/gasoline-call.sh observe '{"what":"screenshot","save_to":"demo/step1.png"}'

# Stop recording
bash scripts/gasoline-call.sh interact '{"what":"screen_recording_stop","name":"demo_feature_x"}'
```

---

## Test Generation

```bash
# Start recording user actions
bash scripts/gasoline-call.sh configure '{"what":"event_recording_start","name":"test_session"}'

# ... perform the user journey ...

# Stop recording
bash scripts/gasoline-call.sh configure '{"what":"event_recording_stop"}'

# Generate test from captured session
bash scripts/gasoline-call.sh generate '{"what":"test","test_name":"user_journey","save_to":"tests/user-journey.spec.ts"}'

# Generate test from specific context
bash scripts/gasoline-call.sh generate '{"what":"test_from_context","context":"interaction","save_to":"tests/interaction.spec.ts"}'

# Heal broken selectors in existing tests
bash scripts/gasoline-call.sh generate '{"what":"test_heal","action":"batch","test_dir":"tests/"}'
```

---

## Reliability Validation

### Canary Flows

Cover all tool paths:

```bash
# Observe path
bash scripts/gasoline-call.sh observe '{"what":"errors","limit":5}'

# Interact path
bash scripts/gasoline-call.sh interact '{"what":"explore_page"}'

# Analyze path
bash scripts/gasoline-call.sh analyze '{"what":"page_summary"}'

# Generate path
bash scripts/gasoline-call.sh generate '{"what":"pr_summary"}'
```

### Stress Transitions

Test reconnect, tab switches, extension restarts:

```bash
# Switch tabs
bash scripts/gasoline-call.sh interact '{"what":"switch_tab","tab_index":1}'
bash scripts/gasoline-call.sh interact '{"what":"switch_tab","tab_index":0}'

# Verify health after stress
bash scripts/gasoline-call.sh configure '{"what":"health"}'
bash scripts/gasoline-call.sh configure '{"what":"doctor"}'
```

---

## Release Readiness

### Quality Gates

```bash
# System health
bash scripts/gasoline-call.sh configure '{"what":"health"}'
bash scripts/gasoline-call.sh configure '{"what":"doctor"}'

# Audit tool usage
bash scripts/gasoline-call.sh configure '{"what":"audit_log","operation":"report"}'
```

### Integration Smoke

Run critical end-to-end flows and capture results:

```bash
# Session diff: capture before and after
bash scripts/gasoline-call.sh configure '{"what":"diff_sessions","verif_session_action":"capture","name":"pre_release"}'

# ... run critical flows ...

bash scripts/gasoline-call.sh configure '{"what":"diff_sessions","verif_session_action":"capture","name":"post_release"}'
bash scripts/gasoline-call.sh configure '{"what":"diff_sessions","verif_session_action":"compare","compare_a":"pre_release","compare_b":"post_release"}'
```

### Decision

Return: go/no-go with blockers, mitigations, and confidence level.

## Troubleshooting

- **Flaky selectors:** Use `list_interactive` to discover stable `element_id` values.
- **Automation timeout:** Increase `timeout_ms` or add explicit `wait_for` steps.
- **Recording fails:** Check disk space and that no other recording is active.
- **Test generation empty:** Ensure actions were captured during the session with `observe(actions)`.

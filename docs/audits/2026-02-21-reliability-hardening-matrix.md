# Reliability Hardening Matrix (Post-v0.7.2)

Date: 2026-02-21
Owner: CODEX (reliability pass)
Scope: MCP bridge, daemon lifecycle, protocol handling, extension connectivity, browser action execution, diagnostics, smoke coverage

## Goal

Prevent silent failures and transport instability by hardening all known failure pathways in one cohesive reliability model.

## Reliability Invariants

1. Every MCP request gets exactly one valid JSON-RPC terminal response.
2. No bridge/daemon crash is silent; every unexpected exit writes structured diagnostics.
3. Startup/readiness states are explicit; no ambiguous "server up but not usable" states.
4. Extension/pilot readiness cannot flap into false-negative failure without evidence.
5. Every fallback path is visible to the LLM in machine-readable output.
6. Every failure pathway has at least one deterministic test.

## Failure Pathways and Controls

### A. Binary/Process/Daemon Lifecycle

1. `LIFECYCLE-01` Wrong binary launched (old install path).
- Trigger: config points to stale binary.
- Symptom: behavior mismatches expected version.
- Prevention: hard startup fingerprint (version/build-id/path/sha).
- Detection: lifecycle log `bridge_mode_start` fingerprint.
- Recovery: controlled daemon takeover by state-dir lock + version check.
- Test: fingerprint unit test + startup log assertion.

2. `LIFECYCLE-02` Multiple daemons for same state-dir.
- Trigger: parallel starts without ownership checks.
- Symptom: nondeterministic command routing, stale state.
- Prevention: single-daemon lock per state-dir by default.
- Detection: lifecycle event with existing PID + takeover decision.
- Recovery: graceful stop old daemon, spawn one authoritative daemon.
- Test: integration test for takeover semantics.

3. `LIFECYCLE-03` Stale PID file blocks startup.
- Trigger: crash/kill leaves stale PID metadata.
- Symptom: startup reports in-use/unknown conflicts.
- Prevention: stale PID owner verification before refusal.
- Detection: `stale_pid_*` lifecycle events.
- Recovery: remove stale metadata and continue startup.
- Test: stale PID cleanup unit/integration tests.

4. `LIFECYCLE-04` Port occupied by non-Gasoline process.
- Trigger: shared port collision.
- Symptom: daemon fails to start; unclear error.
- Prevention: explicit service-name detection.
- Detection: health metadata + process lookup.
- Recovery: return specific non-gasoline conflict error.
- Test: non-gasoline port conflict test.

5. `LIFECYCLE-05` Daemon process alive but unhealthy/deaf.
- Trigger: partial deadlock/frozen server.
- Symptom: bridge retries then timeouts.
- Prevention: readiness checks include health endpoint + command execution health.
- Detection: command readiness counters, failed/timeout trend.
- Recovery: bridge-side restart fast-path + SIGCONT/SIGTERM/SIGKILL sequence.
- Test: forced frozen-daemon recovery test.

6. `LIFECYCLE-06` Unexpected daemon exit lacks diagnostics.
- Trigger: panic/termination with no crash log write.
- Symptom: "transport closed" with no root cause.
- Prevention: always-on exit diagnostic write path with fallback files.
- Detection: `daemon_shutdown` diagnostic entry.
- Recovery: surfaced crash path in stderr + logs.
- Test: exit diagnostic write/fallback tests.

7. `LIFECYCLE-07` Unsafe kill-by-name behavior.
- Trigger: cleanup kills unrelated processes.
- Symptom: collateral process termination.
- Prevention: only kill PIDs known via lock/PID files/port ownership.
- Detection: audit entry for each targeted PID.
- Recovery: bounded/verified kill flow only.
- Test: process ownership contract tests.

### B. Bridge and MCP Transport

8. `BRIDGE-01` Stdio framing parse failure (header order variations).
- Trigger: `Content-Type` before `Content-Length`, mixed framing.
- Symptom: parse errors then transport close.
- Prevention: robust header block parser, order-insensitive.
- Detection: parse error telemetry with framing mode.
- Recovery: emit JSON-RPC parse error, do not panic.
- Test: framed input tests for header-order permutations.

9. `BRIDGE-02` Concurrent stdout interleaving corrupts JSON-RPC.
- Trigger: concurrent request goroutines writing without lock.
- Symptom: malformed responses and client disconnect.
- Prevention: single write gate/mutex for all stdout writes.
- Detection: protocol parser failure counters.
- Recovery: none at runtime; prevention-only.
- Test: contract test asserts all writes use write helper.

10. `BRIDGE-03` Notification gets an illegal response.
- Trigger: treating notification as request.
- Symptom: MCP client closes transport.
- Prevention: no-response rule for no-ID notifications.
- Detection: notification handling tests.
- Recovery: ignore unknown notifications safely.
- Test: notification-no-response tests.

11. `BRIDGE-04` Invalid ID semantics break MCP compliance.
- Trigger: null/invalid id handling mismatch.
- Symptom: client rejects protocol stream.
- Prevention: strict `HasInvalidID` checks + spec-compliant errors.
- Detection: MCP protocol tests.
- Recovery: return `-32600` with null id.
- Test: null-id/invalid-id tests.

12. `BRIDGE-05` Bridge exits before slow in-flight response flush.
- Trigger: EOF/read error while requests still active.
- Symptom: dropped responses, transport closed.
- Prevention: waitgroup drain before shutdown.
- Detection: bridge exit diagnostics with in-flight count.
- Recovery: bounded shutdown wait + forced flush.
- Test: in-flight request completion test.

13. `BRIDGE-06` Panic from channel double-close in daemon state.
- Trigger: repeated failure signaling races.
- Symptom: abrupt bridge termination.
- Prevention: idempotent channel-close guards (`sync.Once` or closed flags).
- Detection: panic recovery diagnostics.
- Recovery: bridge returns structured startup error, not silent exit.
- Test: race-oriented daemon state tests.

14. `BRIDGE-07` First tool call races daemon startup.
- Trigger: tools/call arrives before daemon readiness.
- Symptom: immediate "starting" errors; clients may not retry.
- Prevention: bounded readiness grace window before fail-open retry hint.
- Detection: startup-race counters.
- Recovery: single automatic retry path for first call.
- Test: startup race tests with strict latency bounds.

15. `BRIDGE-08` Bridge cannot prove loaded binary identity.
- Trigger: mixed binaries across clients/paths.
- Symptom: inconsistent behavior impossible to debug quickly.
- Prevention: startup fingerprint log each bridge launch.
- Detection: lifecycle fingerprint event.
- Recovery: operator can instantly confirm executable provenance.
- Test: fingerprint extraction tests.

### C. HTTP Server and Handler Contract

16. `HTTP-01` Handler `WriteTimeout` shorter than tool operation.
- Trigger: long analyze/interact calls exceed server write timeout.
- Symptom: truncated responses / generic bridge connection errors.
- Prevention: align server timeouts with tool SLA ceilings.
- Detection: timeout-tagged handler metrics.
- Recovery: return explicit timeout status to LLM.
- Test: long-call timeout integration test.

17. `HTTP-02` Large response read truncation.
- Trigger: bounded readers too small for valid payloads.
- Symptom: malformed/incomplete responses.
- Prevention: safe but adequate size limits + schema-aware bounds.
- Detection: size-limit breach counters.
- Recovery: structured error with guidance.
- Test: large-payload bridge forward test.

18. `HTTP-03` Non-200 handling loses body context.
- Trigger: upstream returns error payload.
- Symptom: opaque `-32603` failures.
- Prevention: include status and bounded body in bridge error.
- Detection: status/error telemetry.
- Recovery: actionable error text to LLM.
- Test: non-200 forwarding tests.

### D. Command Lifecycle and Async Semantics

19. `CMD-01` Pending commands never reach terminal state.
- Trigger: extension event loss, stale correlation, dispatcher mismatch.
- Symptom: infinite polling.
- Prevention: terminal timeout policy with explicit status transition.
- Detection: pending-age monitors.
- Recovery: timeout/expired terminal status with remediation.
- Test: timeout status tests.

20. `CMD-02` Command failure cause lost.
- Trigger: generic error wrappers.
- Symptom: LLM cannot decide retry strategy.
- Prevention: preserve root cause, subsystem, retryability fields.
- Detection: error bundle completeness checks.
- Recovery: explicit retry guidance.
- Test: command_result schema tests.

21. `CMD-03` Expired command treated as missing.
- Trigger: eviction ambiguity.
- Symptom: false "not found."
- Prevention: explicit expired lifecycle state.
- Detection: failed/expired counters.
- Recovery: return expired with recovery hint.
- Test: eviction and restart-on-eviction tests.

22. `CMD-04` Queue overload causes silent drops.
- Trigger: burst of async actions.
- Symptom: commands disappear.
- Prevention: bounded queue + backpressure response.
- Detection: dropped-count metrics.
- Recovery: explicit overloaded error.
- Test: burst saturation test.

### E. Extension/Pilot/Browser Session State

23. `EXT-01` False extension disconnect/reconnect flapping.
- Trigger: overly aggressive stale thresholds.
- Symptom: repeated reconnect churn and transient failures.
- Prevention: hysteresis/debounce on disconnect transitions.
- Detection: reconnect-rate telemetry.
- Recovery: keep "degraded" state before hard-disconnected state.
- Test: synthetic flap timeline tests.

24. `EXT-02` Pilot reported disabled during startup uncertainty.
- Trigger: stale/late pilot probe.
- Symptom: premature `pilot_disabled` hard errors.
- Prevention: optimistic default enabled with explicit disable override only.
- Detection: pilot source tagging (`user_disabled`, `probe_timeout`, etc.).
- Recovery: defer hard disable errors until authoritative proof.
- Test: startup probe race tests.

25. `EXT-03` Tracked tab stale or wrong tab targeted.
- Trigger: tab switch/new tab/back navigation races.
- Symptom: actions happen on wrong page.
- Prevention: deterministic tab resolution with final URL verification.
- Detection: result includes `resolved_tab_id`, `resolved_url`.
- Recovery: retry with explicit tab_id.
- Test: tab switch/back/new_tab regression tests.

26. `EXT-04` Extension connected but command dispatch unavailable.
- Trigger: partial extension readiness.
- Symptom: command accepted, no result.
- Prevention: command-execution readiness gate.
- Detection: health `command_execution` stats.
- Recovery: return immediate recoverable error.
- Test: readiness gate tests.

### F. Browser Action and Tool Contract

27. `ACTION-01` Action alias mismatch (`what` vs `action`) to extension.
- Trigger: CLI/MCP payload normalization gaps.
- Symptom: `unknown_action` regressions.
- Prevention: canonical normalization before dispatch.
- Detection: contract tests for all action aliases.
- Recovery: fallback alias mapping.
- Test: navigate/new_tab/list_interactive contract tests.

28. `ACTION-02` Successful status with null result payload.
- Trigger: handler path returns empty value on success.
- Symptom: client cannot use result.
- Prevention: non-null result schema enforcement.
- Detection: response validation guard in handler.
- Recovery: convert to explicit error if payload invalid.
- Test: non-null payload contract tests.

29. `ACTION-03` Missing telemetry fields (`timing_ms`, `dom_summary`, `perf_diff`).
- Trigger: incomplete enrichment on action completion.
- Symptom: profiling features appear broken.
- Prevention: centralized result enrichment middleware.
- Detection: field-presence assertions.
- Recovery: fallback minimal metrics.
- Test: action enrichment smoke tests.

30. `ACTION-04` Upload pipeline stalls without terminal result.
- Trigger: long file operations, stale pilot cache, command poll disconnect.
- Symptom: timeout with no poll output.
- Prevention: long-op heartbeat checkpoints + correlation persistence.
- Detection: upload stage telemetry.
- Recovery: resume via correlation_id after reconnect.
- Test: upload e2e + timeout-recovery tests.

31. `ACTION-05` Recording start/stop state machine drift.
- Trigger: stop called while already recording or vice versa.
- Symptom: "already recording"/stuck recording states.
- Prevention: explicit recording state machine with idempotent transitions.
- Detection: recording status telemetry.
- Recovery: `configure(restart)` + state reconciliation.
- Test: recording lifecycle tests.

### G. CSP/CORS and Execution World Fallback

32. `SEC-01` Main-world script blocked by CSP.
- Trigger: strict CSP/Trusted Types.
- Symptom: execute_js and actions fail unexpectedly.
- Prevention: automatic isolated-world fallback for compatible actions.
- Detection: CSP-blocked error classification.
- Recovery: return explicit dual-result:
  - `Error: MAIN world execution FAILED. Fallback in ISOLATED is SUCCESS|ERROR`.
- Test: CSP page repro with forced main->isolated fallback assertions.

33. `SEC-02` Fallback used but not communicated to LLM.
- Trigger: fallback path hidden in generic error.
- Symptom: LLM retries wrong strategy.
- Prevention: standardized fallback outcome envelope.
- Detection: response schema validation.
- Recovery: explicit fallback metadata in result.
- Test: fallback messaging tests.

34. `SEC-03` CORS-dependent pages hide downstream logic failures.
- Trigger: microservice CORS break prevents UI load.
- Symptom: cannot inspect real production logic/data issues.
- Prevention: explicit diagnostic mode design (opt-in, labeled altered environment).
- Detection: mode flag in every result/log.
- Recovery: compare secure run vs bypass run before conclusions.
- Test: CORS/CSP bypass mode contract tests.

### H. Observability and Diagnostics

35. `OBS-01` Transport close with no causal breadcrumb.
- Trigger: bridge exits without exit event detail.
- Symptom: "Transport closed" with no root cause.
- Prevention: bridge exit diagnostics include reason, last method, request counts, framing.
- Detection: mandatory `bridge_exit` event.
- Recovery: auto-surface diagnostic path in stderr/log.
- Test: bridge exit diagnostic tests.

36. `OBS-02` Critical logs lost due buffer cap/noise.
- Trigger: high-volume console/network noise.
- Symptom: root-cause entries evicted.
- Prevention: noise auto-detect + summarized logs + priority retention for errors.
- Detection: dropped counters and high-water marks.
- Recovery: on-demand buffer clear/report snapshots.
- Test: noise suppression smoke tests.

37. `OBS-03` Version/provenance not attached to failures.
- Trigger: error generation path misses fingerprint.
- Symptom: hard to correlate bug reports to binaries.
- Prevention: include version/build metadata in fatal diagnostics.
- Detection: schema check on diagnostics entries.
- Recovery: n/a.
- Test: diagnostic metadata tests.

### I. Smoke and Regression Coverage Gaps

38. `TEST-01` Smoke tests pass without proving behavior.
- Trigger: weak assertions or synthetic/noisy-free pages.
- Symptom: false confidence.
- Prevention: trust-level test design with deterministic fixtures.
- Detection: contract tests for smoke script invariants.
- Recovery: fail smoke when evidence is missing.
- Test: smoke harness contract tests.

39. `TEST-02` No deterministic CSP/CORS repro fixture.
- Trigger: relying on external pages only.
- Symptom: flaky security-path testing.
- Prevention: local CSP/CORS fixture pages with expected failures/successes.
- Detection: fixture health checks.
- Recovery: n/a.
- Test: CSP/CORS fixture smoke tests.

40. `TEST-03` No startup/reconnect race simulation.
- Trigger: only happy-path startup tests.
- Symptom: regressions in first-call reliability.
- Prevention: race-focused tests for daemon startup + extension reconnect.
- Detection: failure-rate threshold in CI.
- Recovery: n/a.
- Test: startup race soak tests.

41. `TEST-04` No CLI vs MCP parity contract.
- Trigger: one path fixed, other regresses.
- Symptom: "works in CLI, fails in MCP" class of bugs.
- Prevention: parity assertions for canonical actions/results.
- Detection: compare schemas + statuses across entrypoints.
- Recovery: n/a.
- Test: parity integration suite.

42. `TEST-05` No failure-injection matrix.
- Trigger: no controlled fault simulation.
- Symptom: untested recovery logic.
- Prevention: test harness fault toggles (daemon down, extension down, timeout, CSP block).
- Detection: expected recovery outcome assertions.
- Recovery: n/a.
- Test: failure-injection suite.

## Systemic Hardening Controls (Unified, Non-Patchwork)

1. Reliability state machine
- Explicit states: `booting`, `daemon_ready`, `extension_degraded`, `extension_ready`, `recovering`, `fatal`.
- No direct hard failures from transient unknown states.

2. Unified error envelope
- Every non-success includes `subsystem`, `reason`, `retryable`, `retry_after_ms`, `fallback_used`, `correlation_id`.

3. Mandatory exit/startup diagnostics
- Every bridge/daemon start and exit writes a structured lifecycle event.

4. Bounded automatic recovery
- One automatic retry path for startup and reconnect races.
- Escalate to explicit terminal error after bounded retries.

5. Contract-first response validation
- Validate non-null results for success responses.
- Enforce required fields for each action class.

6. Test matrix as release gate
- Each failure ID above has at least one automated test.
- Smoke suite demonstrates real behavior on deterministic fixtures.

## Implementation Order

1. Bridge/daemon lifecycle + diagnostics hardening.
2. Startup race and reconnect-state hardening.
3. Action/result contract enforcement.
4. CSP fallback messaging and execution-world guarantees.
5. Smoke/fixture upgrades and parity gates.

## Status Update (2026-02-21)

- Completed: bridge/daemon lifecycle hardening baseline (`daemonStartupGracePeriod`, startup readiness/failure signaling, respawn retry path).
- Completed: startup/reconnect state stabilization (reconnect now only triggers after actual disconnect-threshold crossing).
- Completed: mandatory lifecycle diagnostics (`bridge_mode_start` fingerprint + `bridge_exit` structured event persisted).
- Completed: bridge-side unified soft-error envelope for tool-call failures (`error_code`, `subsystem`, `reason`, `retryable`, `retry_after_ms`, `fallback_used`, `correlation_id`).
- Completed: no-silent-response guardrails for forwarded calls (`204` with id, empty body, invalid JSON now return explicit structured soft errors).
- In progress: action/result contract enforcement expansion across all interact/analyze/generate pathways.
- In progress: deterministic smoke fixtures for CSP/CORS/noise and parity release gates.

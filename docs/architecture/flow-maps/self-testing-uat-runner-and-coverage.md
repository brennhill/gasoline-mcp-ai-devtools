---
doc_type: flow_map
flow_id: self-testing-uat-runner-and-coverage
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
feature_ids:
  - feature-self-testing
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
entrypoints:
  - scripts/smoke-test.sh
  - scripts/smoke-tests/framework-smoke.sh
  - scripts/test-all-split.sh
  - scripts/test-original-uat.sh
  - scripts/test-new-uat.sh
  - scripts/tests/framework.sh
code_paths:
  - scripts/smoke-test.sh
  - scripts/smoke-tests/framework-smoke.sh
  - scripts/smoke-tests/14-browser-push.sh
  - scripts/smoke-tests/15-file-upload.sh
  - scripts/smoke-tests/29-framework-selector-resilience.sh
  - scripts/smoke-tests/30-stability-shutdown.sh
  - scripts/test-all-split.sh
  - scripts/test-original-uat.sh
  - scripts/test-new-uat.sh
  - scripts/uat-result-lib.sh
  - scripts/tests/framework.sh
test_paths:
  - scripts/smoke-tests/14-browser-push.sh
  - scripts/smoke-tests/15-file-upload.sh
  - scripts/smoke-tests/29-framework-selector-resilience.sh
  - scripts/smoke-tests/30-stability-shutdown.sh
  - scripts/tests/cat-01-protocol.sh
  - scripts/tests/cat-02-observe.sh
  - scripts/tests/cat-03-generate.sh
  - scripts/tests/cat-04-configure.sh
  - scripts/tests/cat-05-interact.sh
  - scripts/tests/cat-06-lifecycle.sh
  - scripts/tests/cat-07-concurrency.sh
  - scripts/tests/cat-08-security.sh
  - scripts/tests/cat-09-http.sh
  - scripts/tests/cat-10-regression.sh
  - scripts/tests/cat-11-data-pipeline.sh
  - scripts/tests/cat-12-rich-actions.sh
  - scripts/tests/cat-13-pilot-contract.sh
  - scripts/tests/cat-14-extension-startup.sh
  - scripts/tests/cat-15-pilot-success-path.sh
  - scripts/tests/cat-16-api-contract.sh
  - scripts/tests/cat-17-generation-logic.sh
  - scripts/tests/cat-17-healing-logic.sh
  - scripts/tests/cat-17-performance.sh
  - scripts/tests/cat-18-recording.sh
  - scripts/tests/cat-18-recording-logic.sh
  - scripts/tests/cat-18-playback-logic.sh
  - scripts/tests/cat-19-link-health.sh
  - scripts/tests/cat-19-extended.sh
  - scripts/tests/cat-20-noise-persistence.sh
  - scripts/tests/cat-20-security.sh
  - scripts/tests/cat-20-filtering-logic.sh
  - scripts/tests/cat-21-stress.sh
  - scripts/tests/cat-22-advanced.sh
  - scripts/tests/cat-29-reproduction.sh
  - scripts/tests/cat-30-recording-automation.sh
  - scripts/tests/cat-31-link-crawling.sh
  - scripts/tests/cat-32-auto-detect.sh
---

# Self-Testing UAT Runner and Coverage

## Scope

Covers the split UAT orchestration flow, category result integrity checks, and optional Go runtime coverage collection during UAT daemon execution.

## Entrypoints

- `scripts/smoke-test.sh` orchestrates smoke modules, supports resume/only, and keeps a working daemon available after completion.
- `scripts/smoke-tests/framework-smoke.sh` provides smoke polling/diagnostic helpers and shared cleanup behavior.
- `scripts/test-all-split.sh` orchestrates phase execution and combined summary.
- `scripts/test-original-uat.sh` runs stable categories and validates category result integrity.
- `scripts/test-new-uat.sh` runs newer categories and validates category result integrity.
- `scripts/tests/framework.sh` starts/stops per-category daemons and writes category result files.

## Primary Flow

1. `scripts/smoke-test.sh` initializes smoke framework state, supports `--start-from`/`--only`, and runs module scripts in order.
2. Smoke module checks validate live contracts including `_push_*` piggyback hints (module 14), upload success-page and MD5 verification (module 15), framework resilience retries (module 29), and shutdown/restart availability (module 30).
3. After smoke modules complete, daemon teardown/retention honors `SMOKE_KEEP_DAEMON_ON_EXIT` (default keep-alive for local workflows; strict cleanup mode for automation).
4. `test-all-split.sh` optionally builds a coverage-instrumented daemon (`go build -cover -coverpkg=./...`) when `UAT_GO_COVERAGE=1`.
5. The split runner exports `KABOOM_UAT_WRAPPER` and `KABOOM_UAT_GOCOVERDIR` so category daemons run with optional runtime coverage emission.
6. Phase runners launch category scripts in parallel and wait for completion.
7. Each category script uses `framework.sh`; `finish_category` writes a structured result file with `PASS_COUNT`, `FAIL_COUNT`, `SKIP_COUNT`, and metadata.
8. Phase runners parse result files via `scripts/uat-result-lib.sh`, aggregate totals, and fail on missing/corrupt/invalid result files when integrity enforcement is enabled.
9. Phase runners emit machine-readable phase summaries (`KABOOM_UAT_SUMMARY_FILE`) for the split runner.
10. The split runner merges phase summaries, reports real pass/fail/skip totals and category coverage, and optionally reports runtime Go coverage via `go tool covdata`.

## Error and Recovery Paths

- Transient transport failures (`EOF`, `transport_no_response`, `no_data`, temporary extension disconnect) in framework-resilience smoke checks trigger bounded retries before hard fail.
- Stability module extension disconnect checks are hard failures (not skips) to preserve regression signal.
- Upload smoke checks poll for `/upload/success` redirect completion to avoid false negatives while tabs are still loading.
- Upload server verification parser accepts current execute-js result envelopes (`return_value`) to avoid false MD5/CSRF mismatches.
- Push hint checks poll for `_push_*` piggyback presence and validate clearing after inbox drain.
- Smoke runner post-processing ensures daemon availability for developer workflows even when shutdown tests stop the daemon.
- Missing result file: category reported as missing and counted as integrity error.
- Corrupt result file or invalid counters: category reported as integrity error.
- Zero total assertions: treated as integrity failure to prevent false green runs.
- Coverage mode enabled with no coverage artifacts: split runner marks Go coverage step failed.
- `UAT_ALLOW_NEW_FAILURES=1` keeps phase-2 failures non-blocking while preserving explicit soft-failure reporting.

## State and Contracts

- Category result file contract (`framework.sh`):
  - `PASS_COUNT=<int>`
  - `FAIL_COUNT=<int>`
  - `SKIP_COUNT=<int>`
  - `ELAPSED=<int>`
  - `CATEGORY_ID=<id>`
  - `CATEGORY_NAME="<name>"`
- Phase summary contract:
  - `TOTAL_PASS`, `TOTAL_FAIL`, `TOTAL_SKIP`, `TOTAL_ASSERTIONS`
  - `CATEGORY_TOTAL`, `CATEGORY_REPORTED`, `INTEGRITY_ERRORS`
  - `RESULTS_DIR`, `DURATION`
- Coverage mode contract:
  - `UAT_GO_COVERAGE=1` enables instrumentation path.
  - `UAT_GO_COVERAGE_MIN=<percent>` optionally gates runtime coverage.

## Code Paths

- `scripts/test-all-split.sh`
- `scripts/smoke-test.sh`
- `scripts/smoke-tests/framework-smoke.sh`
- `scripts/smoke-tests/14-browser-push.sh`
- `scripts/smoke-tests/15-file-upload.sh`
- `scripts/smoke-tests/29-framework-selector-resilience.sh`
- `scripts/smoke-tests/30-stability-shutdown.sh`
- `scripts/test-original-uat.sh`
- `scripts/test-new-uat.sh`
- `scripts/uat-result-lib.sh`
- `scripts/tests/framework.sh`

## Test Paths

- UAT category suites under `scripts/tests/cat-*.sh`

## Edit Guardrails

- Keep result-file parsing centralized in `scripts/uat-result-lib.sh`; do not duplicate ad-hoc grep logic.
- Preserve `framework.sh` result-file keys when extending category metadata.
- Keep phase summary files machine-readable key/value pairs so split orchestration remains deterministic.
- When adding/removing categories, update both phase arrays and category coverage expectations.

## Feature Links

- Self-Testing feature pointer: `docs/features/feature/self-testing/flow-map.md`

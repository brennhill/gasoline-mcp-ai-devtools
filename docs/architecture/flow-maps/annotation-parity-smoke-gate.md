---
doc_type: architecture_flow_map
feature_id: feature-annotated-screenshots
status: active
owners: []
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Annotation Parity Smoke Gate

## Scope

Deterministic annotation parity validation without manual draw interaction.

This gate validates:
1. HTTP ingest (`POST /draw-mode/complete`) for annotation + detail payloads.
2. MCP retrieval (`analyze(annotations|annotation_detail)`) including multi-project scope metadata.
3. MCP artifact generation (`generate(visual_test|annotation_report|annotation_issues)`).

## Entrypoints

1. `bash scripts/smoke-test.sh --only 31`
2. `npm run smoke:annotation-parity`
3. `npm run smoke:annotation-parity-suite`
4. `npm run smoke:annotation-parity-benchmark`
5. `bash scripts/smoke-test.sh --start-from 31`

## Primary Flow

1. Smoke module `31-annotation-parity.sh` posts deterministic annotation payload A for `localhost:3000`.
2. Module posts deterministic annotation payload B for `localhost:5173` using the same `annot_session_name`.
3. Module calls `analyze({what:"annotations", annot_session})` and verifies:
   - both annotations are present
   - multi-page totals are correct
   - scope ambiguity metadata is present.
4. Module calls `analyze({what:"annotations", annot_session, url:"http://localhost:3000/*"})` and verifies scoped reduction.
5. Module calls `analyze({what:"annotation_detail", correlation_id})` and verifies selector/tag/framework/context fields.
6. Module calls `generate` annotation formats and verifies expected outputs.
7. Runner enforces daemon version parity (`VERSION` vs `/health.version`) before reusing a running daemon in `--only` / `--start-from` mode.

## Error and Recovery Paths

1. Ingest failure (`/draw-mode/complete`) surfaces HTTP status + response body in failure output.
2. Transient startup or eventual-consistency states (`starting up`, `no_data`, `no annotations`) are retried with bounded attempts in module 31.
3. Resume/version mismatch (`--only` / `--start-from`) triggers daemon restart before module execution.
4. JSON parse failures in gate assertions are treated as hard failures with parse diagnostics.
5. Scope filter mismatch failures include returned IDs and counts for triage.

## State and Contracts

1. Ingest contract fields used by gate:
   - `annot_session_name`
   - `annotations[]`
   - `element_details{}`
   - `page_url`
   - `tab_id`
2. Named-session contract fields validated:
   - `pages`
   - `total_count`
   - `page_count`
   - `projects`
   - `scope_ambiguous`
3. Detail contract fields validated:
   - `selector`
   - `tag`
   - `computed_styles`
   - `parent_context`
   - `css_framework`
4. Generation contract checks:
   - `visual_test` contains `test(` and `page.goto(`
   - `annotation_report` contains markdown report structure
   - `annotation_issues` contains `issues[]` and `total_count`.
5. Runner contract checks:
   - when `VERSION` and `/health.version` differ, stale daemon is not reused for resumed smoke runs.

## Code Paths

1. `scripts/smoke-tests/31-annotation-parity.sh`
2. `scripts/smoke-test.sh`
3. `scripts/smoke-tests/annotation-parity-benchmark.sh`
4. `package.json` (`smoke:annotation-parity`, `smoke:annotation-parity-suite`, `smoke:annotation-parity-benchmark`)
5. `cmd/dev-console/server_routes_media_draw_mode.go`
6. `cmd/dev-console/tools_analyze_annotations_handlers.go`
7. `cmd/dev-console/tools_generate_annotations.go`
8. `scripts/tests/framework.sh`

## Test Paths

1. `bash scripts/smoke-test.sh --only 31`
2. `npm run smoke:annotation-parity`
3. `npm run smoke:annotation-parity-suite`
4. `npm run smoke:annotation-parity-benchmark`

## Edit Guardrails

1. Keep this module deterministic (no manual keypress prompts, no interactive pauses when `NO_COLOR=1`).
2. Keep posted annotation payload schema aligned with server ingest structs.
3. Keep multi-project URLs on distinct localhost ports to preserve scope ambiguity checks.
4. Keep generation checks minimal-but-semantic (structure and critical tokens, not brittle full snapshots).

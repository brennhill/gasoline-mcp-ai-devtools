---
doc_type: architecture_flow_map
feature_id: feature-file-upload
status: shipped
owners: []
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# File Upload Smoke Harness

## Scope

Covers the standalone branded upload harness used by smoke and UAT tests to validate upload session, CSRF, and file submission behavior without external dependencies.

## Entrypoints

1. `scripts/smoke-test.sh --only 15`
2. `scripts/smoke-tests/15-file-upload.sh`
3. `scripts/tests/cat-24-upload.sh`
4. `python3 scripts/smoke-tests/upload-server.py <port>`

## Feature Docs

1. `docs/features/feature/file-upload/index.md`
2. `docs/features/feature/file-upload/flow-map.md`

## Primary Flow

1. Upload smoke/UAT modules start `upload-server.py` on a deterministic localhost port.
2. `GET /` issues a session cookie and directs clients to `/upload`.
3. `GET /upload` (and hardened variants) generate per-session CSRF tokens and serve branded multipart forms.
4. Browser automation calls `interact(what:"upload")` against `#file-input`.
5. `POST /upload` validates cookie + CSRF + required form parts (`Filedata`, `title`) then stores deterministic verification metadata (`md5`, `size`, `name`).
6. Successful submissions redirect to `/upload/success`, while test scripts verify payload via `/api/last-upload`.

## Error and Recovery Paths

1. Missing session cookie returns `401 Not logged in` with recovery link to `/`.
2. CSRF mismatch returns `403 CSRF token expired`; caller must reload form to refresh token.
3. Missing/empty file and missing `title` return `422` validation responses.
4. Unknown routes/methods return `404` branded error pages to keep browser context readable during debugging.

## State and Contracts

1. Session state: `csrf_tokens[session] -> token`.
2. Upload state: `last_upload` contains `id`, `name`, `size`, `md5`, `title`, `tags`, `csrf_ok`, `cookie_ok`.
3. DOM contract used by automation:
   - form action: `action="/upload"`
   - file input: `id="file-input"`, `name="Filedata"`
   - CSRF hidden field: `name="csrf_token"` with 32-char hex value
   - trust status marker for hardened pages: `id="trust-status"`
4. API contract: `GET /health` returns `{"ok":true}` and `GET /api/last-upload` returns latest metadata object or `{ "error": "no uploads yet" }`.

## Code Paths

1. `scripts/smoke-tests/upload-server.py`
2. `scripts/smoke-tests/15-file-upload.sh`
3. `scripts/tests/cat-24-upload.sh`

## Test Paths

1. `scripts/smoke-tests/test-upload-server.py`
2. `cmd/dev-console/smoke_upload_contract_test.go`
3. `scripts/smoke-tests/15-file-upload.sh`
4. `scripts/tests/cat-24-upload.sh`

## Edit Guardrails

1. Preserve upload selectors and field names (`#file-input`, `Filedata`, `csrf_token`, `title`) to avoid breaking smoke/UAT scripts.
2. Keep `/health` and `/api/last-upload` stable for diagnostic and contract checks.
3. Branded styling changes must not alter request semantics or hardened trust-check behavior.
4. Any new upload fixture route requires corresponding updates to smoke docs and tests in the same change.

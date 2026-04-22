---
doc_type: audit
status: active
last_reviewed: 2026-04-22
---

# OpenAPI Drift Backlog

Tracks endpoints where the HTTP-boundary drift checks (`scripts/check-openapi-url-drift.js` and `scripts/check-openapi-server-conformance.sh`) have an active allowlist/exclude entry pending cleanup.

## Goal

**This list stays empty.** Every entry represents a path where drift detection is deliberately suppressed, blocking real regressions on that path from being caught.

Adding an entry requires:

1. A linked GitHub issue tracking the fix.
2. A one-line justification (why the drift can't be fixed right now).
3. A matching entry in the relevant script's allowlist/exclude list.

Removing an entry should be celebrated.

## Current entries

| Path | Script | Owner | Reason | Issue |
|------|--------|-------|--------|-------|
| *(none — keep it that way)* | | | | |

## Permanent exclusions (not drift)

These paths are permanently excluded from Schemathesis because they aren't fuzz-safe. They are **not** drift and do **not** belong in the table above:

- `/tests/*`, `/logs.html`, `/docs`, `/openapi.json` — non-API HTML/JSON.
- `/setup` — one-shot setup wizard.
- `/insecure-proxy` — intentionally outside the normal routing layer.
- `/shutdown` — kills the daemon.
- `/clear` — wipes buffers.
- `/upgrade/install` — fires the installer as a side effect.

See `scripts/check-openapi-server-conformance.sh` "PERMANENT EXCLUDES" block.

## Related

- Tooling overview: `docs/features/feature/openapi-drift-tooling/` *(if created)*
- Reviewed in conjunction with `docs/audits/api-audit.md`

# Proof-First Smoke (Nightly)

This document describes how to run the proof-first smoke module (`28-proof-first.sh`) that validates ambiguous UI handling, CSP fallback behavior, and optional real social composer flows.

## Scope

The proof-first module checks:

1. Ambiguous selector flow: broad selector fails (`ambiguous_target`), scoped recovery succeeds.
2. CSP flow: `execute_js` fails on CSP-restricted page, DOM primitive fallback succeeds.
3. Optional LinkedIn composer flow: open, type, submit, verify close cue.
4. Optional Facebook composer flow: open, type, submit, verify close cue.
5. Evidence mode flow: mutating action with `evidence:"always"` returns `evidence.before` and `evidence.after` artifact paths.

Each test captures reproducible evidence:

- screenshot artifacts (before/after checkpoints)
- command correlation IDs

Artifacts are written to:

- `~/.gasoline/smoke-results/proof-first-artifacts.log`
- screenshot paths returned by `observe({what:"screenshot"})`

## Preconditions

Required:

- Extension connected and tracking a tab.
- AI Web Pilot enabled.
- Smoke daemon reachable (default `localhost:7890`).

Optional social flows:

- Logged-in browser session for the target site(s).
- `SMOKE_SOCIAL=1`.
- `SMOKE_LINKEDIN_URL` and/or `SMOKE_FACEBOOK_URL`.

## Run Modes

Run only proof-first module:

```bash
SMOKE_PROOF_FIRST=1 ./smoketest --only 28
```

Run proof-first with social flows:

```bash
SMOKE_PROOF_FIRST=1 \
SMOKE_SOCIAL=1 \
SMOKE_LINKEDIN_URL="https://www.linkedin.com/feed/" \
SMOKE_FACEBOOK_URL="https://www.facebook.com/" \
SMOKE_SOCIAL_POST_TEXT="This post written with Gasoline MCP" \
./smoketest --only 28
```

Override CSP target URL:

```bash
SMOKE_PROOF_FIRST=1 \
SMOKE_CSP_URL="https://news.google.com/home?hl=en-US" \
./smoketest --only 28
```

## CI/Nightly Recommendation

- Keep module 28 disabled in default PR smoke runs.
- Enable it in nightly or dedicated QA pipelines with the environment variables above.
- Treat missing visual checkpoints or missing close cue as test failures.

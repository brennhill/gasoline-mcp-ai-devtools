---
doc_type: feature_index
feature_id: feature-csp-safe-execution
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-02-24
code_paths:
  - src/background/csp-safe-types.ts
  - src/background/csp-safe-parser.ts
  - src/background/csp-safe-executor.ts
  - src/background/query-execution.ts
test_paths:
  - tests/extension/csp-safe-parser.test.js
  - tests/extension/csp-safe-executor.test.js
  - tests/extension/csp-safe-integration.test.js
---

# CSP-Safe JavaScript Execution

## TL;DR

- Status: implemented
- Tool: interact
- Mode/Action: execute_js
- Location: `docs/features/feature/csp-safe-execution`

## Problem

When a page's Content Security Policy (CSP) blocks `unsafe-eval`, `execute_js` fails because both execution paths use `new Function(code)`. This affects sites like LinkedIn, GitHub, and many enterprise apps.

## Solution: Three-Tier Fallback Chain

| Tier | World | JS Capability | Page Globals | CSP Safe |
|------|-------|--------------|--------------|----------|
| 1. new Function (MAIN) | MAIN | 100% | Yes | No |
| 2. new Function (ISOLATED) | ISOLATED | 100% | No | Yes |
| 3. Structured executor | MAIN | ~85% expressions | Yes | Yes |

Tier 2 is the big win: content scripts in ISOLATED world are exempt from page CSP, so `new Function()` works there. Tier 3 handles the rare case where MAIN world page globals are needed on CSP pages.

## Code and Tests

- Types: `src/background/csp-safe-types.ts`
- Parser: `src/background/csp-safe-parser.ts`
- Executor: `src/background/csp-safe-executor.ts`
- Integration: `src/background/query-execution.ts`
- Tests: `tests/extension/csp-safe-{parser,executor,integration}.test.js`

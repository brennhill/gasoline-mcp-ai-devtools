---
doc_type: flow_map
flow_id: dom-selector-resolution-and-disambiguation
status: active
last_reviewed: 2026-03-02
owners:
  - Brenn
---

# DOM Selector Resolution and Disambiguation

## Scope

Selector resolution inside extension DOM primitives used by `interact` mutating actions (`click`, `type`, `select`, `focus`, and intent helpers).

## Entrypoints

1. `interact` tool calls DOM primitive actions through the extension command path.
2. Action target resolution calls `resolveElements` and `resolveElement` in DOM primitives.

## Primary Flow

1. `list_interactive` enumerates candidates and may emit `:nth-match(N)` selectors for duplicates.
2. A later mutating action receives the selector.
3. `parseNthMatchSelector` extracts base selector + 1-based ordinal when suffix is present.
4. `resolveElements` resolves the base selector and narrows to the indexed candidate.
5. `resolveElement` uses the same helper for single-target resolution.
6. Action runs against the resolved element and reports match strategy.

## Error and Recovery Paths

1. Invalid `:nth-match(N)` format returns no match.
2. Out-of-range ordinal returns no match.
3. Duplicate unresolved selectors surface `ambiguous_target` guidance to use `nth`, `:nth-match(N)`, or `scope_selector`.

## State and Contracts

1. `:nth-match(N)` is a stable contract between `list_interactive` output and mutating action input.
2. Ordinal is 1-based and scoped to the evaluated selector context.

## Code Paths

- `src/background/dom-primitives.ts`
- `scripts/templates/partials/_dom-selectors.tpl`
- `extension/background/dom-primitives.js`

## Test Paths

- `extension/background/dom-primitives.test.js`

## Edit Guardrails

1. Any change to `:nth-match(N)` parsing must keep `resolveElements` and `resolveElement` behavior aligned.
2. Template and generated source must stay synchronized (`node scripts/generate-dom-primitives.js --check`).
3. Add regression coverage when changing ambiguity/disambiguation behavior.

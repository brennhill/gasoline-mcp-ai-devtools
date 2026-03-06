---
doc_type: requirement_standard
scope: traceability
last_reviewed: 2026-02-16
---

# Requirement Traceability Standard

## Objective

Use stable requirement IDs so product intent can be traced to implementation and tests.

## ID Format

- Prefix: feature ID uppercased with separators converted to `_`.
- Suffix: 3-digit sequence.
- Example: `FEATURE_QUERY_DOM_001`

## Where IDs Appear

- `product-spec.md`: requirement definition
- `tech-spec.md`: implementation mapping
- `qa-plan.md`: validation mapping
- tests: test name/comment referencing requirement ID

## Mapping Pattern

- Product requirement: what behavior is required.
- Tech mapping: which code paths satisfy it.
- QA mapping: which tests prove it.

Use `docs/traceability/feature-map.md` as the generated feature-level entrypoint.

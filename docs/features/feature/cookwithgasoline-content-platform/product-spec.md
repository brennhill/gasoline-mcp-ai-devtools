---
doc_type: product_spec
feature_id: feature-cookwithgasoline-content-platform
status: in_progress
last_reviewed: 2026-03-03
owners:
  - Brenn
---

# Product Spec

## Objective

Keep `cookwithgasoline.com` aligned with current product capabilities while preserving a high-conversion marketing layout and machine-readable docs output (`*.md`, `llms.txt`, `llms-full.txt`).

## Scope

- Homepage and discovery pages with modern rhythm and spacing.
- Tool reference pages for `observe`, `analyze`, `interact`, `configure`, and `generate`.
- Blog/article surfaces and markdown mirror routes for agent consumption.
- Contract checks that prevent docs drift when tool modes/actions change in code.

## User Outcomes

- Developers can scan homepage + solutions quickly and understand core capabilities.
- Readers can find accurate, current tool parameters/actions in reference docs.
- Agents can parse deterministic markdown endpoints for every docs/blog route.

## Non-Goals

- Final illustration system (placeholder graphics are acceptable during redesign).
- New backend feature delivery unrelated to documentation or site presentation.

## Linked Architecture

- Canonical flow map: [flow-map.md](./flow-map.md)

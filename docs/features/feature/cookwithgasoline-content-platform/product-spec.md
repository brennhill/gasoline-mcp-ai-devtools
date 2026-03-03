---
doc_type: product-spec
feature_id: feature-cookwithgasoline-content-platform
status: in_progress
last_reviewed: 2026-03-03
---

# Cookwithgasoline Content Platform Product Spec

## Problem

Gasoline needs a first-class documentation and discovery surface that works for both humans and LLM agents:
1. Homepage should clearly explain capability and value.
2. Workflow discovery should make common tasks easy to find and execute.
3. Markdown mirrors should provide stable, crawlable, agent-readable routes.
4. CI must enforce content-contract quality for changed docs/blog files.

## Product Requirement

Deliver a content platform at `cookwithgasoline.com` that supports:
1. Human-oriented navigation and workflow discovery.
2. Agent-oriented markdown exports with canonical paths and metadata.
3. Repeatable contract validation in CI so content quality does not drift.

## Current Scope

1. Landing and workflow/article discovery components.
2. Markdown route generation for docs/blog content.
3. LLM-focused text exports (`llms.txt` and full variant).
4. Content contract linter integrated into CI.

## Non-Goals

1. Replacing source docs authoring in the main docs tree.
2. Implementing full CMS/editor workflows in this phase.

## User Value

1. Faster onboarding through clearer capability and workflow discovery.
2. Better retrieval and grounding for agent clients via markdown mirrors.
3. Lower release risk via automated content-contract enforcement.

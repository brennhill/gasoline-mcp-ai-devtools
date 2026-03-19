---
title: API and Tool Reference
description: Readthedocs-style entry point for Strum tool APIs, request shapes, and common usage patterns.
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['reference']
---

Use this page as the structured starting point for all MCP tool docs.

## Quick Reference

| Tool | Primary Purpose | Entry Doc | Executable Examples |
| --- | --- | --- | --- |
| `Observe` | Read browser logs, requests, errors, and captured state | [Observe](/reference/observe/) | [Observe Examples](/reference/examples/observe-examples/) |
| `Analyze` | Run audits and deeper analyses (performance, accessibility, security) | [Analyze](/reference/analyze/) | [Analyze Examples](/reference/examples/analyze-examples/) |
| `Interact` | Navigate pages and perform browser actions | [Interact](/reference/interact/) | [Interact Examples](/reference/examples/interact-examples/) |
| `Configure` | Manage sessions, noise rules, persistence, and recorder settings | [Configure](/reference/configure/) | [Configure Examples](/reference/examples/configure-examples/) |
| `Generate` | Produce reproductions, reports, and tests from captured data | [Generate](/reference/generate/) | [Generate Examples](/reference/examples/generate-examples/) |

## Common Parameters

These parameters appear repeatedly across Gasoline tools:

| Parameter | Purpose |
| --- | --- |
| `what` | Selects the mode/action to execute |
| `tab_id` | Targets a specific browser tab |
| `timeout_ms` | Sets bounded waiting time for operations |
| `summary` | Returns compact output for faster triage |
| `correlation_id` | Links async actions, follow-up polling, and evidence |

## Readthedocs-style Navigation

1. Start with [Getting Started](/getting-started/) for setup.
2. Open one tool page and skim the parameter table first.
3. Use guides for workflow patterns:
   - [Debug Web Apps](/guides/debug-webapps/)
   - [Demo Scripts](/guides/demo-scripts/)
   - [API Validation](/guides/api-validation/)
   - [Automate and Notify](/guides/automate-and-notify/)
4. Validate behavior with generated markdown endpoints (`*.md`) for agent parsing.

## Agent-friendly Formats

Each docs, blog, and articles route has a markdown endpoint:

- HTML: `https://usestrum.dev/guides/debug-webapps/`
- Markdown: `https://usestrum.dev/guides/debug-webapps.md`

Catalogs:

- [`/llms.txt`](/llms.txt)
- [`/llms-full.txt`](/llms-full.txt)

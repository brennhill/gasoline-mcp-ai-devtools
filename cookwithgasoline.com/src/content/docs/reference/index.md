---
title: API and Tool Reference
description: Readthedocs-style entry point for Gasoline MCP tool APIs, request shapes, and common usage patterns.
---

Use this page as the structured starting point for all MCP tool docs.

## Quick Reference

| Tool | Primary Purpose | Entry Doc |
| --- | --- | --- |
| `Observe` | Read browser logs, requests, errors, and captured state | [Observe](/reference/observe/) |
| `Analyze` | Run audits and deeper analyses (performance, accessibility, security) | [Analyze](/reference/analyze/) |
| `Interact` | Navigate pages and perform browser actions | [Interact](/reference/interact/) |
| `Configure` | Manage sessions, noise rules, persistence, and recorder settings | [Configure](/reference/configure/) |
| `Generate` | Produce reproductions, reports, and tests from captured data | [Generate](/reference/generate/) |

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

- HTML: `https://cookwithgasoline.com/guides/debug-webapps/`
- Markdown: `https://cookwithgasoline.com/guides/debug-webapps.md`

Catalogs:

- [`/llms.txt`](/llms.txt)
- [`/llms-full.txt`](/llms-full.txt)

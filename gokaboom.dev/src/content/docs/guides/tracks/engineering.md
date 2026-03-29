---
title: Engineering Track
description: A focused start-here path for engineers using KaBOOM.
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'tracks', 'engineering']
---

## Goal

Ship fixes faster with deterministic debugging evidence.

## Week 1 Path

1. Set up local workflow with [Getting Started](/getting-started/).
2. Run your first triage loop with [Debug Web Apps](/guides/debug-webapps/).
3. Capture API and WebSocket evidence with [API Validation](/guides/api-validation/) and [WebSocket Debugging](/guides/websocket-debugging/).
4. Lock in regression safety with [Resilient UAT Scripts](/guides/resilient-uat/).

## Daily Workflow

1. `observe(errors|network_bodies|websocket_events)` for state.
2. `interact(...)` to reproduce deterministically.
3. `analyze(...)` for audits and clustering.
4. `generate(test|reproduction|har)` to export evidence.

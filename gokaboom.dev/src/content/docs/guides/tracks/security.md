---
title: Security Track
description: A role path for security and compliance teams validating browser-layer risks.
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'tracks', 'security']
---

## Goal

Shift browser security checks earlier in delivery.

## Week 1 Path

1. Start with [Security Auditing](/guides/security-auditing/).
2. Add protocol-level checks with [API Validation](/guides/api-validation/).
3. Validate user-impacting checks via [Accessibility](/guides/accessibility/).
4. Export findings from [generate()](/reference/generate/) for CI and review workflows.

## Operational Pattern

1. Observe risk signals (`errors`, `network_bodies`, `websocket_events`).
2. Run targeted audits (`security_audit`, `third_party_audit`).
3. Export structured outputs (SARIF, HAR, CSP suggestions).

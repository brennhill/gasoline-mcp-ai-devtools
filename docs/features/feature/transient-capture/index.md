# Transient Capture

Captures transient UI elements (tooltips, toasts, dropdowns, modals) that appear briefly and disappear, ensuring they are observable for debugging and testing.

## Overview

Transient UI elements are notoriously difficult to inspect because they vanish on interaction loss. This feature provides mechanisms to detect, capture, and preserve the state of ephemeral DOM elements before they disappear.

## Key Capabilities

- Detection of short-lived DOM mutations (tooltips, notifications, popovers)
- Snapshot preservation of transient element content and styles
- Integration with the observe tool for historical access to captured transients

## Code References

- `src/lib/transient-capture.ts` — Extension-side transient element detection and capture

## Status

**Shipped** — Active in production with extension-side capture logic.

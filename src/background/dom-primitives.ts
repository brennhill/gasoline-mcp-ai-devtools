// AUTO-GENERATED FILE. DO NOT EDIT DIRECTLY.
// Source: scripts/templates/dom-primitives.ts.tpl
// Action logic extracted to self-contained modules:
//   dom-primitives-read.ts, dom-primitives-action.ts,
//   dom-primitives-intent.ts, dom-primitives-overlay.ts, dom-primitives-stability.ts
// Generator: scripts/generate-dom-primitives.js

/**
 * Purpose: Thin entry point for DOM primitives. Delegates to extracted self-contained modules:
 *   - dom-primitives-read.ts (get_text, get_value, get_attribute, wait_for, wait_for_text, wait_for_absent)
 *   - dom-primitives-action.ts (click, type, select, check, set_attribute, paste, key_press, hover, focus, scroll_to)
 *   - dom-primitives-intent.ts (open_composer, submit_active_composer, confirm_top_dialog)
 *   - dom-primitives-overlay.ts (dismiss_top_overlay, auto_dismiss_overlays)
 *   - dom-primitives-stability.ts (wait_for_stable, action_diff)
 *   - dom-primitives-list-interactive.ts (list_interactive)
 *   - dom-primitives-query.ts (query)
 * Docs: docs/features/feature/interact-explore/index.md
 */
// dom-primitives.ts — DOM primitive entry point and backward-compatible helpers.

import type { DOMPrimitiveOptions, DOMResult } from './dom-types.js'
import { domPrimitiveRead } from './dom-primitives-read.js'
import { domPrimitiveAction } from './dom-primitives-action.js'

// Re-export list_interactive primitive for backward compatibility
export { domPrimitiveListInteractive } from './dom-primitives-list-interactive.js'

// Re-export extracted primitives for direct dispatch from dom-dispatch.ts
export { domPrimitiveRead } from './dom-primitives-read.js'
export { domPrimitiveAction } from './dom-primitives-action.js'

const READ_ACTIONS = new Set(['get_text', 'get_value', 'get_attribute', 'wait_for', 'wait_for_text', 'wait_for_absent'])

/**
 * Unified dispatcher that delegates to the appropriate self-contained primitive.
 * Used by domWaitFor and backward-compatible call sites.
 * For production dispatch, dom-dispatch.ts routes directly to specific primitives.
 */
export function domPrimitive(
  action: string,
  selector: string,
  options: DOMPrimitiveOptions
): DOMResult | Promise<DOMResult> | {
  success: boolean
  elements: unknown[]
  candidate_count?: number
  scope_rect_used?: { x: number; y: number; width: number; height: number }
  error?: string
  message?: string
} {
  if (READ_ACTIONS.has(action)) {
    return domPrimitiveRead(action, selector, options)
  }
  // All mutating selector-based actions go to domPrimitiveAction
  return domPrimitiveAction(action, selector, options)
}

/**
 * Backward-compatible wait helper used by unit tests and legacy call sites.
 * Polls wait_for and listens for DOM mutations for fast resolution.
 */
export function domWaitFor(selector: string, timeoutMs: number = 5000): Promise<DOMResult> {
  const timeout = Math.max(1, timeoutMs)
  const startedAt = Date.now()
  const pollIntervalMs = 50

  return new Promise<DOMResult>((resolve) => {
    let settled = false
    let timer: ReturnType<typeof setTimeout> | null = null
    let observer: MutationObserver | null = null

    const done = (result: DOMResult) => {
      if (settled) return
      settled = true
      if (timer) clearTimeout(timer)
      if (observer) observer.disconnect()
      resolve(result)
    }

    const check = () => {
      const result = domPrimitiveRead('wait_for', selector, { timeout_ms: timeout })
      if (result?.success) {
        done(result)
        return
      }
      if (Date.now() - startedAt >= timeout) {
        done({
          success: false,
          action: 'wait_for',
          selector,
          error: 'timeout',
          message: `Element not found within ${timeout}ms: ${selector}`
        } as DOMResult)
        return
      }
      timer = setTimeout(check, pollIntervalMs)
    }

    try {
      observer = new MutationObserver(() => {
        if (settled) return
        const immediate = domPrimitiveRead('wait_for', selector, { timeout_ms: timeout })
        if (immediate?.success) done(immediate)
      })
      observer.observe(document.body || document.documentElement, {
        childList: true,
        subtree: true,
        attributes: true,
        characterData: true
      })
    } catch {
      // Best-effort optimization only; polling remains authoritative.
    }

    check()
  })
}

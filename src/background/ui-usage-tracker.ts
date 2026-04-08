// ui-usage-tracker.ts — Tracks extension-UI-originated feature usage for sync payload.
// Only tracks actions triggered by the user in the extension UI (popup, context menu,
// keyboard shortcut) — NOT actions dispatched by AI/MCP tool calls.

// =============================================================================
// TYPES
// =============================================================================

/** Features that can be triggered from the extension UI. */
export type UIFeature = 'screenshot' | 'annotations' | 'video' | 'dom_action'

// =============================================================================
// STATE
// =============================================================================

const pending: Map<UIFeature, boolean> = new Map()

// =============================================================================
// PUBLIC API
// =============================================================================

/**
 * Record that a UI feature was used. Called from context menus, popup buttons,
 * keyboard shortcuts — anywhere the user triggers an action without AI.
 */
export function trackUIFeature(feature: UIFeature): void {
  pending.set(feature, true)
}

/**
 * Drain pending features for inclusion in the next sync request.
 * Returns the map and clears internal state. Returns undefined if empty.
 */
export function drainUIFeatures(): Record<string, boolean> | undefined {
  if (pending.size === 0) return undefined
  const result: Record<string, boolean> = {}
  for (const [key, val] of pending) {
    result[key] = val
  }
  pending.clear()
  return result
}

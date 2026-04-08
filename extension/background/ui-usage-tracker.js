// ui-usage-tracker.ts — Tracks extension-UI-originated feature usage for sync payload.
// Only tracks actions triggered by the user in the extension UI (popup, context menu,
// keyboard shortcut) — NOT actions dispatched by AI/MCP tool calls.
// =============================================================================
// STATE
// =============================================================================
const pending = new Map();
// =============================================================================
// PUBLIC API
// =============================================================================
/**
 * Record that a UI feature was used. Called from context menus, popup buttons,
 * keyboard shortcuts — anywhere the user triggers an action without AI.
 */
export function trackUIFeature(feature) {
    pending.set(feature, true);
}
/**
 * Drain pending features for inclusion in the next sync request.
 * Returns the map and clears internal state. Returns undefined if empty.
 */
export function drainUIFeatures() {
    if (pending.size === 0)
        return undefined;
    const result = {};
    for (const [key, val] of pending) {
        result[key] = val;
    }
    pending.clear();
    return result;
}
//# sourceMappingURL=ui-usage-tracker.js.map
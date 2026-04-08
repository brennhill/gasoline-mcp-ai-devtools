/** Features that can be triggered from the extension UI. */
export type UIFeature = 'screenshot' | 'annotations' | 'video' | 'dom_action';
/**
 * Record that a UI feature was used. Called from context menus, popup buttons,
 * keyboard shortcuts — anywhere the user triggers an action without AI.
 */
export declare function trackUIFeature(feature: UIFeature): void;
/**
 * Drain pending features for inclusion in the next sync request.
 * Returns the map and clears internal state. Returns undefined if empty.
 */
export declare function drainUIFeatures(): Record<string, boolean> | undefined;
//# sourceMappingURL=ui-usage-tracker.d.ts.map
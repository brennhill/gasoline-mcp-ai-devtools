/**
 * Purpose: Handles content-script message relay between background and inject contexts.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */
/**
 * Update tracking status by checking storage and current tab ID.
 * Called on script load, storage changes, and tab activation.
 */
export declare function updateTrackingStatus(): Promise<void>;
/**
 * Get the current tracking status
 */
export declare function getIsTrackedTab(): boolean;
/**
 * Get the current tab ID
 */
export declare function getCurrentTabId(): number | null;
/**
 * Initialize tab tracking (call once on script load).
 * Returns a promise that resolves when initial tracking status is known.
 * The onChange callback fires after each status update (initial + storage changes).
 */
export declare function initTabTracking(onChange?: (tracked: boolean) => void): Promise<void>;
//# sourceMappingURL=tab-tracking.d.ts.map
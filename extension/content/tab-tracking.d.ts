/**
 * @fileoverview Tab Tracking Module
 * Manages tracking status for the current tab
 */
/**
 * Update tracking status by checking storage and current tab ID.
 * Called on script load, storage changes, and tab activation.
 */
export declare function updateTrackingStatus(): Promise<void>
/**
 * Get the current tracking status
 */
export declare function getIsTrackedTab(): boolean
/**
 * Get the current tab ID
 */
export declare function getCurrentTabId(): number | null
/**
 * Initialize tab tracking (call once on script load)
 */
export declare function initTabTracking(): void
//# sourceMappingURL=tab-tracking.d.ts.map

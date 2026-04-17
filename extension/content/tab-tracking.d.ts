/**
 * Purpose: Tracks whether this content script's tab is the currently tracked tab via chrome.storage change listeners.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
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
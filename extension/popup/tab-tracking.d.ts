/**
 * Purpose: Manages popup tab-tracking UI state and track/untrack transitions for the active browser tab.
 * Why: Keeps the tracked-tab lifecycle explicit so content-script injection and status UX stay synchronized.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
export declare function initTrackPageButton(): void;
/**
 * Handle clicking on the tracked URL.
 * Switches to the tracked tab.
 */
export declare function handleUrlClick(tabId: number | undefined): Promise<void>;
/**
 * Handle Track This Tab button click.
 * Toggles tracking on/off for the current tab.
 * Blocks tracking on internal Chrome pages.
 */
export declare function handleTrackPageClick(): Promise<void>;
//# sourceMappingURL=tab-tracking.d.ts.map
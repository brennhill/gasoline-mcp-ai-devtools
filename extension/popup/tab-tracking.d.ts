/**
 * @fileoverview Tab Tracking Module for Popup
 * Manages the "Track This Tab" button and tracking status
 */
/**
 * Initialize the Track This Tab button.
 * Shows current tracking status and handles track/untrack.
 * Disables the button on internal Chrome pages where tracking is impossible.
 */
export declare function initTrackPageButton(): Promise<void>
/**
 * Handle clicking on the tracked URL.
 * Switches to the tracked tab.
 */
export declare function handleUrlClick(tabId: number | undefined): Promise<void>
/**
 * Handle Track This Tab button click.
 * Toggles tracking on/off for the current tab.
 * Blocks tracking on internal Chrome pages.
 */
export declare function handleTrackPageClick(): Promise<void>
//# sourceMappingURL=tab-tracking.d.ts.map

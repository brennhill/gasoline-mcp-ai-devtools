/**
 * Purpose: Handles content-script message relay between background and inject contexts.
 * Why: Keeps content-script bridging predictable between extension and page contexts.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */
/**
 * Remove the subtitle element, clean up Escape listener.
 */
export declare function clearSubtitle(): void;
/**
 * Show or update a persistent subtitle bar at the bottom of the viewport.
 * Empty text clears the subtitle. Includes a hover close button and
 * Escape key listener for dismissal.
 */
export declare function showSubtitle(text: string): void;
/**
 * Show or hide a recording watermark (Gasoline flame icon) in the bottom-right corner.
 * The icon renders at 64x64px with 50% opacity, captured in the tab video.
 */
export declare function toggleRecordingWatermark(visible: boolean): void;
//# sourceMappingURL=subtitle.d.ts.map
/**
 * Purpose: Shared recording helpers: slug generation, toast labels, and badge timer lifecycle.
 * Why: Consolidates all recording utility functions so callers have a single import point.
 * Docs: docs/features/feature/flow-recording/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 */
/**
 * Build a filesystem-safe recording slug from the current tab URL.
 */
export declare function buildScreenRecordingSlug(url: string | undefined): string;
/**
 * Build a short human-readable recording toast label from a tab URL.
 */
export declare function buildRecordingToastLabel(url: string | undefined): string;
export declare function startRecordingBadgeTimer(startTime: number): void;
export declare function stopRecordingBadgeTimer(): void;
//# sourceMappingURL=recording-utils.d.ts.map
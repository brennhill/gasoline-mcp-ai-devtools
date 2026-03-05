/**
 * Purpose: Shared frame-target normalization and probing for background command handlers.
 * Why: Keep frame matching behavior/error contracts consistent across analyze/interact paths.
 * Docs: docs/features/feature/interact-explore/index.md
 */
export declare const INVALID_FRAME_ERROR = "invalid_frame: frame parameter must be a CSS selector, 0-based index, or \"all\". Got unsupported type or value";
export declare const FRAME_NOT_FOUND_ERROR = "frame_not_found: no iframe matched the given selector or index. Verify the iframe exists and is loaded on the page";
export type NormalizedFrameTarget = string | number | 'all' | undefined;
/**
 * Normalize and validate a frame argument from tool params.
 * Throws with a stable error contract for unsupported values.
 */
export declare function normalizeFrameArg(frame: unknown): NormalizedFrameTarget;
/**
 * Probe all frames and return frame IDs matching the supplied target.
 * The probe function must be self-contained for chrome.scripting.executeScript.
 */
export declare function resolveMatchedFrameIds(tabId: number, frameTarget: string | number | 'all', probeFn: (frameTarget: string | number) => {
    matches: boolean;
}): Promise<number[]>;
//# sourceMappingURL=frame-targeting.d.ts.map
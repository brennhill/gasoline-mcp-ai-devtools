/**
 * Purpose: Frame-matching probe executed in page context for targeted DOM actions.
 * Why: Centralizes extension coordination to reduce race conditions and split-brain state.
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * Must stay self-contained for chrome.scripting.executeScript({ func }).
 */
export declare function domFrameProbe(frameTarget: string | number): {
    matches: boolean;
};
//# sourceMappingURL=dom-frame-probe.d.ts.map
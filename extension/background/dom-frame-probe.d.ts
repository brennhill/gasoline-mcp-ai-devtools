/**
 * Purpose: Frame-matching probe executed in page context for targeted DOM actions.
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * Must stay self-contained for chrome.scripting.executeScript({ func }).
 */
export declare function domFrameProbe(frameTarget: string | number): {
    matches: boolean;
};
//# sourceMappingURL=dom-frame-probe.d.ts.map
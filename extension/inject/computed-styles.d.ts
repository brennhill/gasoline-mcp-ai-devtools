/**
 * Purpose: Executes in-page actions and query handlers within the page context.
 * Why: Executes page-context actions safely while preserving deterministic command results.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */
interface ComputedStylesParams {
    selector: string;
    properties?: string[];
}
interface ElementStyleResult {
    selector: string;
    tag: string;
    computed_styles: Record<string, string>;
    box_model: {
        x: number;
        y: number;
        width: number;
        height: number;
        top: number;
        right: number;
        bottom: number;
        left: number;
    };
    contrast_ratio?: number;
}
/**
 * Query computed styles for all elements matching a CSS selector.
 */
export declare function queryComputedStyles(params: ComputedStylesParams): ElementStyleResult[];
export {};
//# sourceMappingURL=computed-styles.d.ts.map
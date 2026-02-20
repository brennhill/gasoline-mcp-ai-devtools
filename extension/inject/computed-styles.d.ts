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
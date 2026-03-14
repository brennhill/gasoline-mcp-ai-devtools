/**
 * Purpose: Extracts comprehensive page metadata (viewport, headings, forms, links) for page-level context.
 * Why: Separates page-level metadata extraction from per-element DOM querying so each can evolve independently.
 * Docs: docs/features/feature/query-dom/index.md
 */
interface PageInfoResult {
    url: string;
    title: string;
    viewport: {
        width: number;
        height: number;
    };
    scroll: {
        x: number;
        y: number;
    };
    documentHeight: number;
    headings: string[];
    links: number;
    images: number;
    interactiveElements: number;
    forms: FormInfo[];
}
interface FormInfo {
    id?: string;
    action?: string;
    fields: string[];
}
/**
 * Get comprehensive page info
 */
export declare function getPageInfo(): Promise<PageInfoResult>;
export {};
//# sourceMappingURL=page-info.d.ts.map
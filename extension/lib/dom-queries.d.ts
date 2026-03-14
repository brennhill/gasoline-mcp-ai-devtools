/**
 * Purpose: Structured DOM querying and page info extraction for the inject context.
 * Docs: docs/features/feature/query-dom/index.md
 */
export { runAxeAudit, runAxeAuditWithTimeout, formatAxeResults } from './a11y-audit.js';
export { getPageInfo } from './page-info.js';
export interface DOMQueryParams {
    selector: string;
    include_styles?: boolean;
    properties?: string[];
    include_children?: boolean;
    max_depth?: number;
}
interface BoundingBox {
    x: number;
    y: number;
    width: number;
    height: number;
}
interface DOMElementEntry {
    tag: string;
    text: string;
    visible: boolean;
    attributes?: Record<string, string>;
    boundingBox?: BoundingBox;
    styles?: Record<string, string>;
    children?: DOMElementEntry[];
}
interface DOMQueryResult {
    url: string;
    title: string;
    matchCount: number;
    returnedCount: number;
    matches: DOMElementEntry[];
}
/**
 * Execute a DOM query and return structured results
 */
export declare function executeDOMQuery(params: DOMQueryParams): Promise<DOMQueryResult>;
//# sourceMappingURL=dom-queries.d.ts.map
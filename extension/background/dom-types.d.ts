/**
 * Purpose: Shared DOM action contracts used by background dispatch and injected primitives.
 * Docs: docs/features/feature/interact-explore/index.md
 */
export interface DOMMutationEntry {
    type: 'added' | 'removed' | 'attribute';
    tag?: string;
    id?: string;
    class?: string;
    text_preview?: string;
    attribute?: string;
    old_value?: string;
    new_value?: string;
}
export interface ScopeRect {
    x: number;
    y: number;
    width: number;
    height: number;
}
export interface BoundingBox {
    x: number;
    y: number;
    width: number;
    height: number;
}
export interface DOMResult {
    success: boolean;
    action: string;
    selector: string;
    value?: unknown;
    candidate_count?: number;
    scope_rect_used?: ScopeRect;
    match_count?: number;
    match_strategy?: string;
    matched?: {
        tag?: string;
        role?: string;
        aria_label?: string;
        text_preview?: string;
        selector?: string;
        element_id?: string;
        bbox?: BoundingBox;
        scope_selector_used?: string;
        scope_rect_used?: ScopeRect;
        frame_id?: number;
    };
    candidates?: Array<{
        tag?: string;
        role?: string;
        aria_label?: string;
        text_preview?: string;
        selector?: string;
        element_id?: string;
        bbox?: BoundingBox;
        visible?: boolean;
    }>;
    reason?: string;
    error?: string;
    message?: string;
    dom_summary?: string;
    timing?: {
        total_ms: number;
    };
    dom_changes?: {
        added: number;
        removed: number;
        modified: number;
        summary: string;
    };
    dom_mutations?: DOMMutationEntry[];
    analysis?: string;
}
export interface DOMPrimitiveOptions {
    text?: string;
    value?: string;
    clear?: boolean;
    checked?: boolean;
    name?: string;
    timeout_ms?: number;
    analyze?: boolean;
    observe_mutations?: boolean;
    element_id?: string;
    scope_selector?: string;
    scope_rect?: ScopeRect;
    new_tab?: boolean;
}
export interface DOMActionParams extends DOMPrimitiveOptions {
    action?: string;
    selector?: string;
    reason?: string;
    frame?: string | number;
}
//# sourceMappingURL=dom-types.d.ts.map
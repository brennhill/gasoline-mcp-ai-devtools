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
export interface DOMResult {
    success: boolean;
    action: string;
    selector: string;
    value?: unknown;
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
    execution_world?: 'main' | 'isolated';
    fallback_attempted?: boolean;
    main_world_status?: 'success' | 'error' | 'not_attempted';
    isolated_world_status?: 'success' | 'error' | 'not_attempted';
    fallback_summary?: string;
    main_world_error?: string;
    isolated_world_error?: string;
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
}
export interface DOMActionParams extends DOMPrimitiveOptions {
    action?: string;
    selector?: string;
    reason?: string;
    frame?: string | number;
    world?: 'auto' | 'main' | 'isolated';
}
//# sourceMappingURL=dom-types.d.ts.map
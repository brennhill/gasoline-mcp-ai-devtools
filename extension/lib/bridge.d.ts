/**
 * Purpose: Provides shared runtime utilities used by extension and server workflows.
 * Docs: docs/features/feature/observe/index.md
 */
export interface BridgePayload {
    level?: string;
    message?: string;
    error?: string;
    args?: unknown[];
    filename?: string;
    lineno?: number;
    [key: string]: unknown;
}
/**
 * Post a log message to the content script
 */
export declare function postLog(payload: BridgePayload): void;
//# sourceMappingURL=bridge.d.ts.map
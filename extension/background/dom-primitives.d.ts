import type { PendingQuery } from '../types/queries';
import type { SyncClient } from './sync-client';
interface DOMResult {
    success: boolean;
    action: string;
    selector: string;
    value?: unknown;
    error?: string;
    message?: string;
}
type SendAsyncResult = (syncClient: SyncClient, queryId: string, correlationId: string, status: 'complete' | 'error' | 'timeout', result?: unknown, error?: string) => void;
type ActionToast = (tabId: number, text: string, detail?: string, state?: 'trying' | 'success' | 'warning' | 'error', durationMs?: number) => void;
/**
 * Single self-contained function for all DOM primitives.
 * Passed to chrome.scripting.executeScript({ func: domPrimitive, args: [...] }).
 * MUST NOT reference any module-level variables — Chrome serializes the function source only.
 */
export declare function domPrimitive(action: string, selector: string, options: {
    text?: string;
    value?: string;
    clear?: boolean;
    checked?: boolean;
    name?: string;
    timeout_ms?: number;
}): DOMResult | Promise<DOMResult> | {
    success: boolean;
    elements: unknown[];
};
/**
 * wait_for variant that polls with MutationObserver (used when element not found initially).
 * Separate function because it returns a Promise.
 */
export declare function domWaitFor(selector: string, timeoutMs: number): Promise<DOMResult>;
export declare function executeDOMAction(query: PendingQuery, tabId: number, syncClient: SyncClient, sendAsyncResult: SendAsyncResult, actionToast: ActionToast): Promise<void>;
export {};
//# sourceMappingURL=dom-primitives.d.ts.map
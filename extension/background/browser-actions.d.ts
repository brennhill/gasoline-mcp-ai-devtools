import type { PendingQuery } from '../types';
import type { SyncClient } from './sync-client';
import type { SendAsyncResultFn, ActionToastFn } from './pending-queries';
export type BrowserActionResult = {
    success: boolean;
    action?: string;
    url?: string;
    content_script_status?: string;
    message?: string;
    error?: string;
    summary?: unknown;
    summary_error?: string;
};
export type BrowserActionParams = {
    action?: string;
    url?: string;
    reason?: string;
    summary_script?: string;
};
export declare function handleNavigateAction(tabId: number, url: string, actionToast: ActionToastFn, reason?: string): Promise<BrowserActionResult>;
export declare function handleBrowserAction(tabId: number, params: BrowserActionParams, actionToast: ActionToastFn): Promise<BrowserActionResult>;
export declare function handleAsyncExecuteCommand(query: PendingQuery, tabId: number, world: string, syncClient: SyncClient, sendAsyncResult: SendAsyncResultFn, actionToast: ActionToastFn): Promise<void>;
export declare function handleAsyncBrowserAction(query: PendingQuery, tabId: number, params: BrowserActionParams, syncClient: SyncClient, sendAsyncResult: SendAsyncResultFn, actionToast: ActionToastFn): Promise<void>;
//# sourceMappingURL=browser-actions.d.ts.map

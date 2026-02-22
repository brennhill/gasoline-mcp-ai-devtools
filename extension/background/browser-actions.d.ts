/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
import type { PendingQuery } from '../types';
import type { SyncClient } from './sync-client';
import type { SendAsyncResultFn, ActionToastFn } from './pending-queries';
export type BrowserActionResult = {
    success: boolean;
    action?: string;
    url?: string;
    final_url?: string;
    title?: string;
    tab_id?: number;
    tab_index?: number;
    closed_tab_id?: number;
    content_script_status?: string;
    message?: string;
    error?: string;
    csp_blocked?: boolean;
    failure_cause?: string;
};
export declare function handleNavigateAction(tabId: number, url: string, actionToast: ActionToastFn, reason?: string): Promise<BrowserActionResult>;
export declare function handleBrowserAction(tabId: number, params: {
    action?: string;
    what?: string;
    url?: string;
    reason?: string;
    tab_id?: number;
    tab_index?: number;
    new_tab?: boolean;
}, actionToast: ActionToastFn): Promise<BrowserActionResult>;
export declare function handleAsyncExecuteCommand(query: PendingQuery, tabId: number, world: string, syncClient: SyncClient, sendAsyncResult: SendAsyncResultFn, actionToast: ActionToastFn): Promise<void>;
export declare function handleAsyncBrowserAction(query: PendingQuery, tabId: number, params: {
    action?: string;
    what?: string;
    url?: string;
    tab_id?: number;
    tab_index?: number;
    new_tab?: boolean;
}, syncClient: SyncClient, sendAsyncResult: SendAsyncResultFn, actionToast: ActionToastFn): Promise<void>;
//# sourceMappingURL=browser-actions.d.ts.map
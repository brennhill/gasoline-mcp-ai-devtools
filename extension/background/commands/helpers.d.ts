import type { PendingQuery } from '../../types';
import type { SyncClient } from '../sync-client';
/** Callback signature for sending async command results back through /sync */
export type SendAsyncResultFn = (syncClient: SyncClient, queryId: string, correlationId: string, status: 'complete' | 'error' | 'timeout' | 'cancelled', result?: unknown, error?: string) => void;
/** Callback signature for showing visual action toasts */
export type ActionToastFn = (tabId: number, text: string, detail?: string, state?: 'trying' | 'success' | 'warning' | 'error', durationMs?: number) => void;
export type QueryParamsObject = Record<string, unknown>;
type TargetResolutionSource = 'explicit_tab' | 'tracked_tab' | 'active_tab';
export interface TargetResolution {
    tabId: number;
    url: string;
    source: TargetResolutionSource;
    requestedTabId?: number;
    trackedTabId?: number | null;
    useActiveTab: boolean;
}
interface TargetResolutionError {
    payload: Record<string, unknown>;
    message: string;
}
/** Send a query result back through /sync */
export declare function sendResult(syncClient: SyncClient, queryId: string, result: unknown): void;
/** Send an async command result back through /sync */
export declare function sendAsyncResult(syncClient: SyncClient, queryId: string, correlationId: string, status: 'complete' | 'error' | 'timeout' | 'cancelled', result?: unknown, error?: string): void;
/** Show a visual action toast on the tracked tab */
export declare function actionToast(tabId: number, action: string, detail?: string, state?: 'trying' | 'success' | 'warning' | 'error', durationMs?: number): void;
export declare function parseQueryParamsObject(params: PendingQuery['params']): QueryParamsObject;
export declare function withTargetContext(result: unknown, target: TargetResolution): Record<string, unknown>;
export declare function requiresTargetTab(queryType: string): boolean;
export declare function resolveTargetTab(query: PendingQuery, paramsObj: QueryParamsObject): Promise<{
    target?: TargetResolution;
    error?: TargetResolutionError;
}>;
/**
 * Check if a URL is restricted — content scripts cannot run on these pages.
 * Covers internal browser pages and known CSP-restricted origins.
 */
export declare function isRestrictedUrl(url: string | undefined): boolean;
export {};
//# sourceMappingURL=helpers.d.ts.map
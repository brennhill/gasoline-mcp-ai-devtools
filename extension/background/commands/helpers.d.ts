/**
 * Purpose: Shared infrastructure for command dispatch -- result helpers, target tab resolution, action toast, and type aliases.
 */
import type { PendingQuery } from '../../types/index.js';
import type { SyncClient } from '../sync-client.js';
export { type QueryParamsObject, type TargetResolution, withTargetContext, requiresTargetTab, isBrowserEscapeAction, persistTrackedTab, resolveTargetTab, isRestrictedUrl } from './target-resolution.js';
import type { QueryParamsObject } from './target-resolution.js';
/** Callback signature for sending async command results back through /sync */
export type SendAsyncResultFn = (syncClient: SyncClient, queryId: string, correlationId: string, status: 'complete' | 'error' | 'timeout' | 'cancelled', result?: unknown, error?: string) => void;
/** Callback signature for showing visual action toasts */
export type ActionToastFn = (tabId: number, text: string, detail?: string, state?: 'trying' | 'success' | 'warning' | 'error', durationMs?: number) => void;
export declare function debugLog(category: string, message: string, data?: unknown): void;
/** Send a query result back through /sync */
export declare function sendResult(syncClient: SyncClient, queryId: string, result: unknown): void;
/** Send an async command result back through /sync */
export declare function sendAsyncResult(syncClient: SyncClient, queryId: string, correlationId: string, status: 'complete' | 'error' | 'timeout' | 'cancelled', result?: unknown, error?: string): void;
/** Show a visual action toast on the tracked tab */
export declare function actionToast(tabId: number, action: string, detail?: string, state?: 'trying' | 'success' | 'warning' | 'error', durationMs?: number): void;
export declare function parseQueryParamsObject(params: PendingQuery['params']): QueryParamsObject;
/** Check if an error indicates the content script is not loaded on the target page. */
export declare function isContentScriptUnreachableError(err: unknown): boolean;
/**
 * Minimal context shape needed by requireAiWebPilot.
 * Avoids circular import with registry.ts (which defines CommandContext).
 */
interface AiWebPilotGuardContext {
    sendResult: (result: unknown) => void;
}
/**
 * Guard that checks AI Web Pilot is enabled.
 * Returns true if enabled and the caller should proceed.
 * Returns false if disabled — the error response has already been sent.
 */
export declare function requireAiWebPilot(ctx: AiWebPilotGuardContext): boolean;
//# sourceMappingURL=helpers.d.ts.map
/**
 * Purpose: Dispatches hardware-level input via Chrome DevTools Protocol.
 * Why: Synthetic DOM events have isTrusted:false which anti-bot systems and complex SPAs ignore.
 *      CDP Input.dispatch* commands produce true hardware events indistinguishable from real user input.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { PendingQuery } from '../types/queries';
import type { SyncClient } from './sync-client';
import type { DOMActionParams, DOMResult } from './dom-types';
type SendAsyncResult = (syncClient: SyncClient, queryId: string, correlationId: string, status: 'complete' | 'error' | 'timeout', result?: unknown, error?: string) => void;
type ActionToast = (tabId: number, text: string, detail?: string, state?: 'trying' | 'success' | 'warning' | 'error', durationMs?: number) => void;
/** Check whether an action should attempt CDP before DOM primitives. */
export declare function isCDPEscalatable(action: string): boolean;
/**
 * Attempt CDP-first execution for click/type/key_press.
 * Returns a DOMResult on success, or null to signal fallback to DOM primitives.
 * Any error is caught internally — callers just check for null.
 */
export declare function tryCDPEscalation(tabId: number, action: string, params: DOMActionParams): Promise<DOMResult | null>;
export declare function executeCDPAction(query: PendingQuery, tabId: number, syncClient: SyncClient, sendAsyncResult: SendAsyncResult, actionToast: ActionToast): Promise<void>;
export {};
//# sourceMappingURL=cdp-dispatch.d.ts.map
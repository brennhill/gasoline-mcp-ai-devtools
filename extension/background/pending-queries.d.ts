/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Pending Query Handlers
 * Handles all query types from the server: DOM, accessibility, browser actions,
 * execute commands, and state management.
 *
 * All results are returned via syncClient.queueCommandResult() which routes them
 * through the unified /sync endpoint. No direct HTTP POSTs to legacy endpoints.
 *
 * Split into modules:
 * - query-execution.ts: JS execution with world-aware routing and CSP fallback
 * - browser-actions.ts: Browser navigation/action handlers with async timeout support
 */
import type { PendingQuery } from '../types';
import type { SyncClient } from './sync-client';
/** Callback signature for sending async command results back through /sync */
export type SendAsyncResultFn = (syncClient: SyncClient, queryId: string, correlationId: string, status: 'complete' | 'error' | 'timeout', result?: unknown, error?: string) => void;
/** Callback signature for showing visual action toasts */
export type ActionToastFn = (tabId: number, text: string, detail?: string, state?: 'trying' | 'success' | 'warning' | 'error', durationMs?: number) => void;
export declare function handlePendingQuery(query: PendingQuery, syncClient: SyncClient): Promise<void>;
export declare function handlePilotCommand(command: string, params: unknown): Promise<unknown>;
//# sourceMappingURL=pending-queries.d.ts.map
/**
 * @fileoverview Pending Query Handlers
 * Handles all query types from the server: DOM, accessibility, browser actions,
 * execute commands, and state management.
 *
 * All results are returned via syncClient.queueCommandResult() which routes them
 * through the unified /sync endpoint. No direct HTTP POSTs to legacy endpoints.
 */
import type { PendingQuery } from '../types';
import type { SyncClient } from './sync-client';
export declare function handlePendingQuery(query: PendingQuery, syncClient: SyncClient): Promise<void>;
export declare function handlePilotCommand(command: string, params: unknown): Promise<unknown>;
//# sourceMappingURL=pending-queries.d.ts.map
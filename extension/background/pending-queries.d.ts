/**
 * @fileoverview Pending Query Handlers
 * Handles all query types from the server: DOM, accessibility, browser actions,
 * execute commands, and state management.
 */
import type { PendingQuery } from '../types';
export declare function handlePendingQuery(query: PendingQuery): Promise<void>;
export declare function handlePilotCommand(command: string, params: unknown): Promise<unknown>;
//# sourceMappingURL=pending-queries.d.ts.map
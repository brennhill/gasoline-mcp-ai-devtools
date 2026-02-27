/**
 * Purpose: Handles extension background coordination and message routing.
 * Why: Centralizes extension coordination to reduce race conditions and split-brain state.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
import type { PendingQuery } from '../types';
import type { SyncClient } from './sync-client';
import './commands/observe';
import './commands/analyze';
import './commands/analyze-navigation';
import './commands/interact';
import './commands/interact-explore';
export type { SendAsyncResultFn, ActionToastFn } from './commands/helpers';
export { handlePilotCommand } from './commands/interact';
export declare function handlePendingQuery(query: PendingQuery, syncClient: SyncClient): Promise<void>;
//# sourceMappingURL=pending-queries.d.ts.map
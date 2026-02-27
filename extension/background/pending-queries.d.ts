/**
 * Purpose: Handles extension background coordination and message routing.
 * Why: Centralizes extension coordination to reduce race conditions and split-brain state.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
import type { PendingQuery } from '../types/index.js';
import type { SyncClient } from './sync-client.js';
import './commands/observe.js';
import './commands/analyze.js';
import './commands/analyze-navigation.js';
import './commands/analyze-page-structure.js';
import './commands/interact.js';
import './commands/interact-content.js';
import './commands/interact-explore.js';
export type { SendAsyncResultFn, ActionToastFn } from './commands/helpers.js';
export { handlePilotCommand } from './commands/interact.js';
export declare function handlePendingQuery(query: PendingQuery, syncClient: SyncClient): Promise<void>;
//# sourceMappingURL=pending-queries.d.ts.map
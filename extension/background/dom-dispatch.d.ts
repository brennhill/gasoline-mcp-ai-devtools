/**
 * Purpose: Handles extension background coordination and message routing.
 * Why: Centralizes extension coordination to reduce race conditions and split-brain state.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
import type { PendingQuery } from '../types/queries.js';
import type { SyncClient } from './sync-client.js';
import type { SendAsyncResultFn, ActionToastFn } from './commands/helpers.js';
export declare function executeDOMAction(query: PendingQuery, tabId: number, syncClient: SyncClient, sendAsyncResult: SendAsyncResultFn, actionToast: ActionToastFn): Promise<void>;
//# sourceMappingURL=dom-dispatch.d.ts.map
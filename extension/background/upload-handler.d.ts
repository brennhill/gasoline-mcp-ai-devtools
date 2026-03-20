/**
 * Purpose: Handles file upload queries by fetching file data from the Go server and injecting it into DOM file inputs via DataTransfer or OS automation escalation.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { PendingQuery } from '../types/queries.js';
import type { SyncClient } from './sync-client.js';
import type { SendAsyncResultFn, ActionToastFn } from './pending-queries.js';
export declare function executeUpload(query: PendingQuery, tabId: number, syncClient: SyncClient, sendAsyncResult: SendAsyncResultFn, actionToast: ActionToastFn): Promise<void>;
//# sourceMappingURL=upload-handler.d.ts.map
import type { PendingQuery } from '../types';
import type { SyncClient } from './sync-client';
import './commands/observe';
import './commands/analyze';
import './commands/interact';
export type { SendAsyncResultFn, ActionToastFn } from './commands/helpers';
export { handlePilotCommand } from './commands/interact';
export declare function handlePendingQuery(query: PendingQuery, syncClient: SyncClient): Promise<void>;
//# sourceMappingURL=pending-queries.d.ts.map
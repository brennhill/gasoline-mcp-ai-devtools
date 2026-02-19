import type { PendingQuery } from '../../types';
import type { SyncClient } from '../sync-client';
import type { SendAsyncResultFn, QueryParamsObject, TargetResolution } from './helpers';
import { actionToast } from './helpers';
export interface CommandContext {
    query: PendingQuery;
    syncClient: SyncClient;
    tabId: number;
    params: QueryParamsObject;
    target: TargetResolution | undefined;
    /** Send a sync result, wrapped with target context */
    sendResult: (result: unknown) => void;
    /** Send an async result, wrapped with target context */
    sendAsyncResult: SendAsyncResultFn;
    /** Show action toast on the target tab */
    actionToast: typeof actionToast;
}
export type CommandHandler = (ctx: CommandContext) => Promise<void>;
export declare function registerCommand(type: string, handler: CommandHandler): void;
export declare function dispatch(query: PendingQuery, syncClient: SyncClient): Promise<void>;
//# sourceMappingURL=registry.d.ts.map
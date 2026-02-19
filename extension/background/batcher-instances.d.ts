import type { LogEntry, WebSocketEvent, NetworkBodyPayload, EnhancedAction, PerformanceSnapshot } from '../types';
import * as communication from './communication';
/** Mutable connection status passed in from the state owner */
export interface ConnectionStatusRef {
    connected: boolean;
    entries: number;
    maxEntries: number;
    errorCount: number;
    logFile: string;
    logFileSize?: number;
    serverVersion?: string;
    extensionVersion?: string;
    versionMismatch?: boolean;
}
type DebugLogFn = (category: string, message: string, data?: unknown) => void;
/** Dependencies injected by index.ts to avoid circular imports */
export interface BatcherDeps {
    getServerUrl: () => string;
    getConnectionStatus: () => ConnectionStatusRef;
    setConnectionStatus: (patch: Partial<ConnectionStatusRef>) => void;
    debugLog: DebugLogFn;
}
export interface BatcherInstances {
    logBatcherWithCB: communication.BatcherWithCircuitBreaker<LogEntry>;
    logBatcher: communication.Batcher<LogEntry>;
    wsBatcherWithCB: communication.BatcherWithCircuitBreaker<WebSocketEvent>;
    wsBatcher: communication.Batcher<WebSocketEvent>;
    enhancedActionBatcherWithCB: communication.BatcherWithCircuitBreaker<EnhancedAction>;
    enhancedActionBatcher: communication.Batcher<EnhancedAction>;
    networkBodyBatcherWithCB: communication.BatcherWithCircuitBreaker<NetworkBodyPayload>;
    networkBodyBatcher: communication.Batcher<NetworkBodyPayload>;
    perfBatcherWithCB: communication.BatcherWithCircuitBreaker<PerformanceSnapshot>;
    perfBatcher: communication.Batcher<PerformanceSnapshot>;
}
/**
 * Create all batcher instances wired to the shared circuit breaker.
 * Called once from index.ts during module initialization.
 */
export declare function createBatcherInstances(deps: BatcherDeps, sharedCircuitBreaker: communication.CircuitBreaker): BatcherInstances;
export {};
//# sourceMappingURL=batcher-instances.d.ts.map
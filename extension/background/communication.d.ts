/**
 * @fileoverview Communication - Handles server communication, circuit breaker,
 * batching, and all HTTP interactions with the Gasoline server.
 */
import type { LogEntry, WebSocketEvent, NetworkBodyPayload, EnhancedAction, PerformanceSnapshot, WaterfallEntry, ConnectionStatus, CircuitBreakerState, CircuitBreakerStats, MemoryPressureState } from '../types';
/** Rate limit configuration */
export declare const RATE_LIMIT_CONFIG: {
    maxFailures: number;
    resetTimeout: number;
    backoffSchedule: readonly number[];
    retryBudget: number;
};
/** Circuit breaker options */
interface CircuitBreakerOptions {
    maxFailures?: number;
    resetTimeout?: number;
    initialBackoff?: number;
    maxBackoff?: number;
}
/** Circuit breaker instance */
interface CircuitBreaker {
    execute: <T>(args: unknown) => Promise<T>;
    getState: () => CircuitBreakerState;
    getStats: () => CircuitBreakerStats;
    reset: () => void;
    recordFailure: () => void;
}
/** Batcher instance */
interface Batcher<T> {
    add: (entry: T) => void;
    flush: () => Promise<void> | void;
    clear: () => void;
    getPending?: () => T[];
}
/** Batcher with circuit breaker result */
interface BatcherWithCircuitBreaker<T> {
    batcher: Batcher<T>;
    circuitBreaker: {
        getState: () => CircuitBreakerState;
        getStats: () => CircuitBreakerStats;
        reset: () => void;
    };
    getConnectionStatus: () => {
        connected: boolean;
    };
}
/** Server health response */
interface ServerHealthResponse {
    connected: boolean;
    error?: string;
    version?: string;
    logs?: {
        logFile?: string;
        logFileSize?: number;
        entries?: number;
        maxEntries?: number;
    };
}
/** Batcher configuration options */
interface BatcherConfig {
    debounceMs?: number;
    maxBatchSize?: number;
    retryBudget?: number;
    maxFailures?: number;
    resetTimeout?: number;
    sharedCircuitBreaker?: CircuitBreaker;
}
/** Log batcher options */
interface LogBatcherOptions {
    debounceMs?: number;
    maxBatchSize?: number;
    memoryPressureGetter?: () => MemoryPressureState;
}
/**
 * Circuit breaker with exponential backoff for server communication.
 * Prevents the extension from hammering a down/slow server.
 */
export declare function createCircuitBreaker(sendFn: (args: unknown) => Promise<unknown>, options?: CircuitBreakerOptions): CircuitBreaker;
/**
 * Creates a batcher wired with circuit breaker logic for rate limiting.
 */
export declare function createBatcherWithCircuitBreaker<T>(sendFn: (entries: T[]) => Promise<unknown>, options?: BatcherConfig): BatcherWithCircuitBreaker<T>;
/**
 * Create a simple log batcher without circuit breaker
 */
export declare function createLogBatcher<T>(flushFn: (entries: T[]) => void, options?: LogBatcherOptions): Batcher<T>;
/**
 * Send log entries to the server
 */
export declare function sendLogsToServer(serverUrl: string, entries: LogEntry[], debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<{
    entries: number;
}>;
/**
 * Send WebSocket events to the server
 */
export declare function sendWSEventsToServer(serverUrl: string, events: WebSocketEvent[], debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/**
 * Send network bodies to the server
 */
export declare function sendNetworkBodiesToServer(serverUrl: string, bodies: NetworkBodyPayload[], debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/**
 * Send network waterfall data to server
 */
export declare function sendNetworkWaterfallToServer(serverUrl: string, payload: {
    entries: WaterfallEntry[];
    pageURL: string;
}, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/**
 * Send enhanced actions to server
 */
export declare function sendEnhancedActionsToServer(serverUrl: string, actions: EnhancedAction[], debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/**
 * Send performance snapshots to server
 */
export declare function sendPerformanceSnapshotsToServer(serverUrl: string, snapshots: PerformanceSnapshot[], debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/**
 * Check server health
 */
export declare function checkServerHealth(serverUrl: string): Promise<ServerHealthResponse>;
/**
 * Update extension badge
 */
export declare function updateBadge(status: ConnectionStatus): void;
/**
 * Post query results back to the server
 */
export declare function postQueryResult(serverUrl: string, queryId: string, type: string, result: unknown): Promise<void>;
/**
 * POST async command result to server using correlation_id
 */
export declare function postAsyncCommandResult(serverUrl: string, correlationId: string, status: 'pending' | 'complete' | 'timeout', result?: unknown, error?: string | null, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/**
 * Post extension settings to server
 */
export declare function postSettings(serverUrl: string, sessionId: string, settings: Record<string, boolean | string>, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/**
 * Poll the server's /settings endpoint for AI capture overrides
 */
export declare function pollCaptureSettings(serverUrl: string): Promise<Record<string, string> | null>;
/**
 * Post extension logs to server
 */
export declare function postExtensionLogs(serverUrl: string, logs: Array<{
    timestamp: string;
    level: string;
    message: string;
    source: string;
    category: string;
    data?: unknown;
}>): Promise<void>;
/**
 * Send status ping to server
 */
export declare function sendStatusPing(serverUrl: string, statusMessage: {
    type: string;
    tracking_enabled: boolean;
    tracked_tab_id: number | null;
    tracked_tab_url: string | null;
    message: string;
    extension_connected: boolean;
    timestamp: string;
}, diagnosticLogFn?: (message: string) => void): Promise<void>;
/**
 * Poll server for pending queries
 */
export declare function pollPendingQueries(serverUrl: string, sessionId: string, pilotState: '0' | '1', diagnosticLogFn?: (message: string) => void, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<Array<{
    id: string;
    type: string;
    params: string | Record<string, unknown>;
    correlation_id?: string;
}>>;
/**
 * Format a log entry with timestamp and truncation
 */
export declare function formatLogEntry(entry: LogEntry): LogEntry;
/**
 * Determine if a log should be captured based on level filter
 */
export declare function shouldCaptureLog(logLevel: string, filterLevel: string, logType?: string): boolean;
/**
 * Capture a screenshot of the visible tab area
 */
export declare function captureScreenshot(tabId: number, serverUrl: string, relatedErrorId: string | null, errorType: string | null, canTakeScreenshotFn: (tabId: number) => {
    allowed: boolean;
    reason?: string;
    nextAllowedIn?: number | null;
}, recordScreenshotFn: (tabId: number) => void, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<{
    success: boolean;
    entry?: LogEntry;
    error?: string;
    nextAllowedIn?: number | null;
}>;
export {};
//# sourceMappingURL=communication.d.ts.map
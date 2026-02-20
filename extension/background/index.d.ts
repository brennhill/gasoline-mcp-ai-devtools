/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Main Background Service Worker — Business logic and export hub.
 * Mutable state lives in state.ts; this module owns debug logging, log handling,
 * connection management, and batcher wiring. Delegates batcher instance creation
 * to batcher-instances.ts and sync client lifecycle to sync-manager.ts.
 */
import type { LogEntry, ChromeMessageSender } from '../types';
import { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot } from './message-handlers';
import { handlePendingQuery as handlePendingQueryImpl, handlePilotCommand as handlePilotCommandImpl } from './pending-queries';
export { DEFAULT_SERVER_URL } from '../lib/constants';
export { DebugCategory } from './debug';
/**
 * Log a diagnostic message only when debug mode is enabled
 */
export declare function diagnosticLog(message: string): void;
/**
 * Log a debug message (only when debug mode is enabled)
 */
export declare function debugLog(category: string, message: string, data?: unknown): void;
/**
 * Get all debug log entries
 */
export declare function getDebugLog(): import("../types").DebugLogEntry[];
/**
 * Clear debug log buffer
 */
export declare function clearDebugLog(): void;
/**
 * Export debug log as JSON string
 */
export declare function exportDebugLog(): string;
/**
 * Set debug mode enabled/disabled
 */
export declare function setDebugMode(enabled: boolean): void;
export declare const sharedServerCircuitBreaker: import("./circuit-breaker").CircuitBreaker;
export declare const logBatcherWithCB: import("./batchers").BatcherWithCircuitBreaker<LogEntry>;
export declare const logBatcher: import("./batchers").Batcher<LogEntry>;
export declare const wsBatcherWithCB: import("./batchers").BatcherWithCircuitBreaker<import("../types").WebSocketEvent>;
export declare const wsBatcher: import("./batchers").Batcher<import("../types").WebSocketEvent>;
export declare const enhancedActionBatcherWithCB: import("./batchers").BatcherWithCircuitBreaker<import("../types").EnhancedAction>;
export declare const enhancedActionBatcher: import("./batchers").Batcher<import("../types").EnhancedAction>;
export declare const networkBodyBatcherWithCB: import("./batchers").BatcherWithCircuitBreaker<import("../types").NetworkBodyPayload>;
export declare const networkBodyBatcher: import("./batchers").Batcher<import("../types").NetworkBodyPayload>;
export declare const perfBatcherWithCB: import("./batchers").BatcherWithCircuitBreaker<import("../types").PerformanceSnapshot>;
export declare const perfBatcher: import("./batchers").Batcher<import("../types").PerformanceSnapshot>;
export declare function handleLogMessage(payload: LogEntry, sender: ChromeMessageSender, tabId?: number): Promise<void>;
export declare function handleClearLogs(): Promise<{
    success: boolean;
    error?: string;
}>;
/**
 * Check if a connection check is currently running (for testing)
 */
export declare function isConnectionCheckRunning(): boolean;
export declare function checkConnectionAndUpdate(): Promise<void>;
export declare function sendStatusPingWrapper(): Promise<void>;
/**
 * Reset sync client connection (call when user enables pilot/tracking)
 */
export declare function resetSyncClientConnection(): void;
export declare const handlePendingQuery: typeof handlePendingQueryImpl;
export declare const handlePilotCommand: typeof handlePilotCommandImpl;
export { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot };
//# sourceMappingURL=index.d.ts.map
/**
 * @fileoverview Server Communication - HTTP functions for sending data to
 * the Gasoline server.
 */
import type { LogEntry, WebSocketEvent, NetworkBodyPayload, EnhancedAction, PerformanceSnapshot, ConnectionStatus, WaterfallEntry } from '../types';
/**
 * Server health response
 */
export interface ServerHealthResponse {
    connected: boolean;
    error?: string;
    version?: string;
    availableVersion?: string;
    logs?: {
        logFile?: string;
        logFileSize?: number;
        entries?: number;
        maxEntries?: number;
    };
}
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
//# sourceMappingURL=server.d.ts.map
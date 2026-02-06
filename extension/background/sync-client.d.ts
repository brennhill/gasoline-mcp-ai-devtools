/**
 * @fileoverview Unified Sync Client - Replaces multiple polling loops with single /sync endpoint.
 * Features: Simple exponential backoff, binary connection state, self-healing for MV3.
 */
/** Settings to send to server */
export interface SyncSettings {
    pilot_enabled: boolean;
    tracking_enabled: boolean;
    tracked_tab_id: number;
    tracked_tab_url: string;
    tracked_tab_title: string;
    capture_logs: boolean;
    capture_network: boolean;
    capture_websocket: boolean;
    capture_actions: boolean;
}
/** Extension log entry */
export interface SyncExtensionLog {
    timestamp: string;
    level: string;
    message: string;
    source: string;
    category: string;
    data?: unknown;
}
/** Command result to send to server */
export interface SyncCommandResult {
    id: string;
    correlation_id?: string;
    status: 'complete' | 'error' | 'timeout';
    result?: unknown;
    error?: string;
}
/** Command from server */
export interface SyncCommand {
    id: string;
    type: string;
    params: unknown;
    correlation_id?: string;
}
/** Sync state */
export interface SyncState {
    connected: boolean;
    lastSyncAt: number;
    consecutiveFailures: number;
    lastCommandAck: string | null;
}
/** Callbacks for sync client */
export interface SyncClientCallbacks {
    onCommand: (command: SyncCommand) => Promise<void>;
    onConnectionChange: (connected: boolean) => void;
    onCaptureOverrides?: (overrides: Record<string, string>) => void;
    onVersionMismatch?: (extensionVersion: string, serverVersion: string) => void;
    getSettings: () => Promise<SyncSettings>;
    getExtensionLogs: () => SyncExtensionLog[];
    clearExtensionLogs: () => void;
    debugLog?: (category: string, message: string, data?: unknown) => void;
}
export declare class SyncClient {
    private serverUrl;
    private sessionId;
    private callbacks;
    private state;
    private intervalId;
    private running;
    private syncing;
    private flushRequested;
    private pendingResults;
    private extensionVersion;
    constructor(serverUrl: string, sessionId: string, callbacks: SyncClientCallbacks, extensionVersion?: string);
    /** Get current sync state */
    getState(): SyncState;
    /** Check if connected */
    isConnected(): boolean;
    /** Start the sync loop */
    start(): void;
    /** Stop the sync loop */
    stop(): void;
    /** Queue a command result to send on next sync, then flush immediately */
    queueCommandResult(result: SyncCommandResult): void;
    /** Trigger an immediate sync to deliver queued results with minimal latency */
    flush(): void;
    /** Reset connection state (e.g., when user toggles pilot/tracking) */
    resetConnection(): void;
    /** Update server URL */
    setServerUrl(url: string): void;
    private scheduleNextSync;
    private doSync;
    private onSuccess;
    private onFailure;
    private log;
}
/**
 * Create a sync client instance
 */
export declare function createSyncClient(serverUrl: string, sessionId: string, callbacks: SyncClientCallbacks, extensionVersion?: string): SyncClient;
//# sourceMappingURL=sync-client.d.ts.map
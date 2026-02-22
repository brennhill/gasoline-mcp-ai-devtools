/**
 * @fileoverview Mutable module-level state for the background service worker.
 * Owns all `let` variables and their setter functions so that state ownership
 * is explicit and separated from business logic in index.ts.
 */
/** Session ID for detecting extension reloads */
export declare const EXTENSION_SESSION_ID: string;
/** Connection status (mutable internal state) */
export interface MutableConnectionStatus {
    connected: boolean;
    entries: number;
    maxEntries: number;
    errorCount: number;
    logFile: string;
    logFileSize?: number;
    serverVersion?: string;
    extensionVersion?: string;
    versionMismatch?: boolean;
    securityMode?: 'normal' | 'insecure_proxy';
    productionParity?: boolean;
    insecureRewritesApplied?: string[];
}
export interface ExtensionLogQueueEntry {
    timestamp: string;
    level: string;
    message: string;
    source: string;
    category: string;
    data?: unknown;
}
/**
 * Compatibility mirrors for legacy imports.
 * New code should prefer getters/setters below.
 */
export declare let serverUrl: string;
export declare let debugMode: boolean;
export declare let connectionStatus: MutableConnectionStatus;
export declare let currentLogLevel: string;
export declare let screenshotOnError: boolean;
export declare let _captureOverrides: Record<string, string>;
export declare let aiControlled: boolean;
export declare let _connectionCheckRunning: boolean;
export declare let __aiWebPilotEnabledCache: boolean;
export declare let __aiWebPilotCacheInitialized: boolean;
export declare let __pilotInitCallback: (() => void) | null;
export declare const extensionLogQueue: ExtensionLogQueueEntry[];
export declare function getServerUrl(): string;
export declare function isDebugMode(): boolean;
export declare function getConnectionStatus(): MutableConnectionStatus;
export declare function getCurrentLogLevel(): string;
export declare function isScreenshotOnError(): boolean;
export declare function getCaptureOverrides(): Record<string, string>;
export declare function isAiControlled(): boolean;
export declare function isConnectionCheckRunning(): boolean;
export declare function isAiWebPilotCacheInitialized(): boolean;
export declare function getPilotInitCallback(): (() => void) | null;
export declare function getExtensionLogQueue(): ExtensionLogQueueEntry[];
export declare function clearExtensionLogQueue(): void;
export declare function pushExtensionLog(entry: ExtensionLogQueueEntry): void;
export declare function capExtensionLogs(maxEntries: number): void;
export declare const initReady: Promise<void>;
export declare function markInitComplete(): void;
export declare function setServerUrl(url: string): void;
/** Low-level flag setter. Use index.setDebugMode for the version that also logs. */
export declare function _setDebugModeRaw(enabled: boolean): void;
export declare function setCurrentLogLevel(level: string): void;
export declare function setScreenshotOnError(enabled: boolean): void;
export declare function setConnectionStatus(patch: Partial<MutableConnectionStatus>): void;
export declare function setConnectionCheckRunning(running: boolean): void;
export declare function setAiWebPilotEnabledCache(enabled: boolean): void;
export declare function setAiWebPilotCacheInitialized(initialized: boolean): void;
export declare function setPilotInitCallback(callback: (() => void) | null): void;
export declare function applyCaptureOverrides(overrides: Record<string, string>): void;
/**
 * Reset pilot cache for testing
 */
export declare function _resetPilotCacheForTesting(value?: boolean): void;
/**
 * Check if AI Web Pilot is enabled
 */
export declare function isAiWebPilotEnabled(): boolean;
export declare function resetStateForTesting(): void;
//# sourceMappingURL=state.d.ts.map
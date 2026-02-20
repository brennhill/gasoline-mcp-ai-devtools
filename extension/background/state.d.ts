/**
 * @fileoverview Mutable module-level state for the background service worker.
 * Owns all `let` variables and their setter functions so that state ownership
 * is explicit and separated from business logic in index.ts.
 */
/** Session ID for detecting extension reloads */
export declare const EXTENSION_SESSION_ID: string;
/** Server URL */
export declare let serverUrl: string;
/** Debug mode flag */
export declare let debugMode: boolean;
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
}
export declare let connectionStatus: MutableConnectionStatus;
/** Log level filter */
export declare let currentLogLevel: string;
/** Screenshot settings */
export declare let screenshotOnError: boolean;
/** AI capture control state */
export declare let _captureOverrides: Record<string, string>;
export declare let aiControlled: boolean;
/** Connection check mutex */
export declare let _connectionCheckRunning: boolean;
/** AI Web Pilot state */
export declare let __aiWebPilotEnabledCache: boolean;
export declare let __aiWebPilotCacheInitialized: boolean;
export declare let __pilotInitCallback: (() => void) | null;
export declare const initReady: Promise<void>;
export declare function markInitComplete(): void;
/** Extension log queue for server posting */
export declare const extensionLogQueue: Array<{
    timestamp: string;
    level: string;
    message: string;
    source: string;
    category: string;
    data?: unknown;
}>;
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
//# sourceMappingURL=state.d.ts.map
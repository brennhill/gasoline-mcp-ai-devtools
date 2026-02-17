/**
 * Purpose: Provides shared runtime utilities used by extension and server workflows.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import type { WebSocketCaptureMode } from '../types/index';
type WebSocketMessageData = string | ArrayBuffer | Blob;
interface SizedObject {
    size: number;
}
interface ConnectionStats {
    incoming: {
        count: number;
        bytes: number;
        lastPreview: string | null;
        lastAt: number | null;
    };
    outgoing: {
        count: number;
        bytes: number;
        lastPreview: string | null;
        lastAt: number | null;
    };
}
type MessageDirection = 'incoming' | 'outgoing';
interface TruncationResult {
    data: string;
    truncated: boolean;
}
interface SamplingInfo {
    rate: string;
    logged: string;
    window: string;
}
interface SchemaInfo {
    detectedKeys: string[] | null;
    consistent: boolean;
    variants?: string[];
}
export interface ConnectionTracker {
    id: string;
    url: string;
    messageCount: number;
    _sampleCounter: number;
    _messageRate: number;
    _messageTimestamps: number[];
    _schemaKeys: string[];
    _schemaVariants: Map<string, number>;
    _schemaConsistent: boolean;
    _schemaDetected: boolean;
    stats: ConnectionStats;
    recordMessage(direction: MessageDirection, data: WebSocketMessageData | null): void;
    shouldSample(direction: MessageDirection): boolean;
    shouldLogLifecycle(): boolean;
    getSamplingInfo(): SamplingInfo;
    getMessageRate(): number;
    setMessageRate(rate: number): void;
    getSchema(): SchemaInfo;
    isSchemaChange(data: string | null): boolean;
}
/**
 * Get the byte size of a WebSocket message
 */
export declare function getSize(data: WebSocketMessageData | SizedObject | null): number;
/**
 * Format a WebSocket payload for logging
 */
export declare function formatPayload(data: WebSocketMessageData | null): string;
/**
 * Truncate a WebSocket message to the size limit
 */
export declare function truncateWsMessage(message: string): TruncationResult;
/**
 * Create a connection tracker for adaptive sampling and schema detection
 */
export declare function createConnectionTracker(id: string, url: string): ConnectionTracker;
/**
 * Install WebSocket capture by wrapping the WebSocket constructor.
 * If the early-patch script ran first (world: "MAIN", document_start),
 * uses the saved original constructor and adopts buffered connections.
 */
export declare function installWebSocketCapture(): void;
/**
 * Set the WebSocket capture mode
 */
export declare function setWebSocketCaptureMode(mode: WebSocketCaptureMode): void;
/**
 * Set WebSocket capture enabled state
 */
export declare function setWebSocketCaptureEnabled(enabled: boolean): void;
/**
 * Get the current WebSocket capture mode
 */
export declare function getWebSocketCaptureMode(): WebSocketCaptureMode;
/**
 * Uninstall WebSocket capture, restoring the original constructor
 */
export declare function uninstallWebSocketCapture(): void;
/**
 * Reset all module state for testing purposes
 * Restores original WebSocket if installed, resets capture settings to defaults.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
export declare function resetForTesting(): void;
export {};
//# sourceMappingURL=websocket.d.ts.map
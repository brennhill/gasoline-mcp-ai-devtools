/**
 * Purpose: Implements popup settings controls for websocket capture mode and safe log clearing actions.
 * Why: Keeps destructive and behavior-changing popup operations centralized with explicit UX safeguards.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
/**
 * @fileoverview Settings Module
 * Handles log level, WebSocket mode, and clear logs functionality
 */
import type { WebSocketCaptureMode } from '../types';
/**
 * Handle WebSocket mode change
 */
export declare function handleWebSocketModeChange(mode: WebSocketCaptureMode): void;
/**
 * Initialize the WebSocket mode selector
 */
export declare function initWebSocketModeSelector(): Promise<void>;
/**
 * Reset clear confirmation state (exported for testing)
 */
export declare function resetClearConfirm(): void;
/**
 * Handle clear logs button click (with confirmation)
 */
export declare function handleClearLogs(): Promise<{
    success?: boolean;
    error?: string;
} | null>;
//# sourceMappingURL=settings.d.ts.map
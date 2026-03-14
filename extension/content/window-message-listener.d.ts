/**
 * Purpose: Listens for window.postMessage events from inject.js, resolves pending request promises, and forwards telemetry to the background via chrome.runtime.sendMessage.
 * Why: Consolidates message forwarding and message listening into one module since they share the same data flow.
 * Docs: docs/features/feature/observe/index.md
 */
import type { BackgroundMessageFromContent } from './types.js';
/** Dispatch table: page postMessage type -> background message type */
export declare const MESSAGE_MAP: Record<string, string>;
/**
 * Safely send a message to the background script.
 * Handles extension context invalidation gracefully.
 */
export declare function safeSendMessage(msg: BackgroundMessageFromContent): void;
/**
 * Check if the extension context is still valid
 */
export declare function isContextValid(): boolean;
export declare function initWindowMessageListener(): void;
//# sourceMappingURL=window-message-listener.d.ts.map
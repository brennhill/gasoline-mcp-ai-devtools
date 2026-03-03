/**
 * Purpose: Background script for push delivery — screenshot push, chat push, capability tracking.
 * Why: Enables browser-to-AI message injection via keyboard shortcuts.
 * Docs: docs/features/feature/browser-push/index.md
 */
/** Per-session push capability state from the daemon. */
export interface PushCapabilities {
    push_enabled: boolean;
    supports_sampling: boolean;
    supports_notifications: boolean;
    client_name: string;
    inbox_count: number;
}
/**
 * Fetch push capabilities from the daemon.
 * Caches for 10s to avoid hammering the endpoint.
 */
export declare function fetchPushCapabilities(): Promise<PushCapabilities | null>;
/** Clear the capabilities cache (e.g., on reconnect). */
export declare function clearPushCapabilitiesCache(): void;
/**
 * Install the push_screenshot keyboard shortcut listener.
 * When Alt+Shift+S is pressed, captures the active tab's screenshot
 * and pushes to the daemon.
 */
export declare function installPushCommandListener(logFn?: (message: string) => void): void;
/**
 * Install the push_chat keyboard shortcut listener.
 * When Alt+Shift+C is pressed, sends a message to the content script
 * to show/toggle the chat widget.
 */
export declare function installChatCommandListener(logFn?: (message: string) => void): void;
/**
 * Push a screenshot to the daemon's push pipeline.
 */
export declare function pushScreenshot(screenshotDataUrl: string, note: string, pageUrl: string, tabId: number): Promise<{
    status: string;
    event_id?: string;
} | null>;
/**
 * Push a chat message to the daemon's push pipeline.
 * If conversationId is provided, the daemon tracks the message for SSE response delivery.
 */
export declare function pushChatMessage(message: string, pageUrl: string, tabId: number, conversationId?: string): Promise<{
    status: string;
    event_id?: string;
    conversation_id?: string;
} | null>;
//# sourceMappingURL=push-handler.d.ts.map
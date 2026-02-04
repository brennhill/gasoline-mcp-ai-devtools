/**
 * @fileoverview Message Forwarding Module
 * Forwards messages between page context and background script
 */
// Dispatch table: page postMessage type -> background message type
export const MESSAGE_MAP = {
    GASOLINE_LOG: 'log',
    GASOLINE_WS: 'ws_event',
    GASOLINE_NETWORK_BODY: 'network_body',
    GASOLINE_ENHANCED_ACTION: 'enhanced_action',
    GASOLINE_PERFORMANCE_SNAPSHOT: 'performance_snapshot',
};
// Track whether the extension context is still valid
let contextValid = true;
/**
 * Safely send a message to the background script
 * Handles extension context invalidation gracefully
 */
export function safeSendMessage(msg) {
    if (!contextValid)
        return;
    try {
        chrome.runtime.sendMessage(msg);
    }
    catch (e) {
        if (e instanceof Error && e.message?.includes('Extension context invalidated')) {
            contextValid = false;
            console.warn('[Gasoline] Please refresh this page. The Gasoline extension was reloaded ' +
                'and this page still has the old content script. A page refresh will ' +
                'reconnect capture automatically.');
        }
    }
}
/**
 * Check if the extension context is still valid
 */
export function isContextValid() {
    return contextValid;
}
//# sourceMappingURL=message-forwarding.js.map
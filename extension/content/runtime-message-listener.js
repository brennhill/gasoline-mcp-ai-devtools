/**
 * @fileoverview Runtime Message Listener Module
 * Handles chrome.runtime messages from background script
 */
import { isValidBackgroundSender, handlePing, handleToggleMessage, forwardHighlightMessage, handleStateCommand, handleExecuteJs, handleExecuteQuery, handleA11yQuery, handleDomQuery, handleGetNetworkWaterfall, } from './message-handlers.js';
/**
 * Initialize runtime message listener
 * Listens for messages from background (feature toggles and pilot commands)
 */
export function initRuntimeMessageListener() {
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        // SECURITY: Validate sender is from the extension background, not from page context
        if (!isValidBackgroundSender(sender)) {
            console.warn('[Gasoline] Rejected message from untrusted sender:', sender.id);
            return false;
        }
        // Handle ping to check if content script is loaded
        if (message.type === 'GASOLINE_PING') {
            return handlePing(sendResponse);
        }
        // Handle toggle messages
        handleToggleMessage(message);
        // Handle GASOLINE_HIGHLIGHT from background
        if (message.type === 'GASOLINE_HIGHLIGHT') {
            forwardHighlightMessage(message)
                .then((result) => {
                sendResponse(result);
            })
                .catch((err) => {
                sendResponse({ success: false, error: err.message });
            });
            return true; // Will respond asynchronously
        }
        // Handle state management commands from background
        if (message.type === 'GASOLINE_MANAGE_STATE') {
            handleStateCommand(message.params)
                .then((result) => sendResponse(result))
                .catch((err) => sendResponse({ error: err.message }));
            return true; // Keep channel open for async response
        }
        // Handle GASOLINE_EXECUTE_JS from background (direct pilot command)
        if (message.type === 'GASOLINE_EXECUTE_JS') {
            const params = message.params || {};
            return handleExecuteJs(params, sendResponse);
        }
        // Handle GASOLINE_EXECUTE_QUERY from background (polling system)
        if (message.type === 'GASOLINE_EXECUTE_QUERY') {
            return handleExecuteQuery(message.params || {}, sendResponse);
        }
        // Handle A11Y_QUERY from background (run accessibility audit in page context)
        if (message.type === 'A11Y_QUERY') {
            return handleA11yQuery(message.params || {}, sendResponse);
        }
        // Handle DOM_QUERY from background (execute CSS selector query in page context)
        if (message.type === 'DOM_QUERY') {
            return handleDomQuery(message.params || {}, sendResponse);
        }
        // Handle GET_NETWORK_WATERFALL from background (collect PerformanceResourceTiming data)
        if (message.type === 'GET_NETWORK_WATERFALL') {
            return handleGetNetworkWaterfall(sendResponse);
        }
        return undefined;
    });
}
//# sourceMappingURL=runtime-message-listener.js.map
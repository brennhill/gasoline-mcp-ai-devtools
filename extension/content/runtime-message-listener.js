/**
 * @fileoverview Runtime Message Listener Module
 * Handles chrome.runtime messages from background script
 */
import { isValidBackgroundSender, handlePing, handleToggleMessage, forwardHighlightMessage, handleStateCommand, handleExecuteJs, handleExecuteQuery, handleA11yQuery, handleDomQuery, handleGetNetworkWaterfall, } from './message-handlers.js';
/**
 * Show a brief visual toast overlay for AI actions (navigate, execute_js, etc.)
 * Injected directly into the page DOM by the content script.
 */
function showActionToast(text, durationMs = 3000) {
    // Remove existing toast
    const existing = document.getElementById('gasoline-action-toast');
    if (existing)
        existing.remove();
    const toast = document.createElement('div');
    toast.id = 'gasoline-action-toast';
    toast.textContent = text;
    Object.assign(toast.style, {
        position: 'fixed',
        top: '16px',
        left: '50%',
        transform: 'translateX(-50%)',
        padding: '10px 24px',
        background: 'linear-gradient(135deg, #ff6b00 0%, #ff9500 100%)',
        color: '#fff',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        fontSize: '14px',
        fontWeight: '600',
        borderRadius: '8px',
        boxShadow: '0 4px 20px rgba(255, 107, 0, 0.4)',
        zIndex: '2147483647',
        pointerEvents: 'none',
        opacity: '0',
        transition: 'opacity 0.2s ease-in',
    });
    const target = document.body || document.documentElement;
    if (!target)
        return;
    target.appendChild(toast);
    // Fade in
    requestAnimationFrame(() => {
        toast.style.opacity = '1';
    });
    // Fade out and remove
    setTimeout(() => {
        toast.style.opacity = '0';
        setTimeout(() => toast.remove(), 300);
    }, durationMs);
}
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
        // Show AI action toast overlay
        if (message.type === 'GASOLINE_ACTION_TOAST') {
            const { text, duration_ms } = message;
            if (text)
                showActionToast(text, duration_ms);
            return false;
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
            // message.params contains action, state, include_url from the manage_state tool
            // handleStateCommand accepts params with optional action (StateAction), name, state, include_url
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
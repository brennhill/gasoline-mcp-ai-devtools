/**
 * @fileoverview Runtime Message Listener Module
 * Handles chrome.runtime messages from background script
 */
import { isValidBackgroundSender, handlePing, handleToggleMessage, forwardHighlightMessage, handleStateCommand, handleExecuteJs, handleExecuteQuery, handleA11yQuery, handleDomQuery, handleGetNetworkWaterfall, } from './message-handlers.js';
/** Color themes for each toast state */
const TOAST_THEMES = {
    trying: { bg: 'linear-gradient(135deg, #ff6b00 0%, #ff9500 100%)', shadow: 'rgba(255, 107, 0, 0.4)' },
    success: { bg: 'linear-gradient(135deg, #22c55e 0%, #16a34a 100%)', shadow: 'rgba(34, 197, 94, 0.4)' },
    warning: { bg: 'linear-gradient(135deg, #f59e0b 0%, #d97706 100%)', shadow: 'rgba(245, 158, 11, 0.4)' },
    error: { bg: 'linear-gradient(135deg, #ef4444 0%, #dc2626 100%)', shadow: 'rgba(239, 68, 68, 0.4)' },
};
/** Truncate text to maxLen characters with ellipsis */
function truncateText(text, maxLen) {
    if (text.length <= maxLen)
        return text;
    return text.slice(0, maxLen - 1) + '\u2026';
}
/**
 * Show a brief visual toast overlay for AI actions.
 * Supports color-coded states and structured content with truncation.
 */
function showActionToast(text, detail, state = 'trying', durationMs = 3000) {
    // Remove existing toast
    const existing = document.getElementById('gasoline-action-toast');
    if (existing)
        existing.remove();
    const theme = TOAST_THEMES[state] ?? TOAST_THEMES.trying;
    const toast = document.createElement('div');
    toast.id = 'gasoline-action-toast';
    // Build content: label + truncated detail
    const label = document.createElement('span');
    label.textContent = truncateText(text, 30);
    Object.assign(label.style, { fontWeight: '700' });
    toast.appendChild(label);
    if (detail) {
        const sep = document.createElement('span');
        sep.textContent = '  ';
        Object.assign(sep.style, { opacity: '0.6', margin: '0 4px' });
        toast.appendChild(sep);
        const det = document.createElement('span');
        det.textContent = truncateText(detail, 50);
        Object.assign(det.style, { fontWeight: '400', opacity: '0.9' });
        toast.appendChild(det);
    }
    Object.assign(toast.style, {
        position: 'fixed',
        top: '16px',
        left: '50%',
        transform: 'translateX(-50%)',
        padding: '8px 20px',
        background: theme.bg,
        color: '#fff',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        fontSize: '13px',
        borderRadius: '8px',
        boxShadow: `0 4px 20px ${theme.shadow}`,
        zIndex: '2147483647',
        pointerEvents: 'none',
        opacity: '0',
        transition: 'opacity 0.2s ease-in',
        maxWidth: '500px',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        display: 'flex',
        alignItems: 'center',
        gap: '0',
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
            const msg = message;
            if (msg.text)
                showActionToast(msg.text, msg.detail, msg.state || 'trying', msg.duration_ms);
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
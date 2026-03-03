/**
 * Purpose: Background script for push delivery — screenshot push, chat push, capability tracking.
 * Why: Enables browser-to-AI message injection via keyboard shortcuts.
 * Docs: docs/features/feature/browser-push/index.md
 */
// push-handler.ts — Background handlers for screenshot push and push capability tracking.
import { getServerUrl } from './state.js';
import { getRequestHeaders } from './server.js';
/** Timeout for push fetch calls (ms). */
const PUSH_FETCH_TIMEOUT_MS = 8_000;
let cachedCapabilities = null;
let capabilitiesFetchedAt = 0;
const CAPABILITIES_CACHE_TTL_MS = 10_000; // 10s cache
/**
 * Fetch push capabilities from the daemon.
 * Caches for 10s to avoid hammering the endpoint.
 */
export async function fetchPushCapabilities() {
    const now = Date.now();
    if (cachedCapabilities && now - capabilitiesFetchedAt < CAPABILITIES_CACHE_TTL_MS) {
        return cachedCapabilities;
    }
    try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), PUSH_FETCH_TIMEOUT_MS);
        const response = await fetch(`${getServerUrl()}/push/capabilities`, {
            method: 'GET',
            headers: getRequestHeaders(),
            signal: controller.signal
        });
        clearTimeout(timeoutId);
        if (!response.ok)
            return null;
        cachedCapabilities = (await response.json());
        capabilitiesFetchedAt = now;
        return cachedCapabilities;
    }
    catch {
        return null;
    }
}
/** Clear the capabilities cache (e.g., on reconnect). */
export function clearPushCapabilitiesCache() {
    cachedCapabilities = null;
    capabilitiesFetchedAt = 0;
}
/**
 * Install the push_screenshot keyboard shortcut listener.
 * When Alt+Shift+S is pressed, captures the active tab's screenshot
 * and pushes to the daemon.
 */
export function installPushCommandListener(logFn) {
    if (typeof chrome === 'undefined' || !chrome.commands)
        return;
    chrome.commands.onCommand.addListener(async (command) => {
        if (command !== 'push_screenshot')
            return;
        try {
            const caps = await fetchPushCapabilities();
            if (!caps || !caps.push_enabled) {
                await showPushUnavailableToast('Cannot push screenshot, only compatible with Claude Code');
                return;
            }
            const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
            const tab = tabs[0];
            if (!tab?.id)
                return;
            // Show "trying" toast for visual loading state
            try {
                await chrome.tabs.sendMessage(tab.id, {
                    type: 'GASOLINE_ACTION_TOAST',
                    text: 'Capturing screenshot...',
                    state: 'trying',
                    duration_ms: 3000
                });
            }
            catch {
                // Tab unreachable for toast
            }
            const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId ?? chrome.windows.WINDOW_ID_CURRENT, {
                format: 'png'
            });
            const result = await pushScreenshot(dataUrl, '', tab.url ?? '', tab.id);
            try {
                if (result) {
                    await chrome.tabs.sendMessage(tab.id, {
                        type: 'GASOLINE_ACTION_TOAST',
                        text: 'Screenshot pushed',
                        detail: result.status === 'delivered' ? 'Sent via sampling' : 'Queued in inbox',
                        state: 'success',
                        duration_ms: 2000
                    });
                }
                else {
                    await chrome.tabs.sendMessage(tab.id, {
                        type: 'GASOLINE_ACTION_TOAST',
                        text: 'Screenshot push failed',
                        detail: 'Could not reach Gasoline daemon',
                        state: 'error',
                        duration_ms: 3000
                    });
                }
            }
            catch {
                // Tab unreachable for toast
            }
        }
        catch (err) {
            if (logFn)
                logFn(`Screenshot push error: ${err.message}`);
        }
    });
}
/**
 * Install the push_chat keyboard shortcut listener.
 * When Alt+Shift+C is pressed, sends a message to the content script
 * to show/toggle the chat widget.
 */
export function installChatCommandListener(logFn) {
    if (typeof chrome === 'undefined' || !chrome.commands)
        return;
    chrome.commands.onCommand.addListener(async (command) => {
        if (command !== 'push_chat')
            return;
        try {
            const caps = await fetchPushCapabilities();
            if (!caps || !caps.push_enabled) {
                await showPushUnavailableToast('Cannot push chat, only compatible with Claude Code');
                return;
            }
            const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
            const tab = tabs[0];
            if (!tab?.id)
                return;
            await chrome.tabs.sendMessage(tab.id, {
                type: 'GASOLINE_TOGGLE_CHAT',
                client_name: caps.client_name || 'AI',
                server_url: getServerUrl()
            });
        }
        catch (err) {
            if (logFn)
                logFn(`Chat toggle error: ${err.message}`);
        }
    });
}
/**
 * Push a screenshot to the daemon's push pipeline.
 */
export async function pushScreenshot(screenshotDataUrl, note, pageUrl, tabId) {
    try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), PUSH_FETCH_TIMEOUT_MS);
        const response = await fetch(`${getServerUrl()}/push/screenshot`, {
            method: 'POST',
            headers: getRequestHeaders(),
            body: JSON.stringify({
                screenshot_data_url: screenshotDataUrl,
                note,
                page_url: pageUrl,
                tab_id: tabId
            }),
            signal: controller.signal
        });
        clearTimeout(timeoutId);
        if (!response.ok)
            return null;
        return (await response.json());
    }
    catch {
        return null;
    }
}
/**
 * Push a chat message to the daemon's push pipeline.
 * If conversationId is provided, the daemon tracks the message for SSE response delivery.
 */
export async function pushChatMessage(message, pageUrl, tabId, conversationId) {
    try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), PUSH_FETCH_TIMEOUT_MS);
        const body = {
            message,
            page_url: pageUrl,
            tab_id: tabId
        };
        if (conversationId) {
            body.conversation_id = conversationId;
        }
        const response = await fetch(`${getServerUrl()}/push/message`, {
            method: 'POST',
            headers: getRequestHeaders(),
            body: JSON.stringify(body),
            signal: controller.signal
        });
        clearTimeout(timeoutId);
        if (!response.ok)
            return null;
        return (await response.json());
    }
    catch {
        return null;
    }
}
/** Show a toast when push is unavailable. */
async function showPushUnavailableToast(detail) {
    try {
        const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
        const tab = tabs[0];
        if (!tab?.id)
            return;
        await chrome.tabs.sendMessage(tab.id, {
            type: 'GASOLINE_ACTION_TOAST',
            text: 'Push unavailable',
            detail,
            state: 'error',
            duration_ms: 3000
        });
    }
    catch {
        // Tab unreachable
    }
}
//# sourceMappingURL=push-handler.js.map
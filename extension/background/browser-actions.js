// browser-actions.ts — Browser navigation and action handlers.
// Handles navigate, refresh, back, forward actions with async timeout support.
import * as eventListeners from './event-listeners.js';
import * as index from './index.js';
import { DebugCategory } from './debug.js';
import { broadcastTrackingState } from './message-handlers.js';
import { executeWithWorldRouting } from './query-execution.js';
import { ASYNC_COMMAND_TIMEOUT_MS } from '../lib/constants.js';
const { debugLog } = index;
// =============================================================================
// TIMEOUT CONFIGURATION
// =============================================================================
const ASYNC_EXECUTE_TIMEOUT_MS = ASYNC_COMMAND_TIMEOUT_MS;
const ASYNC_BROWSER_ACTION_TIMEOUT_MS = ASYNC_COMMAND_TIMEOUT_MS;
// =============================================================================
// NAVIGATION
// =============================================================================
// #lizard forgives
export async function handleNavigateAction(tabId, url, actionToast, reason) {
    if (url.startsWith('chrome://') || url.startsWith('chrome-extension://')) {
        return { success: false, error: 'restricted_url', message: 'Cannot navigate to Chrome internal pages' };
    }
    actionToast(tabId, reason || 'navigate', reason ? undefined : url, 'trying', 10000);
    await chrome.tabs.update(tabId, { url });
    await eventListeners.waitForTabLoad(tabId);
    await new Promise((r) => setTimeout(r, 500));
    if (await eventListeners.pingContentScript(tabId)) {
        broadcastTrackingState().catch(() => { });
        actionToast(tabId, reason || 'navigate', reason ? undefined : url, 'success');
        return { success: true, action: 'navigate', url, content_script_status: 'loaded', message: 'Content script ready' };
    }
    const tab = await chrome.tabs.get(tabId);
    if (tab.url?.startsWith('file://')) {
        return { success: true, action: 'navigate', url, content_script_status: 'unavailable', message: 'Content script cannot load on file:// URLs. Enable "Allow access to file URLs" in extension settings.' };
    }
    debugLog(DebugCategory.CAPTURE, 'Content script not loaded after navigate, refreshing', { tabId, url });
    await chrome.tabs.reload(tabId);
    await eventListeners.waitForTabLoad(tabId);
    await new Promise((r) => setTimeout(r, 1000));
    if (await eventListeners.pingContentScript(tabId)) {
        broadcastTrackingState().catch(() => { });
        return { success: true, action: 'navigate', url, content_script_status: 'refreshed', message: 'Page refreshed to load content script' };
    }
    return { success: true, action: 'navigate', url, content_script_status: 'failed', message: 'Navigation complete but content script could not be loaded. AI Web Pilot tools may not work.' };
}
// =============================================================================
// BROWSER ACTION DISPATCH
// =============================================================================
export async function handleBrowserAction(tabId, params, actionToast) {
    const { action, url, reason } = params || {};
    if (!index.__aiWebPilotEnabledCache) {
        return { success: false, error: 'ai_web_pilot_disabled', message: 'AI Web Pilot is not enabled' };
    }
    try {
        switch (action) {
            case 'refresh':
                actionToast(tabId, reason || 'refresh', reason ? undefined : 'reloading page', 'trying', 10000);
                await chrome.tabs.reload(tabId);
                await eventListeners.waitForTabLoad(tabId);
                actionToast(tabId, reason || 'refresh', undefined, 'success');
                return { success: true, action: 'refresh' };
            case 'navigate':
                if (!url)
                    return { success: false, error: 'missing_url', message: 'URL required for navigate action' };
                return handleNavigateAction(tabId, url, actionToast, reason);
            case 'back':
                await chrome.tabs.goBack(tabId);
                return { success: true, action: 'back' };
            case 'forward':
                await chrome.tabs.goForward(tabId);
                return { success: true, action: 'forward' };
            case 'new_tab':
                if (!url)
                    return { success: false, error: 'missing_url', message: 'URL required for new_tab action' };
                actionToast(tabId, reason || 'new_tab', reason ? undefined : 'opening new tab', 'trying', 5000);
                await chrome.tabs.create({ url, active: false });
                actionToast(tabId, reason || 'new_tab', undefined, 'success');
                return { success: true, action: 'new_tab', url };
            default:
                return { success: false, error: 'unknown_action', message: `Unknown action: ${action}` };
        }
    }
    catch (err) {
        return { success: false, error: 'browser_action_failed', message: err.message };
    }
}
// =============================================================================
// ASYNC EXECUTE COMMAND
// =============================================================================
export async function handleAsyncExecuteCommand(query, tabId, world, syncClient, sendAsyncResult, actionToast) {
    const startTime = Date.now();
    // Extract reason for toast display
    let reason;
    try {
        const p = typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
        reason = p?.reason;
    }
    catch {
        /* ignore parse errors */
    }
    try {
        const result = await Promise.race([
            executeWithWorldRouting(tabId, query.params, world),
            new Promise((_, reject) => {
                setTimeout(() => reject(new Error(`Script execution timed out after ${ASYNC_EXECUTE_TIMEOUT_MS}ms. Script may be stuck in a loop or waiting for user input.`)), ASYNC_EXECUTE_TIMEOUT_MS);
            })
        ]);
        if (result.success) {
            actionToast(tabId, reason || 'execute_js', undefined, 'success');
        }
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', result);
        debugLog(DebugCategory.CONNECTION, 'Completed async command', {
            correlationId: query.correlation_id,
            elapsed: Date.now() - startTime,
            success: result.success
        });
    }
    catch {
        const timeoutMessage = `JavaScript execution exceeded ${ASYNC_EXECUTE_TIMEOUT_MS / 1000}s timeout. RECOMMENDED ACTIONS:

1. Break your task into smaller discrete steps that execute in < ${ASYNC_EXECUTE_TIMEOUT_MS / 1000}s
2. Check your script for infinite loops or blocking operations
3. Simplify the operation or target a smaller DOM scope`;
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'timeout', null, timeoutMessage);
        debugLog(DebugCategory.CONNECTION, 'Async command timeout', {
            correlationId: query.correlation_id,
            elapsed: Date.now() - startTime
        });
    }
}
// =============================================================================
// ASYNC BROWSER ACTION
// =============================================================================
export async function handleAsyncBrowserAction(query, tabId, params, syncClient, sendAsyncResult, actionToast) {
    const startTime = Date.now();
    const executionPromise = handleBrowserAction(tabId, params, actionToast)
        .then((result) => {
        return result;
    })
        .catch((err) => {
        return {
            success: false,
            error: err.message || 'Browser action failed'
        };
    });
    try {
        const execResult = await Promise.race([
            executionPromise,
            new Promise((_, reject) => {
                setTimeout(() => reject(new Error(`Browser action execution timed out after ${ASYNC_BROWSER_ACTION_TIMEOUT_MS}ms. Action may be waiting for user interaction or network response.`)), ASYNC_BROWSER_ACTION_TIMEOUT_MS);
            })
        ]);
        if (execResult.success !== false) {
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', execResult);
        }
        else {
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', null, execResult.error);
        }
        debugLog(DebugCategory.CONNECTION, 'Completed async browser action', {
            correlationId: query.correlation_id,
            elapsed: Date.now() - startTime,
            success: execResult.success !== false
        });
    }
    catch {
        // nosemgrep: missing-template-string-indicator
        const timeoutMessage = `Browser action exceeded ${ASYNC_BROWSER_ACTION_TIMEOUT_MS / 1000}s timeout. DIAGNOSTIC STEPS:

1. Check page status: observe({what: 'page'})
2. Check for console errors: observe({what: 'errors'})
3. Check network requests: observe({what: 'network', status_min: 400})`;
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'timeout', null, timeoutMessage);
        debugLog(DebugCategory.CONNECTION, 'Async browser action timeout', {
            correlationId: query.correlation_id,
            elapsed: Date.now() - startTime
        });
    }
}
//# sourceMappingURL=browser-actions.js.map
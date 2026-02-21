// helpers.ts — Shared infrastructure for command dispatch.
// Types, result helpers, target resolution, action toast, and constants.
import { getTrackedTabInfo, clearTrackedTab } from '../event-listeners.js';
import { debugLog, diagnosticLog } from '../index.js';
import { DebugCategory } from '../debug.js';
// =============================================================================
// RESULT HELPERS
// =============================================================================
/** Send a query result back through /sync */
export function sendResult(syncClient, queryId, result) {
    debugLog(DebugCategory.CONNECTION, 'sendResult via /sync', { queryId, hasResult: result != null });
    syncClient.queueCommandResult({ id: queryId, status: 'complete', result });
}
/** Send an async command result back through /sync */
export function sendAsyncResult(syncClient, queryId, correlationId, status, result, error) {
    debugLog(DebugCategory.CONNECTION, 'sendAsyncResult via /sync', {
        queryId,
        correlationId,
        status,
        hasResult: result != null,
        error: error || null
    });
    syncClient.queueCommandResult({
        id: queryId,
        correlation_id: correlationId,
        status,
        result,
        error
    });
}
// =============================================================================
// ACTION TOAST
// =============================================================================
/** Map raw action names to human-readable toast labels */
const PRETTY_LABELS = {
    navigate: 'Navigate to',
    refresh: 'Refresh',
    execute_js: 'Execute',
    click: 'Click',
    type: 'Type',
    select: 'Select',
    check: 'Check',
    focus: 'Focus',
    scroll_to: 'Scroll to',
    wait_for: 'Wait for',
    key_press: 'Key press',
    highlight: 'Highlight',
    subtitle: 'Subtitle',
    upload: 'Upload file'
};
/** Show a visual action toast on the tracked tab */
export function actionToast(tabId, action, detail, state = 'success', durationMs = 3000) {
    chrome.tabs
        .sendMessage(tabId, {
        type: 'GASOLINE_ACTION_TOAST',
        text: PRETTY_LABELS[action] || action,
        detail,
        state,
        duration_ms: durationMs
    })
        .catch(() => { });
}
// =============================================================================
// PARAMS PARSING
// =============================================================================
export function parseQueryParamsObject(params) {
    if (typeof params === 'string') {
        try {
            const parsed = JSON.parse(params);
            if (parsed && typeof parsed === 'object') {
                return parsed;
            }
        }
        catch {
            return {};
        }
        return {};
    }
    if (params && typeof params === 'object') {
        return params;
    }
    return {};
}
// =============================================================================
// TARGET RESOLUTION
// =============================================================================
export function withTargetContext(result, target) {
    const targetContext = {
        resolved_tab_id: target.tabId,
        resolved_url: target.url,
        target_context: {
            source: target.source,
            requested_tab_id: target.requestedTabId ?? null,
            tracked_tab_id: target.trackedTabId ?? null,
            use_active_tab: target.useActiveTab
        }
    };
    if (result && typeof result === 'object' && !Array.isArray(result)) {
        return {
            ...result,
            ...targetContext
        };
    }
    return {
        value: result ?? null,
        ...targetContext
    };
}
const TARGETED_QUERY_TYPES = new Set([
    'subtitle',
    'screenshot',
    'browser_action',
    'highlight',
    'page_info',
    'waterfall',
    'dom',
    'a11y',
    'dom_action',
    'upload',
    'record_start',
    'execute',
    'link_health',
    'draw_mode',
    'computed_styles',
    'form_discovery'
]);
export function requiresTargetTab(queryType) {
    return TARGETED_QUERY_TYPES.has(queryType);
}
export function isBrowserEscapeAction(queryType, paramsObj) {
    if (queryType !== 'browser_action')
        return false;
    const action = typeof paramsObj.action === 'string'
        ? paramsObj.action
        : typeof paramsObj.what === 'string'
            ? paramsObj.what
            : '';
    return action === 'navigate' || action === 'refresh' || action === 'back' || action === 'forward' || action === 'new_tab';
}
async function getTabWithRetry(tabId, retry = false) {
    try {
        return await chrome.tabs.get(tabId);
    }
    catch {
        if (!retry) {
            return null;
        }
        await new Promise((r) => setTimeout(r, 300));
        try {
            return await chrome.tabs.get(tabId);
        }
        catch {
            return null;
        }
    }
}
async function getActiveTab() {
    const activeTabs = await chrome.tabs.query({ active: true, currentWindow: true });
    const tab = activeTabs[0];
    if (!tab?.id) {
        return null;
    }
    return tab;
}
function buildMissingTargetError(queryType, useActiveTab, trackedTabId) {
    const message = "No target tab resolved. Provide 'tab_id', enable tab tracking, or set 'use_active_tab=true' explicitly.";
    return {
        message,
        payload: {
            success: false,
            error: 'missing_target',
            message,
            query_type: queryType,
            use_active_tab: useActiveTab,
            tracked_tab_id: trackedTabId
        }
    };
}
export async function resolveTargetTab(query, paramsObj) {
    const explicitTabId = typeof query.tab_id === 'number' && query.tab_id > 0 ? query.tab_id : undefined;
    const useActiveTab = paramsObj.use_active_tab === true;
    if (explicitTabId) {
        const explicitTab = await getTabWithRetry(explicitTabId);
        if (!explicitTab?.id) {
            const message = `Requested tab_id ${explicitTabId} is not available`;
            return {
                error: {
                    message,
                    payload: {
                        success: false,
                        error: 'target_tab_not_found',
                        message,
                        requested_tab_id: explicitTabId
                    }
                }
            };
        }
        return {
            target: {
                tabId: explicitTab.id,
                url: explicitTab.url || '',
                source: 'explicit_tab',
                requestedTabId: explicitTabId,
                trackedTabId: null,
                useActiveTab
            }
        };
    }
    if (useActiveTab) {
        const activeTab = await getActiveTab();
        if (!activeTab?.id) {
            return {
                error: {
                    message: 'No active tab available',
                    payload: {
                        success: false,
                        error: 'no_active_tab',
                        message: 'No active tab available',
                        use_active_tab: true
                    }
                }
            };
        }
        return {
            target: {
                tabId: activeTab.id,
                url: activeTab.url || '',
                source: 'active_tab',
                trackedTabId: null,
                useActiveTab
            }
        };
    }
    const storage = await getTrackedTabInfo();
    const trackedTabId = storage.trackedTabId ?? null;
    if (trackedTabId) {
        diagnosticLog(`[Diagnostic] Using tracked tab ${trackedTabId} for query ${query.type}`);
        const trackedTab = await getTabWithRetry(trackedTabId, true);
        if (trackedTab?.id) {
            return {
                target: {
                    tabId: trackedTab.id,
                    url: trackedTab.url || storage.trackedTabUrl || '',
                    source: 'tracked_tab',
                    trackedTabId,
                    useActiveTab
                }
            };
        }
        diagnosticLog(`[Diagnostic] Tracked tab ${trackedTabId} unavailable, clearing tracking state`);
        clearTrackedTab();
        try {
            const toastTab = await getActiveTab();
            if (toastTab?.id) {
                chrome.tabs
                    .sendMessage(toastTab.id, {
                    type: 'GASOLINE_ACTION_TOAST',
                    text: 'Tracked tab unavailable',
                    detail: "Provide tab_id or use 'use_active_tab=true'",
                    state: 'warning',
                    duration_ms: 5000
                })
                    .catch(() => { });
            }
        }
        catch {
            /* best effort */
        }
        if (isBrowserEscapeAction(query.type, paramsObj)) {
            const activeTab = await getActiveTab();
            if (activeTab?.id) {
                diagnosticLog(`[Diagnostic] Falling back to active tab ${activeTab.id} for escape action ${query.type}`);
                return {
                    target: {
                        tabId: activeTab.id,
                        url: activeTab.url || '',
                        source: 'active_tab_fallback',
                        trackedTabId,
                        useActiveTab: true
                    }
                };
            }
        }
        return { error: buildMissingTargetError(query.type, useActiveTab, trackedTabId) };
    }
    if (isBrowserEscapeAction(query.type, paramsObj)) {
        const activeTab = await getActiveTab();
        if (activeTab?.id) {
            diagnosticLog(`[Diagnostic] Using active tab fallback ${activeTab.id} for escape action ${query.type}`);
            return {
                target: {
                    tabId: activeTab.id,
                    url: activeTab.url || '',
                    source: 'active_tab_fallback',
                    trackedTabId: null,
                    useActiveTab: true
                }
            };
        }
    }
    return { error: buildMissingTargetError(query.type, useActiveTab, trackedTabId) };
}
/**
 * Check if a URL is restricted — content scripts cannot run on these pages.
 * Covers internal browser pages and known CSP-restricted origins.
 */
export function isRestrictedUrl(url) {
    if (!url)
        return true;
    const blocked = ['chrome://', 'chrome-extension://', 'about:', 'edge://', 'brave://', 'devtools://'];
    return blocked.some((p) => url.startsWith(p));
}
//# sourceMappingURL=helpers.js.map
/**
 * @fileoverview Pending Query Handlers
 * Handles all query types from the server: DOM, accessibility, browser actions,
 * execute commands, and state management.
 *
 * All results are returned via syncClient.queueCommandResult() which routes them
 * through the unified /sync endpoint. No direct HTTP POSTs to legacy endpoints.
 *
 * Split into modules:
 * - query-execution.ts: JS execution with world-aware routing and CSP fallback
 * - browser-actions.ts: Browser navigation/action handlers with async timeout support
 */
import * as eventListeners from './event-listeners.js';
import * as index from './index.js';
import { DebugCategory } from './debug.js';
import { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot } from './message-handlers.js';
import { executeDOMAction } from './dom-primitives.js';
import { executeUpload } from './upload-handler.js';
import { canTakeScreenshot, recordScreenshot } from './state-manager.js';
import { startRecording, stopRecording } from './recording.js';
import { executeWithWorldRouting } from './query-execution.js';
import { handleBrowserAction, handleAsyncBrowserAction, handleAsyncExecuteCommand } from './browser-actions.js';
// Extract values from index for easier reference (but NOT DebugCategory - imported directly above)
const { debugLog, diagnosticLog } = index;
// =============================================================================
// RESULT HELPERS
// =============================================================================
/** Send a query result back through /sync */
function sendResult(syncClient, queryId, result) {
    debugLog(DebugCategory.CONNECTION, 'sendResult via /sync', { queryId, hasResult: result != null });
    syncClient.queueCommandResult({ id: queryId, status: 'complete', result });
}
/** Send an async command result back through /sync */
function sendAsyncResult(syncClient, queryId, correlationId, status, result, error) {
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
function actionToast(tabId, action, detail, state = 'success', durationMs = 3000) {
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
    'draw_mode'
]);
function requiresTargetTab(queryType) {
    return TARGETED_QUERY_TYPES.has(queryType);
}
function parseQueryParamsObject(params) {
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
function normalizeAnalyzeFrameTarget(frame) {
    if (frame === undefined || frame === null)
        return undefined;
    if (typeof frame === 'number') {
        if (!Number.isInteger(frame) || frame < 0)
            return null;
        return frame;
    }
    if (typeof frame === 'string') {
        const trimmed = frame.trim();
        if (trimmed.length === 0)
            return null;
        return trimmed;
    }
    return null;
}
/**
 * Frame selection probe executed in page context.
 * Must be self-contained for chrome.scripting.executeScript({ func }).
 */
function analyzeFrameProbe(frameTarget) {
    const isTop = window === window.top;
    const getParentFrameIndex = () => {
        if (isTop)
            return -1;
        try {
            const parentFrames = window.parent?.frames;
            if (!parentFrames)
                return -1;
            for (let i = 0; i < parentFrames.length; i++) {
                if (parentFrames[i] === window)
                    return i;
            }
        }
        catch {
            return -1;
        }
        return -1;
    };
    if (frameTarget === undefined) {
        return { matches: isTop };
    }
    if (frameTarget === 'all') {
        return { matches: true };
    }
    if (typeof frameTarget === 'number') {
        return { matches: getParentFrameIndex() === frameTarget };
    }
    if (isTop) {
        return { matches: false };
    }
    try {
        const frameEl = window.frameElement;
        if (!frameEl || typeof frameEl.matches !== 'function') {
            return { matches: false };
        }
        return { matches: frameEl.matches(frameTarget) };
    }
    catch {
        return { matches: false };
    }
}
async function resolveAnalyzeFrameSelection(tabId, frame) {
    const normalized = normalizeAnalyzeFrameTarget(frame);
    if (normalized === null) {
        throw new Error('invalid_frame');
    }
    const probeResults = await chrome.scripting.executeScript({
        target: { tabId, allFrames: true },
        world: 'MAIN',
        func: analyzeFrameProbe,
        args: [normalized]
    });
    const frameIds = Array.from(new Set(probeResults
        .filter((r) => !!r.result?.matches)
        .map((r) => r.frameId)
        .filter((id) => typeof id === 'number')));
    if (frameIds.length === 0) {
        throw new Error('frame_not_found');
    }
    if (normalized === undefined) {
        return { frameIds, mode: 'main' };
    }
    if (normalized === 'all') {
        return { frameIds, mode: 'all' };
    }
    return { frameIds, mode: 'targeted' };
}
function stripFrameParam(params) {
    const copy = { ...params };
    delete copy.frame;
    return copy;
}
async function sendFrameQueries(tabId, frameIds, message) {
    return Promise.all(frameIds.map(async (frameId) => {
        try {
            const result = (await chrome.tabs.sendMessage(tabId, message, { frameId }));
            return { frame_id: frameId, result };
        }
        catch (err) {
            return {
                frame_id: frameId,
                error: err.message || 'frame_query_failed'
            };
        }
    }));
}
function toNonNegativeInt(value) {
    if (typeof value !== 'number' || !Number.isFinite(value))
        return 0;
    const n = Math.floor(value);
    return n > 0 ? n : 0;
}
function aggregateDOMFrameResults(results) {
    const MAX_MATCHES = 200;
    const matches = [];
    const frames = [];
    let totalMatchCount = 0;
    let totalReturnedCount = 0;
    let url = '';
    let title = '';
    for (const entry of results) {
        if (entry.error) {
            frames.push({ frame_id: entry.frame_id, error: entry.error });
            continue;
        }
        const payload = entry.result || {};
        const frameMatchCount = toNonNegativeInt(payload.matchCount);
        const frameReturnedCount = toNonNegativeInt(payload.returnedCount);
        const frameMatches = Array.isArray(payload.matches) ? payload.matches : [];
        if (!url && typeof payload.url === 'string') {
            url = payload.url;
        }
        if (!title && typeof payload.title === 'string') {
            title = payload.title;
        }
        totalMatchCount += frameMatchCount;
        totalReturnedCount += frameReturnedCount;
        if (matches.length < MAX_MATCHES) {
            matches.push(...frameMatches.slice(0, MAX_MATCHES - matches.length));
        }
        frames.push({
            frame_id: entry.frame_id,
            match_count: frameMatchCount,
            returned_count: frameReturnedCount,
            ...(payload.error ? { error: payload.error } : {})
        });
    }
    return {
        url,
        title,
        matchCount: totalMatchCount,
        returnedCount: totalReturnedCount,
        matches,
        frames
    };
}
function aggregateA11yFrameResults(results) {
    const violations = [];
    const passes = [];
    const incomplete = [];
    const inapplicable = [];
    const frames = [];
    const errors = [];
    for (const entry of results) {
        if (entry.error) {
            frames.push({ frame_id: entry.frame_id, error: entry.error });
            errors.push(entry.error);
            continue;
        }
        const payload = entry.result || {};
        const frameViolations = Array.isArray(payload.violations) ? payload.violations : [];
        const framePasses = Array.isArray(payload.passes) ? payload.passes : [];
        const frameIncomplete = Array.isArray(payload.incomplete) ? payload.incomplete : [];
        const frameInapplicable = Array.isArray(payload.inapplicable) ? payload.inapplicable : [];
        violations.push(...frameViolations);
        passes.push(...framePasses);
        incomplete.push(...frameIncomplete);
        inapplicable.push(...frameInapplicable);
        const frameSummary = payload.summary;
        frames.push({
            frame_id: entry.frame_id,
            summary: {
                violations: toNonNegativeInt(frameSummary?.violations ?? frameViolations.length),
                passes: toNonNegativeInt(frameSummary?.passes ?? framePasses.length),
                incomplete: toNonNegativeInt(frameSummary?.incomplete ?? frameIncomplete.length),
                inapplicable: toNonNegativeInt(frameSummary?.inapplicable ?? frameInapplicable.length)
            },
            ...(payload.error ? { error: payload.error } : {})
        });
        if (typeof payload.error === 'string' && payload.error.length > 0) {
            errors.push(payload.error);
        }
    }
    return {
        violations,
        passes,
        incomplete,
        inapplicable,
        summary: {
            violations: violations.length,
            passes: passes.length,
            incomplete: incomplete.length,
            inapplicable: inapplicable.length
        },
        frames,
        ...(errors.length > 0 ? { error: errors.join('; ') } : {})
    };
}
function withTargetContext(result, target) {
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
async function resolveTargetTab(query, paramsObj) {
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
    const storage = await eventListeners.getTrackedTabInfo();
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
        eventListeners.clearTrackedTab();
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
        return { error: buildMissingTargetError(query.type, useActiveTab, trackedTabId) };
    }
    return { error: buildMissingTargetError(query.type, useActiveTab, trackedTabId) };
}
// =============================================================================
// PENDING QUERY HANDLING
// =============================================================================
export async function handlePendingQuery(query, syncClient) {
    // Wait for initialization to complete (max 2s) so pilot cache is populated
    await Promise.race([index.initReady, new Promise((r) => setTimeout(r, 2000))]);
    debugLog(DebugCategory.CONNECTION, 'handlePendingQuery ENTER', {
        id: query.id,
        type: query.type,
        correlation_id: query.correlation_id || null,
        hasSyncClient: !!syncClient
    });
    let target;
    const wrapResult = (result) => {
        if (!target) {
            return result;
        }
        return withTargetContext(result, target);
    };
    const sendQueryResult = (result) => {
        sendResult(syncClient, query.id, wrapResult(result));
    };
    const sendQueryAsyncResult = (client, queryId, correlationId, status, result, error) => {
        sendAsyncResult(client, queryId, correlationId, status, wrapResult(result), error);
    };
    try {
        if (query.type.startsWith('state_')) {
            await handleStateQuery(query, syncClient);
            return;
        }
        const paramsObj = parseQueryParamsObject(query.params);
        const needsTargetTab = requiresTargetTab(query.type);
        let tabId;
        if (needsTargetTab) {
            const resolved = await resolveTargetTab(query, paramsObj);
            if (resolved.error) {
                if (query.correlation_id) {
                    sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', resolved.error.payload, resolved.error.message);
                }
                else {
                    sendResult(syncClient, query.id, resolved.error.payload);
                }
                return;
            }
            target = resolved.target;
            tabId = target?.tabId;
        }
        if (needsTargetTab && !tabId) {
            const payload = {
                success: false,
                error: 'missing_target',
                message: 'No target tab resolved for query'
            };
            if (query.correlation_id) {
                sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', payload, payload.message);
            }
            else {
                sendResult(syncClient, query.id, payload);
            }
            return;
        }
        const targetTabId = tabId ?? 0;
        if (query.type === 'subtitle') {
            let params;
            try {
                params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
            }
            catch {
                params = {};
            }
            chrome.tabs
                .sendMessage(targetTabId, {
                type: 'GASOLINE_SUBTITLE',
                text: params.text ?? ''
            })
                .catch(() => { });
            sendQueryResult({ success: true, subtitle: params.text || 'cleared' });
            return;
        }
        if (query.type === 'screenshot') {
            try {
                const rateCheck = canTakeScreenshot(targetTabId);
                if (!rateCheck.allowed) {
                    sendQueryResult({
                        error: `Rate limited: ${rateCheck.reason}`,
                        ...(rateCheck.nextAllowedIn != null ? { next_allowed_in: rateCheck.nextAllowedIn } : {})
                    });
                    return;
                }
                const tab = await chrome.tabs.get(targetTabId);
                const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, {
                    format: 'jpeg',
                    quality: 80
                });
                recordScreenshot(targetTabId);
                // POST to /screenshots with query_id — server saves file and resolves query directly
                const response = await fetch(`${index.serverUrl}/screenshots`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json', 'X-Gasoline-Client': 'gasoline-extension' },
                    body: JSON.stringify({
                        data_url: dataUrl,
                        url: tab.url,
                        query_id: query.id
                    })
                });
                if (!response.ok) {
                    sendQueryResult({ error: `Server returned ${response.status}` });
                }
                // No sendResult needed — server resolves the query via query_id
            }
            catch (err) {
                sendQueryResult({
                    error: 'screenshot_failed',
                    message: err.message || 'Failed to capture screenshot'
                });
            }
            return;
        }
        if (query.type === 'browser_action') {
            let params;
            try {
                params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
            }
            catch {
                sendQueryResult({
                    success: false,
                    error: 'invalid_params',
                    message: 'Failed to parse browser_action params as JSON'
                });
                return;
            }
            if (query.correlation_id) {
                await handleAsyncBrowserAction(query, targetTabId, params, syncClient, sendQueryAsyncResult, actionToast);
            }
            else {
                const result = await handleBrowserAction(targetTabId, params, actionToast);
                sendQueryResult(result);
            }
            return;
        }
        if (query.type === 'highlight') {
            let params;
            try {
                params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
            }
            catch {
                sendQueryResult({
                    error: 'invalid_params',
                    message: 'Failed to parse highlight params as JSON'
                });
                return;
            }
            const result = await handlePilotCommand('GASOLINE_HIGHLIGHT', params);
            if (query.correlation_id) {
                const err = result && typeof result === 'object' && 'error' in result ? result.error : undefined;
                sendQueryAsyncResult(syncClient, query.id, query.correlation_id, 'complete', result, err);
            }
            else {
                sendQueryResult(result);
            }
            return;
        }
        if (query.type === 'page_info') {
            const tab = await chrome.tabs.get(targetTabId);
            const result = {
                url: tab.url,
                title: tab.title,
                favicon: tab.favIconUrl,
                status: tab.status,
                viewport: {
                    width: tab.width,
                    height: tab.height
                }
            };
            sendQueryResult(result);
            return;
        }
        if (query.type === 'tabs') {
            const allTabs = await chrome.tabs.query({});
            const tabsList = allTabs.map((tab) => ({
                id: tab.id,
                url: tab.url,
                title: tab.title,
                active: tab.active,
                windowId: tab.windowId,
                index: tab.index
            }));
            sendQueryResult({ tabs: tabsList });
            return;
        }
        // Waterfall query - fetch network waterfall data on demand
        if (query.type === 'waterfall') {
            debugLog(DebugCategory.CAPTURE, 'Handling waterfall query', { queryId: query.id, tabId: targetTabId });
            try {
                const tab = await chrome.tabs.get(targetTabId);
                debugLog(DebugCategory.CAPTURE, 'Got tab for waterfall', { tabId: targetTabId, url: tab.url });
                const result = (await chrome.tabs.sendMessage(targetTabId, {
                    type: 'GET_NETWORK_WATERFALL'
                }));
                debugLog(DebugCategory.CAPTURE, 'Waterfall result from content script', {
                    entries: result?.entries?.length || 0
                });
                sendQueryResult({
                    entries: result?.entries || [],
                    page_url: tab.url || '',
                    count: result?.entries?.length || 0
                });
                debugLog(DebugCategory.CAPTURE, 'Posted waterfall result', { queryId: query.id });
            }
            catch (err) {
                debugLog(DebugCategory.CAPTURE, 'Waterfall query error', {
                    queryId: query.id,
                    error: err.message
                });
                sendQueryResult({
                    error: 'waterfall_query_failed',
                    message: err.message || 'Failed to fetch network waterfall',
                    entries: []
                });
            }
            return;
        }
        if (query.type === 'dom') {
            try {
                const frameSelection = await resolveAnalyzeFrameSelection(targetTabId, paramsObj.frame);
                // Fast path: preserve legacy behavior when no frame is specified.
                if (frameSelection.mode === 'main') {
                    const result = await chrome.tabs.sendMessage(targetTabId, {
                        type: 'DOM_QUERY',
                        params: query.params
                    });
                    if (query.correlation_id) {
                        sendQueryAsyncResult(syncClient, query.id, query.correlation_id, 'complete', result);
                    }
                    else {
                        sendQueryResult(result);
                    }
                    return;
                }
                const frameParams = stripFrameParam(paramsObj);
                const perFrame = await sendFrameQueries(targetTabId, frameSelection.frameIds, {
                    type: 'DOM_QUERY',
                    params: frameParams
                });
                let result;
                if (perFrame.length === 1) {
                    const first = perFrame[0];
                    if (!first) {
                        result = { error: 'dom_query_failed', message: 'No frame response received' };
                    }
                    else if (first.error) {
                        result = { error: 'dom_query_failed', message: first.error, frame_id: first.frame_id };
                    }
                    else {
                        result = { ...(first.result || {}), frame_id: first.frame_id };
                    }
                }
                else {
                    result = aggregateDOMFrameResults(perFrame);
                }
                if (query.correlation_id) {
                    sendQueryAsyncResult(syncClient, query.id, query.correlation_id, 'complete', result);
                }
                else {
                    sendQueryResult(result);
                }
            }
            catch (err) {
                const message = err.message || 'Failed to execute DOM query';
                const isFrameNotFound = message === 'frame_not_found';
                const isInvalidFrame = message === 'invalid_frame';
                if (query.correlation_id) {
                    sendQueryAsyncResult(syncClient, query.id, query.correlation_id, 'complete', null, isInvalidFrame || isFrameNotFound ? message : 'Failed to execute DOM query');
                }
                else {
                    sendQueryResult({
                        error: isInvalidFrame || isFrameNotFound ? message : 'dom_query_failed',
                        message: isInvalidFrame || isFrameNotFound ? message : 'Failed to execute DOM query'
                    });
                }
            }
            return;
        }
        if (query.type === 'a11y') {
            try {
                const frameSelection = await resolveAnalyzeFrameSelection(targetTabId, paramsObj.frame);
                // Fast path: preserve legacy behavior when no frame is specified.
                if (frameSelection.mode === 'main') {
                    const result = await chrome.tabs.sendMessage(targetTabId, {
                        type: 'A11Y_QUERY',
                        params: query.params
                    });
                    if (query.correlation_id) {
                        sendQueryAsyncResult(syncClient, query.id, query.correlation_id, 'complete', result);
                    }
                    else {
                        sendQueryResult(result);
                    }
                    return;
                }
                const frameParams = stripFrameParam(paramsObj);
                const perFrame = await sendFrameQueries(targetTabId, frameSelection.frameIds, {
                    type: 'A11Y_QUERY',
                    params: frameParams
                });
                let result;
                if (perFrame.length === 1) {
                    const first = perFrame[0];
                    if (!first) {
                        result = { error: 'a11y_audit_failed', message: 'No frame response received' };
                    }
                    else if (first.error) {
                        result = { error: 'a11y_audit_failed', message: first.error, frame_id: first.frame_id };
                    }
                    else {
                        result = { ...(first.result || {}), frame_id: first.frame_id };
                    }
                }
                else {
                    result = aggregateA11yFrameResults(perFrame);
                }
                if (query.correlation_id) {
                    sendQueryAsyncResult(syncClient, query.id, query.correlation_id, 'complete', result);
                }
                else {
                    sendQueryResult(result);
                }
            }
            catch (err) {
                const message = err.message || 'Failed to execute accessibility audit';
                const isFrameNotFound = message === 'frame_not_found';
                const isInvalidFrame = message === 'invalid_frame';
                if (query.correlation_id) {
                    sendQueryAsyncResult(syncClient, query.id, query.correlation_id, 'complete', null, isInvalidFrame || isFrameNotFound ? message : 'Failed to execute accessibility audit');
                }
                else {
                    sendQueryResult({
                        error: isInvalidFrame || isFrameNotFound ? message : 'a11y_audit_failed',
                        message: isInvalidFrame || isFrameNotFound ? message : 'Failed to execute accessibility audit'
                    });
                }
            }
            return;
        }
        if (query.type === 'dom_action') {
            if (!index.__aiWebPilotEnabledCache) {
                sendQueryAsyncResult(syncClient, query.id, query.correlation_id, 'complete', null, 'ai_web_pilot_disabled');
                return;
            }
            await executeDOMAction(query, targetTabId, syncClient, sendQueryAsyncResult, actionToast);
            return;
        }
        if (query.type === 'upload') {
            if (!index.__aiWebPilotEnabledCache) {
                sendQueryAsyncResult(syncClient, query.id, query.correlation_id, 'complete', null, 'ai_web_pilot_disabled');
                return;
            }
            await executeUpload(query, targetTabId, syncClient, sendQueryAsyncResult, actionToast);
            return;
        }
        if (query.type === 'record_start') {
            if (!index.__aiWebPilotEnabledCache) {
                sendQueryAsyncResult(syncClient, query.id, query.correlation_id, 'complete', undefined, 'ai_web_pilot_disabled');
                return;
            }
            let params;
            try {
                params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
            }
            catch {
                params = {};
            }
            const result = await startRecording(params.name ?? 'recording', params.fps ?? 15, query.id, params.audio ?? '', false, targetTabId);
            sendQueryAsyncResult(syncClient, query.id, query.correlation_id, 'complete', result, result.error || undefined);
            return;
        }
        if (query.type === 'record_stop') {
            if (!index.__aiWebPilotEnabledCache) {
                sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', undefined, 'ai_web_pilot_disabled');
                return;
            }
            const result = await stopRecording();
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', result, result.error || undefined);
            return;
        }
        if (query.type === 'execute') {
            if (!index.__aiWebPilotEnabledCache) {
                if (query.correlation_id) {
                    sendQueryAsyncResult(syncClient, query.id, query.correlation_id, 'complete', null, 'ai_web_pilot_disabled');
                }
                else {
                    sendQueryResult({
                        success: false,
                        error: 'ai_web_pilot_disabled',
                        message: 'AI Web Pilot is not enabled in the extension popup'
                    });
                }
                return;
            }
            // Parse world param for routing
            let execParams;
            try {
                execParams = typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
            }
            catch {
                execParams = {};
            }
            const world = execParams.world || 'auto';
            if (query.correlation_id) {
                await handleAsyncExecuteCommand(query, targetTabId, world, syncClient, sendQueryAsyncResult, actionToast);
            }
            else {
                try {
                    const result = await executeWithWorldRouting(targetTabId, query.params, world);
                    sendQueryResult(result);
                }
                catch (err) {
                    sendQueryResult({
                        success: false,
                        error: 'execution_failed',
                        message: err.message || 'Execution failed'
                    });
                }
            }
            return;
        }
        if (query.type === 'link_health') {
            try {
                const result = await chrome.tabs.sendMessage(targetTabId, {
                    type: 'LINK_HEALTH_QUERY',
                    params: query.params
                });
                sendQueryResult(result);
            }
            catch (err) {
                sendQueryResult({
                    error: 'link_health_failed',
                    message: err.message || 'Link health check failed'
                });
            }
            return;
        }
        if (query.type === 'draw_mode') {
            if (!index.__aiWebPilotEnabledCache) {
                sendQueryResult({
                    error: 'ai_web_pilot_disabled',
                    message: 'AI Web Pilot is not enabled in the extension popup'
                });
                return;
            }
            let params;
            try {
                params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
            }
            catch {
                params = {};
            }
            if (params.action === 'start') {
                try {
                    const result = await chrome.tabs.sendMessage(targetTabId, {
                        type: 'GASOLINE_DRAW_MODE_START',
                        started_by: 'llm',
                        session_name: params.session || '',
                        correlation_id: query.correlation_id || query.id || ''
                    });
                    sendQueryResult({
                        status: result?.status || 'active',
                        message: 'Draw mode activated. User can now draw annotations on the page. Results will be delivered when user finishes (presses ESC).',
                        annotation_count: result?.annotation_count || 0
                    });
                }
                catch (err) {
                    sendQueryResult({
                        error: 'draw_mode_failed',
                        message: err.message ||
                            'Failed to activate draw mode. Ensure content script is loaded (try refreshing the page).'
                    });
                }
            }
            else {
                sendQueryResult({
                    error: 'unknown_draw_mode_action',
                    message: `Unknown draw mode action: ${params.action}. Use 'start'.`
                });
            }
            return;
        }
    }
    catch (err) {
        const errMsg = err.message || 'Unexpected error handling query';
        debugLog(DebugCategory.CONNECTION, 'Error handling pending query', {
            type: query.type,
            id: query.id,
            error: errMsg
        });
        if (query.correlation_id) {
            sendQueryAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, errMsg);
        }
        else {
            sendQueryResult({ error: 'query_handler_error', message: errMsg });
        }
    }
}
// =============================================================================
// STATE QUERY HANDLING
// =============================================================================
async function handleStateQuery(query, syncClient) {
    if (!index.__aiWebPilotEnabledCache) {
        sendResult(syncClient, query.id, { error: 'ai_web_pilot_disabled' });
        return;
    }
    let params;
    try {
        params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
    }
    catch {
        sendResult(syncClient, query.id, {
            error: 'invalid_params',
            message: 'Failed to parse state query params as JSON'
        });
        return;
    }
    const action = params.action;
    try {
        let result;
        switch (action) {
            case 'capture': {
                const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
                const firstTab = tabs[0];
                if (!firstTab?.id) {
                    sendResult(syncClient, query.id, { error: 'no_active_tab' });
                    return;
                }
                result = await chrome.tabs.sendMessage(firstTab.id, {
                    type: 'GASOLINE_MANAGE_STATE',
                    params: { action: 'capture' }
                });
                break;
            }
            case 'save': {
                const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
                const firstTab = tabs[0];
                if (!firstTab?.id) {
                    sendResult(syncClient, query.id, { error: 'no_active_tab' });
                    return;
                }
                const captureResult = (await chrome.tabs.sendMessage(firstTab.id, {
                    type: 'GASOLINE_MANAGE_STATE',
                    params: { action: 'capture' }
                }));
                if (captureResult.error) {
                    sendResult(syncClient, query.id, { error: captureResult.error });
                    return;
                }
                result = await saveStateSnapshot(params.name, captureResult);
                break;
            }
            case 'load': {
                const snapshot = await loadStateSnapshot(params.name);
                if (!snapshot) {
                    sendResult(syncClient, query.id, {
                        error: `Snapshot '${params.name}' not found`
                    });
                    return;
                }
                const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
                const firstTab = tabs[0];
                if (!firstTab?.id) {
                    sendResult(syncClient, query.id, { error: 'no_active_tab' });
                    return;
                }
                result = await chrome.tabs.sendMessage(firstTab.id, {
                    type: 'GASOLINE_MANAGE_STATE',
                    params: {
                        action: 'restore',
                        state: snapshot,
                        include_url: params.include_url !== false
                    }
                });
                break;
            }
            case 'list':
                result = { snapshots: await listStateSnapshots() };
                break;
            case 'delete':
                result = await deleteStateSnapshot(params.name);
                break;
            default:
                result = { error: `Unknown action: ${action}` };
        }
        sendResult(syncClient, query.id, result);
    }
    catch (err) {
        sendResult(syncClient, query.id, { error: err.message });
    }
}
// =============================================================================
// PILOT COMMAND
// =============================================================================
export async function handlePilotCommand(command, params) {
    if (!index.__aiWebPilotEnabledCache) {
        if (typeof chrome !== 'undefined' && chrome.storage) {
            const localResult = await new Promise((resolve) => {
                chrome.storage.local.get(['aiWebPilotEnabled'], (result) => {
                    resolve(result);
                });
            });
            if (localResult.aiWebPilotEnabled === true) {
                // Update cache (note: this module imports from index.ts which has the state)
                // We can't directly update it, so we return the error
            }
        }
    }
    if (!index.__aiWebPilotEnabledCache) {
        return { error: 'ai_web_pilot_disabled' };
    }
    try {
        const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
        const firstTab = tabs[0];
        if (!firstTab?.id) {
            return { error: 'no_active_tab' };
        }
        const tabId = firstTab.id;
        const result = await chrome.tabs.sendMessage(tabId, {
            type: command,
            params
        });
        return result || { success: true };
    }
    catch (err) {
        return { error: err.message || 'command_failed' };
    }
}
//# sourceMappingURL=pending-queries.js.map
/**
 * Purpose: Handles extension background coordination and message routing.
 * Why: Centralizes extension coordination to reduce race conditions and split-brain state.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
import { isAiWebPilotEnabled } from '../state.js';
import { executeDOMAction } from '../dom-dispatch.js';
import { executeUpload } from '../upload-handler.js';
import { startRecording, stopRecording } from '../recording.js';
import { executeWithWorldRouting } from '../query-execution.js';
import { handleBrowserAction, handleAsyncBrowserAction, handleAsyncExecuteCommand } from '../browser-actions.js';
import { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot } from '../message-handlers.js';
import { registerCommand } from './registry.js';
import { sendAsyncResult } from './helpers.js';
function statusFromError(error) {
    return error ? 'error' : 'complete';
}
// =============================================================================
// SUBTITLE
// =============================================================================
registerCommand('subtitle', async (ctx) => {
    let params;
    try {
        params = typeof ctx.query.params === 'string' ? JSON.parse(ctx.query.params) : ctx.query.params;
    }
    catch {
        params = {};
    }
    chrome.tabs
        .sendMessage(ctx.tabId, {
        type: 'GASOLINE_SUBTITLE',
        text: params.text ?? ''
    })
        .catch(() => { });
    ctx.sendResult({ success: true, subtitle: params.text || 'cleared' });
});
// =============================================================================
// HIGHLIGHT
// =============================================================================
registerCommand('highlight', async (ctx) => {
    let params;
    try {
        params = typeof ctx.query.params === 'string' ? JSON.parse(ctx.query.params) : ctx.query.params;
    }
    catch {
        ctx.sendResult({
            error: 'invalid_params',
            message: 'Failed to parse highlight params as JSON'
        });
        return;
    }
    const result = await handlePilotCommand('GASOLINE_HIGHLIGHT', params, ctx.tabId);
    if (ctx.query.correlation_id) {
        const err = result && typeof result === 'object' && 'error' in result ? result.error : undefined;
        ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, statusFromError(err), result, err);
    }
    else {
        ctx.sendResult(result);
    }
});
// =============================================================================
// BROWSER ACTION
// =============================================================================
registerCommand('browser_action', async (ctx) => {
    let params;
    try {
        params = typeof ctx.query.params === 'string' ? JSON.parse(ctx.query.params) : ctx.query.params;
    }
    catch {
        ctx.sendResult({
            success: false,
            error: 'invalid_params',
            message: 'Failed to parse browser_action params as JSON'
        });
        return;
    }
    if (ctx.query.correlation_id) {
        await handleAsyncBrowserAction(ctx.query, ctx.tabId, params, ctx.syncClient, ctx.sendAsyncResult, ctx.actionToast);
    }
    else {
        const result = await handleBrowserAction(ctx.tabId, params, ctx.actionToast);
        ctx.sendResult(result);
    }
});
// =============================================================================
// DOM ACTION
// =============================================================================
registerCommand('dom_action', async (ctx) => {
    if (!isAiWebPilotEnabled()) {
        ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'error', null, 'ai_web_pilot_disabled');
        return;
    }
    await executeDOMAction(ctx.query, ctx.tabId, ctx.syncClient, ctx.sendAsyncResult, ctx.actionToast);
});
// =============================================================================
// UPLOAD
// =============================================================================
registerCommand('upload', async (ctx) => {
    if (!isAiWebPilotEnabled()) {
        ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'error', null, 'ai_web_pilot_disabled');
        return;
    }
    await executeUpload(ctx.query, ctx.tabId, ctx.syncClient, ctx.sendAsyncResult, ctx.actionToast);
});
// =============================================================================
// RECORD START
// =============================================================================
registerCommand('record_start', async (ctx) => {
    if (!isAiWebPilotEnabled()) {
        ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'error', undefined, 'ai_web_pilot_disabled');
        return;
    }
    let params;
    try {
        params = typeof ctx.query.params === 'string' ? JSON.parse(ctx.query.params) : ctx.query.params;
    }
    catch {
        params = {};
    }
    const result = await startRecording(params.name ?? 'recording', params.fps ?? 15, ctx.query.id, params.audio ?? '', false, ctx.tabId);
    const error = result.error || undefined;
    ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, statusFromError(error), result, error);
});
// =============================================================================
// RECORD STOP
// =============================================================================
registerCommand('record_stop', async (ctx) => {
    if (!isAiWebPilotEnabled()) {
        sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'error', undefined, 'ai_web_pilot_disabled');
        return;
    }
    const result = await stopRecording();
    const error = result.error || undefined;
    sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, statusFromError(error), result, error);
});
// =============================================================================
// STATE QUERIES (state_capture, state_save, state_load, state_list, state_delete)
// =============================================================================
registerCommand('state_*', async (ctx) => {
    if (!isAiWebPilotEnabled()) {
        ctx.sendResult({ error: 'ai_web_pilot_disabled' });
        return;
    }
    let params;
    try {
        params = typeof ctx.query.params === 'string' ? JSON.parse(ctx.query.params) : ctx.query.params;
    }
    catch {
        ctx.sendResult({
            error: 'invalid_params',
            message: 'Failed to parse state query params as JSON'
        });
        return;
    }
    const action = params.action;
    // Use the tracked tab from the command context instead of querying for active tab.
    // chrome.tabs.query({ active: true, currentWindow: true }) is unreliable from a service worker.
    const tabId = ctx.tabId;
    if (!tabId) {
        ctx.sendResult({ error: 'no_tracked_tab', message: 'No tracked tab available for state command' });
        return;
    }
    try {
        let result;
        switch (action) {
            case 'capture': {
                result = await chrome.tabs.sendMessage(tabId, {
                    type: 'GASOLINE_MANAGE_STATE',
                    params: { action: 'capture' }
                });
                break;
            }
            case 'save': {
                const captureResult = (await chrome.tabs.sendMessage(tabId, {
                    type: 'GASOLINE_MANAGE_STATE',
                    params: { action: 'capture' }
                }));
                if (captureResult.error) {
                    ctx.sendResult({ error: captureResult.error });
                    return;
                }
                result = await saveStateSnapshot(params.name, captureResult);
                break;
            }
            case 'load': {
                const snapshot = await loadStateSnapshot(params.name);
                if (!snapshot) {
                    ctx.sendResult({
                        error: `Snapshot '${params.name}' not found`
                    });
                    return;
                }
                result = await chrome.tabs.sendMessage(tabId, {
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
        ctx.sendResult(result);
    }
    catch (err) {
        ctx.sendResult({ error: err.message });
    }
});
// =============================================================================
// EXECUTE
// =============================================================================
registerCommand('execute', async (ctx) => {
    if (!isAiWebPilotEnabled()) {
        if (ctx.query.correlation_id) {
            ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'error', null, 'ai_web_pilot_disabled');
        }
        else {
            ctx.sendResult({
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
        execParams = typeof ctx.query.params === 'string' ? JSON.parse(ctx.query.params) : ctx.query.params;
    }
    catch {
        execParams = {};
    }
    const world = execParams.world || 'auto';
    if (ctx.query.correlation_id) {
        await handleAsyncExecuteCommand(ctx.query, ctx.tabId, world, ctx.syncClient, ctx.sendAsyncResult, ctx.actionToast);
    }
    else {
        try {
            const result = await executeWithWorldRouting(ctx.tabId, ctx.query.params, world);
            ctx.sendResult(result);
        }
        catch (err) {
            ctx.sendResult({
                success: false,
                error: 'execution_failed',
                message: err.message || 'Execution failed'
            });
        }
    }
});
// =============================================================================
// PILOT COMMAND (exported for use by index.ts re-export chain)
// =============================================================================
function isContentScriptUnreachableError(err) {
    const message = err?.message || '';
    return message.includes('Receiving end does not exist') || message.includes('Could not establish connection');
}
function buildFallbackStatusMessage(status) {
    return `Error: MAIN world execution FAILED. Fallback in ISOLATED is ${status}.`;
}
function runHighlightFallback(params) {
    const selector = typeof params?.selector === 'string' && params.selector.trim().length > 0 ? params.selector : 'body';
    const rawDuration = typeof params?.duration_ms === 'number' ? params.duration_ms : 5000;
    const durationMs = Math.max(0, Math.min(30000, Math.floor(rawDuration)));
    try {
        const target = document.querySelector(selector);
        if (!target) {
            return {
                success: false,
                error: `Element not found: ${selector}`,
                selector
            };
        }
        const existing = document.getElementById('gasoline-highlighter');
        existing?.remove();
        const rect = target.getBoundingClientRect();
        const overlay = document.createElement('div');
        overlay.id = 'gasoline-highlighter';
        overlay.dataset.selector = selector;
        overlay.style.position = 'fixed';
        overlay.style.top = `${rect.top}px`;
        overlay.style.left = `${rect.left}px`;
        overlay.style.width = `${rect.width}px`;
        overlay.style.height = `${rect.height}px`;
        overlay.style.border = '4px solid red';
        overlay.style.backgroundColor = 'rgba(255, 0, 0, 0.1)';
        overlay.style.zIndex = '2147483647';
        overlay.style.pointerEvents = 'none';
        overlay.style.boxSizing = 'border-box';
        overlay.style.borderRadius = '4px';
        (document.body || document.documentElement).appendChild(overlay);
        const syncOverlay = () => {
            const element = document.querySelector(selector);
            if (!element || !overlay.isConnected)
                return;
            const bounds = element.getBoundingClientRect();
            overlay.style.top = `${bounds.top}px`;
            overlay.style.left = `${bounds.left}px`;
            overlay.style.width = `${bounds.width}px`;
            overlay.style.height = `${bounds.height}px`;
        };
        const onViewportChange = () => syncOverlay();
        window.addEventListener('scroll', onViewportChange, { passive: true });
        window.addEventListener('resize', onViewportChange, { passive: true });
        setTimeout(() => {
            window.removeEventListener('scroll', onViewportChange);
            window.removeEventListener('resize', onViewportChange);
            overlay.remove();
        }, durationMs);
        return {
            success: true,
            selector,
            duration_ms: durationMs,
            bounds: {
                x: rect.x,
                y: rect.y,
                width: rect.width,
                height: rect.height
            }
        };
    }
    catch (err) {
        return {
            success: false,
            error: 'highlight_fallback_failed',
            message: err?.message || 'Highlight fallback failed'
        };
    }
}
async function executeHighlightFallback(tabId, params, mainWorldError) {
    try {
        const execution = await chrome.scripting.executeScript({
            target: { tabId },
            world: 'ISOLATED',
            func: runHighlightFallback,
            args: [typeof params === 'object' && params ? params : {}]
        });
        const first = execution?.[0]?.result;
        const payload = first && typeof first === 'object' ? first : {};
        const success = payload.success !== false;
        const fallbackStatus = success ? 'SUCCESS' : 'ERROR';
        return {
            ...payload,
            execution_world: 'ISOLATED',
            fallback_reason: 'content_script_unreachable',
            fallback_status: fallbackStatus,
            main_world_error: mainWorldError,
            message: typeof payload.message === 'string' && payload.message.length > 0
                ? payload.message
                : buildFallbackStatusMessage(fallbackStatus)
        };
    }
    catch (err) {
        const fallbackError = err?.message || 'highlight_fallback_failed';
        return {
            success: false,
            error: 'highlight_fallback_failed',
            execution_world: 'ISOLATED',
            fallback_reason: 'content_script_unreachable',
            fallback_status: 'ERROR',
            main_world_error: mainWorldError,
            fallback_error: fallbackError,
            message: `${buildFallbackStatusMessage('ERROR')} ${fallbackError}`
        };
    }
}
async function handlePilotCommandOnTab(tabId, command, params) {
    try {
        const result = await chrome.tabs.sendMessage(tabId, {
            type: command,
            params
        });
        return result || { success: true };
    }
    catch (err) {
        if (command === 'GASOLINE_HIGHLIGHT' && isContentScriptUnreachableError(err)) {
            return executeHighlightFallback(tabId, params, err.message || 'command_failed');
        }
        throw err;
    }
}
export async function handlePilotCommand(command, params, preferredTabId) {
    if (!isAiWebPilotEnabled()) {
        return { error: 'ai_web_pilot_disabled' };
    }
    try {
        if (typeof preferredTabId === 'number' && Number.isFinite(preferredTabId) && preferredTabId > 0) {
            return await handlePilotCommandOnTab(preferredTabId, command, params);
        }
        const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
        const firstTab = tabs[0];
        const tabId = firstTab?.id;
        if (!tabId) {
            return { error: 'no_active_tab' };
        }
        return await handlePilotCommandOnTab(tabId, command, params);
    }
    catch (err) {
        return { error: err.message || 'command_failed' };
    }
}
//# sourceMappingURL=interact.js.map
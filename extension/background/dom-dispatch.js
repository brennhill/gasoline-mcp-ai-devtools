/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
import { domFrameProbe } from './dom-frame-probe.js';
import { domPrimitive } from './dom-primitives.js';
import { domPrimitiveListInteractive } from './dom-primitives-list-interactive.js';
const FALLBACK_SUCCESS_SUMMARY = 'Error: MAIN world execution FAILED. Fallback in ISOLATED is SUCCESS.';
const FALLBACK_ERROR_SUMMARY = 'Error: MAIN world execution FAILED. Fallback in ISOLATED is ERROR.';
class WorldExecutionError extends Error {
    payload;
    constructor(payload, message) {
        super(message);
        this.payload = payload;
        this.name = 'WorldExecutionError';
    }
}
function parseDOMParams(query) {
    try {
        return typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
    }
    catch {
        return null;
    }
}
function isReadOnlyAction(action) {
    return action === 'list_interactive' || action.startsWith('get_');
}
function normalizeWorldMode(world) {
    if (world === 'main' || world === 'isolated' || world === 'auto') {
        return world;
    }
    return 'auto';
}
function normalizeFrameTarget(frame) {
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
async function resolveExecutionTarget(tabId, frame) {
    const normalized = normalizeFrameTarget(frame);
    if (normalized === null) {
        throw new Error('invalid_frame');
    }
    if (normalized === undefined || normalized === 'all') {
        return { tabId, allFrames: true };
    }
    const probeResults = await chrome.scripting.executeScript({
        target: { tabId, allFrames: true },
        world: 'MAIN',
        func: domFrameProbe,
        args: [normalized]
    });
    const frameIds = Array.from(new Set(probeResults
        .filter((r) => !!r.result?.matches)
        .map((r) => r.frameId)
        .filter((id) => typeof id === 'number')));
    if (frameIds.length === 0) {
        throw new Error('frame_not_found');
    }
    return { tabId, frameIds };
}
/** Pick the best result from multi-frame executeScript. Prefers main frame, falls back to first success. */
function pickFrameResult(results) {
    const mainFrame = results.find((r) => r.frameId === 0);
    if (mainFrame?.result && mainFrame.result.success) {
        return { result: mainFrame.result, frameId: 0 };
    }
    for (const r of results) {
        if (r.result && r.result.success) {
            return { result: r.result, frameId: r.frameId };
        }
    }
    if (mainFrame?.result)
        return { result: mainFrame.result, frameId: 0 };
    return results[0] ? { result: results[0].result, frameId: results[0].frameId } : null;
}
/** Merge list_interactive results from all frames (up to 100 elements). */
function mergeListInteractive(results) {
    const elements = [];
    for (const r of results) {
        const res = r.result;
        if (res?.elements)
            elements.push(...res.elements);
        if (elements.length >= 100)
            break;
    }
    return { success: true, elements: elements.slice(0, 100) };
}
const WAIT_FOR_POLL_INTERVAL_MS = 80;
function toDOMResult(value) {
    if (!value || typeof value !== 'object')
        return null;
    const candidate = value;
    if (typeof candidate.success !== 'boolean')
        return null;
    if (typeof candidate.action !== 'string' || typeof candidate.selector !== 'string')
        return null;
    return candidate;
}
function withTimeoutResult(results, selector, timeoutMs) {
    const timeoutResult = {
        success: false,
        action: 'wait_for',
        selector,
        error: 'timeout',
        message: `Element not found within ${timeoutMs}ms: ${selector}`
    };
    if (results.length === 0) {
        return [{ frameId: 0, result: timeoutResult }];
    }
    return results.map((result) => ({ ...result, result: timeoutResult }));
}
function wait(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}
async function executeWaitFor(target, params, world) {
    const selector = params.selector || '';
    const timeoutMs = Math.max(1, params.timeout_ms || 5000);
    const startedAt = Date.now();
    const quickCheck = await chrome.scripting.executeScript({
        target,
        world,
        func: domPrimitive,
        args: [params.action, selector, { timeout_ms: timeoutMs }]
    });
    const quickPicked = pickFrameResult(quickCheck);
    const quickResult = toDOMResult(quickPicked?.result);
    if (quickResult?.success) {
        return quickResult;
    }
    let lastResults = quickCheck;
    while (Date.now() - startedAt < timeoutMs) {
        await wait(Math.min(WAIT_FOR_POLL_INTERVAL_MS, timeoutMs));
        const probeResults = await chrome.scripting.executeScript({
            target,
            world,
            func: domPrimitive,
            args: [params.action, selector, { timeout_ms: timeoutMs }]
        });
        lastResults = probeResults;
        const picked = pickFrameResult(probeResults);
        const result = toDOMResult(picked?.result);
        if (result?.success) {
            return probeResults;
        }
    }
    return withTimeoutResult(lastResults, selector, timeoutMs);
}
async function executeStandardAction(target, params, world) {
    return chrome.scripting.executeScript({
        target,
        world,
        func: domPrimitive,
        args: [
            params.action,
            params.selector || '',
            {
                text: params.text,
                value: params.value,
                clear: params.clear,
                checked: params.checked,
                name: params.name,
                timeout_ms: params.timeout_ms,
                analyze: params.analyze,
                observe_mutations: params.observe_mutations
            }
        ]
    });
}
async function executeListInteractive(target, world) {
    return chrome.scripting.executeScript({
        target,
        world,
        func: domPrimitiveListInteractive
    });
}
function baseWorldMeta(mode) {
    if (mode === 'main') {
        return {
            execution_world: 'main',
            fallback_attempted: false,
            main_world_status: 'not_attempted',
            isolated_world_status: 'not_attempted',
            fallback_summary: 'MAIN world execution mode selected.'
        };
    }
    if (mode === 'isolated') {
        return {
            execution_world: 'isolated',
            fallback_attempted: false,
            main_world_status: 'not_attempted',
            isolated_world_status: 'not_attempted',
            fallback_summary: 'ISOLATED world execution mode selected.'
        };
    }
    return {
        execution_world: 'main',
        fallback_attempted: false,
        main_world_status: 'not_attempted',
        isolated_world_status: 'not_attempted',
        fallback_summary: 'AUTO world execution mode selected.'
    };
}
function attachWorldMeta(result, meta) {
    if (result && typeof result === 'object' && !Array.isArray(result)) {
        return { ...result, ...meta };
    }
    return { value: result ?? null, ...meta };
}
async function executeByWorld(target, params, world) {
    if (params.action === 'list_interactive') {
        return executeListInteractive(target, world);
    }
    if (params.action === 'wait_for') {
        return executeWaitFor(target, params, world);
    }
    return executeStandardAction(target, params, world);
}
async function executeWithWorldMode(target, params, mode) {
    const meta = baseWorldMeta(mode);
    const run = async (world) => executeByWorld(target, params, world);
    if (mode === 'main') {
        try {
            const rawResult = await run('MAIN');
            meta.main_world_status = 'success';
            meta.execution_world = 'main';
            return { rawResult, meta };
        }
        catch (err) {
            meta.main_world_status = 'error';
            meta.main_world_error = err?.message || 'main_world_execution_failed';
            throw new WorldExecutionError({
                success: false,
                action: params.action || 'unknown',
                selector: params.selector || '',
                error: 'main_world_execution_failed',
                message: meta.main_world_error,
                ...meta
            }, meta.main_world_error);
        }
    }
    if (mode === 'isolated') {
        try {
            const rawResult = await run('ISOLATED');
            meta.isolated_world_status = 'success';
            meta.execution_world = 'isolated';
            return { rawResult, meta };
        }
        catch (err) {
            meta.isolated_world_status = 'error';
            meta.isolated_world_error = err?.message || 'isolated_world_execution_failed';
            throw new WorldExecutionError({
                success: false,
                action: params.action || 'unknown',
                selector: params.selector || '',
                error: 'isolated_world_execution_failed',
                message: meta.isolated_world_error,
                ...meta
            }, meta.isolated_world_error);
        }
    }
    try {
        const rawResult = await run('MAIN');
        meta.main_world_status = 'success';
        meta.execution_world = 'main';
        meta.fallback_summary = 'MAIN world execution succeeded. Fallback not attempted.';
        return { rawResult, meta };
    }
    catch (mainErr) {
        meta.fallback_attempted = true;
        meta.main_world_status = 'error';
        meta.main_world_error = mainErr?.message || 'main_world_execution_failed';
        try {
            const rawResult = await run('ISOLATED');
            meta.isolated_world_status = 'success';
            meta.execution_world = 'isolated';
            meta.fallback_summary = FALLBACK_SUCCESS_SUMMARY;
            return { rawResult, meta };
        }
        catch (isolatedErr) {
            meta.isolated_world_status = 'error';
            meta.isolated_world_error = isolatedErr?.message || 'isolated_world_execution_failed';
            meta.execution_world = 'isolated';
            meta.fallback_summary = FALLBACK_ERROR_SUMMARY;
            throw new WorldExecutionError({
                success: false,
                action: params.action || 'unknown',
                selector: params.selector || '',
                error: 'dom_world_fallback_failed',
                message: FALLBACK_ERROR_SUMMARY,
                ...meta
            }, FALLBACK_ERROR_SUMMARY);
        }
    }
}
function sendToastForResult(tabId, readOnly, result, actionToast, toastLabel, toastDetail) {
    if (readOnly)
        return;
    if (result.success) {
        actionToast(tabId, toastLabel, toastDetail, 'success');
    }
    else {
        actionToast(tabId, toastLabel, result.error || 'failed', 'error');
    }
}
// Enrich results with effective tab context (post-execution URL).
// Agents compare resolved_url (dispatch time) vs effective_url (execution time) to detect drift.
async function enrichWithEffectiveContext(tabId, result) {
    try {
        const tab = await chrome.tabs.get(tabId);
        if (result && typeof result === 'object' && !Array.isArray(result)) {
            return { ...result, effective_tab_id: tabId, effective_url: tab.url, effective_title: tab.title };
        }
        return result;
    }
    catch {
        return result;
    }
}
// #lizard forgives
export async function executeDOMAction(query, tabId, syncClient, sendAsyncResult, actionToast) {
    const params = parseDOMParams(query);
    if (!params) {
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'invalid_params');
        return;
    }
    const { action, selector, reason } = params;
    if (!action) {
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'missing_action');
        return;
    }
    if (action === 'wait_for' && !selector) {
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'missing_selector');
        return;
    }
    const toastLabel = reason || action;
    const toastDetail = reason ? undefined : selector || 'page';
    const readOnly = isReadOnlyAction(action);
    const worldMode = normalizeWorldMode(params.world);
    try {
        const executionTarget = await resolveExecutionTarget(tabId, params.frame);
        const tryingShownAt = Date.now();
        if (!readOnly)
            actionToast(tabId, toastLabel, toastDetail, 'trying', 10000);
        const { rawResult, meta } = await executeWithWorldMode(executionTarget, params, worldMode);
        // wait_for quick-check can return a DOMResult directly
        if (!Array.isArray(rawResult)) {
            if (!readOnly)
                actionToast(tabId, toastLabel, toastDetail, 'success');
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', await enrichWithEffectiveContext(tabId, attachWorldMeta(rawResult, meta)));
            return;
        }
        // Ensure "trying" toast is visible for at least 500ms
        const MIN_TOAST_MS = 500;
        const elapsed = Date.now() - tryingShownAt;
        if (!readOnly && elapsed < MIN_TOAST_MS)
            await new Promise((r) => setTimeout(r, MIN_TOAST_MS - elapsed));
        // list_interactive: merge elements from all frames
        if (action === 'list_interactive') {
            const merged = attachWorldMeta(mergeListInteractive(rawResult), meta);
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', await enrichWithEffectiveContext(tabId, merged));
            return;
        }
        const picked = pickFrameResult(rawResult);
        const firstResult = picked?.result;
        if (firstResult && typeof firstResult === 'object') {
            const resultPayload = params.frame !== undefined && params.frame !== null && picked
                ? { ...firstResult, frame_id: picked.frameId }
                : firstResult;
            const resultWithWorldMeta = attachWorldMeta(resultPayload, meta);
            sendToastForResult(tabId, readOnly, resultWithWorldMeta, actionToast, toastLabel, toastDetail);
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', await enrichWithEffectiveContext(tabId, resultWithWorldMeta));
        }
        else {
            if (!readOnly)
                actionToast(tabId, toastLabel, 'no result', 'error');
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'no_result');
        }
    }
    catch (err) {
        if (err instanceof WorldExecutionError) {
            actionToast(tabId, action, err.message, 'error');
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', await enrichWithEffectiveContext(tabId, err.payload), err.message);
            return;
        }
        actionToast(tabId, action, err.message, 'error');
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, err.message);
    }
}
//# sourceMappingURL=dom-dispatch.js.map
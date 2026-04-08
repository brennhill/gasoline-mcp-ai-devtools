/**
 * Purpose: Shared infrastructure for command dispatch -- result helpers, target tab resolution, action toast, and type aliases.
 */
import { sendTabToast } from '../tab-state.js';
import { DebugCategory } from '../debug.js';
import { isAiWebPilotEnabled } from '../state.js';
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js';
import { errorMessage } from '../../lib/error-utils.js';
// Re-export target resolution symbols so existing consumers keep working
export { withTargetContext, requiresTargetTab, isBrowserEscapeAction, persistTrackedTab, resolveTargetTab, isRestrictedUrl } from './target-resolution.js';
export function debugLog(category, message, data = null) {
    const globalLogger = globalThis
        .__KABOOM_DEBUG_LOG__;
    if (typeof globalLogger === 'function') {
        globalLogger(category, message, data);
        return;
    }
    // Keep helpers usable before the main debug logger is initialized.
    const debugEnabled = globalThis.__KABOOM_REGISTRY_DEBUG__ === true;
    if (!debugEnabled)
        return;
    const prefix = `${KABOOM_LOG_PREFIX.slice(0, -1)}:${category}]`;
    if (data === null) {
        console.debug(`${prefix} ${message}`);
        return;
    }
    console.debug(`${prefix} ${message}`, data);
}
function diagnosticLog(message) {
    debugLog(DebugCategory.CONNECTION, message);
}
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
    wait_for_stable: 'Waiting for page to stabilize...',
    key_press: 'Key press',
    highlight: 'Highlight',
    subtitle: 'Subtitle',
    upload: 'Upload file'
};
const PRETTY_TRYING_LABELS = {
    scroll_to: 'Scrolling to',
    open_composer: 'Opening composer',
    submit_active_composer: 'Submitting active composer',
    confirm_top_dialog: 'Confirming top dialog',
    dismiss_top_overlay: 'Dismissing top overlay',
    auto_dismiss_overlays: 'Dismissing overlays'
};
function humanizeActionLabel(action) {
    const explicit = PRETTY_LABELS[action];
    if (explicit)
        return explicit;
    if (!/^[a-z0-9]+(?:_[a-z0-9]+)+$/.test(action))
        return action;
    const sentence = action.replaceAll('_', ' ');
    return sentence.charAt(0).toUpperCase() + sentence.slice(1);
}
function inferWaitTarget(detail) {
    if (!detail)
        return undefined;
    const trimmed = detail.trim();
    if (!trimmed || trimmed.toLowerCase() === 'page')
        return undefined;
    return trimmed;
}
function resolveToastCopy(action, detail, state) {
    if (state !== 'trying')
        return { text: humanizeActionLabel(action), detail };
    if (action === 'wait_for') {
        const waitTarget = inferWaitTarget(detail);
        if (waitTarget)
            return { text: `Waiting for ${waitTarget}` };
        return { text: 'Waiting for condition...' };
    }
    const tryingText = PRETTY_TRYING_LABELS[action];
    if (tryingText)
        return { text: tryingText, detail };
    return { text: humanizeActionLabel(action), detail };
}
/** Show a visual action toast on the tracked tab */
export function actionToast(tabId, action, detail, state = 'success', durationMs = 3000) {
    const toastCopy = resolveToastCopy(action, detail, state);
    sendTabToast(tabId, toastCopy.text, toastCopy.detail ?? '', state, durationMs);
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
// Target resolution extracted to ./target-resolution.ts (re-exported above)
// =============================================================================
// CONTENT SCRIPT ERROR DETECTION
// =============================================================================
/** Check if an error indicates the content script is not loaded on the target page. */
export function isContentScriptUnreachableError(err) {
    const message = errorMessage(err, '');
    return message.includes('Receiving end does not exist') || message.includes('Could not establish connection');
}
/**
 * Guard that checks AI Web Pilot is enabled.
 * Returns true if enabled and the caller should proceed.
 * Returns false if disabled — the error response has already been sent.
 */
export function requireAiWebPilot(ctx) {
    if (isAiWebPilotEnabled())
        return true;
    ctx.sendResult({ error: 'ai_web_pilot_disabled' });
    return false;
}
//# sourceMappingURL=helpers.js.map
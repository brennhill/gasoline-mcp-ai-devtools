/**
 * @fileoverview Message bridge for posting log events to the content script.
 * Enriches error-level messages with context annotations and user action replay.
 */
import { getContextAnnotations } from './context.js';
import { getActionBuffer } from './actions.js';
/**
 * Post a log message to the content script
 */
export function postLog(payload) {
    // Include context annotations and action replay for errors
    const context = getContextAnnotations();
    const actions = payload.level === 'error' ? getActionBuffer() : null;
    // Build enrichments list to help AI understand what data is attached
    const enrichments = [];
    if (context && payload.level === 'error')
        enrichments.push('context');
    if (actions && actions.length > 0)
        enrichments.push('userActions');
    window.postMessage({
        type: 'GASOLINE_LOG',
        payload: {
            ts: new Date().toISOString(),
            url: window.location.href,
            // Extract message from multiple possible sources
            message: payload.message ||
                payload.error ||
                (payload.args?.[0] !== null && payload.args?.[0] !== undefined ? String(payload.args[0]) : ''),
            // Derive source from url (filename:line if available)
            source: payload.filename ? `${payload.filename}:${payload.lineno || 0}` : '',
            ...(enrichments.length > 0 ? { _enrichments: enrichments } : {}),
            ...(context && payload.level === 'error' ? { _context: context } : {}),
            ...(actions && actions.length > 0 ? { _actions: actions } : {}),
            ...payload, // Allow payload to override defaults like url
        },
    }, '*');
}
//# sourceMappingURL=bridge.js.map
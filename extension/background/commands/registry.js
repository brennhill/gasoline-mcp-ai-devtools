// registry.ts — Command registry and dispatch loop.
// Replaces the monolithic if-chain in pending-queries.ts with a Map-based registry.
import * as index from '../index.js';
import { DebugCategory } from '../debug.js';
import { sendResult, sendAsyncResult, requiresTargetTab, resolveTargetTab, parseQueryParamsObject, withTargetContext, actionToast, isRestrictedUrl } from './helpers.js';
const { debugLog } = index;
// =============================================================================
// REGISTRY
// =============================================================================
const handlers = new Map();
export function registerCommand(type, handler) {
    handlers.set(type, handler);
}
// =============================================================================
// DISPATCH
// =============================================================================
export async function dispatch(query, syncClient) {
    // Wait for initialization to complete (max 2s) so pilot cache is populated
    await Promise.race([index.initReady, new Promise((r) => setTimeout(r, 2000))]);
    debugLog(DebugCategory.CONNECTION, 'handlePendingQuery ENTER', {
        id: query.id,
        type: query.type,
        correlation_id: query.correlation_id || null,
        hasSyncClient: !!syncClient
    });
    // Normalize state_* types to a wildcard key
    let queryType = query.type;
    if (queryType.startsWith('state_')) {
        queryType = 'state_*';
    }
    const handler = handlers.get(queryType);
    if (!handler) {
        debugLog(DebugCategory.CONNECTION, 'Unknown query type', { type: query.type });
        sendResult(syncClient, query.id, {
            error: 'unknown_query_type',
            message: `Unknown query type: ${query.type}`
        });
        return;
    }
    // Target resolution
    let target;
    const paramsObj = parseQueryParamsObject(query.params);
    const needsTarget = requiresTargetTab(query.type);
    if (needsTarget) {
        const resolved = await resolveTargetTab(query, paramsObj);
        if (resolved.error) {
            if (query.correlation_id) {
                sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', resolved.error.payload, resolved.error.message);
            }
            else {
                sendResult(syncClient, query.id, resolved.error.payload);
            }
            return;
        }
        target = resolved.target;
    }
    const tabId = target?.tabId ?? 0;
    if (needsTarget && !tabId) {
        const payload = {
            success: false,
            error: 'missing_target',
            message: 'No target tab resolved for query'
        };
        if (query.correlation_id) {
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', payload, payload.message);
        }
        else {
            sendResult(syncClient, query.id, payload);
        }
        return;
    }
    // Restricted page detection: content scripts cannot run on internal browser pages
    if (needsTarget && isRestrictedUrl(target?.url)) {
        const payload = {
            success: false,
            error: 'restricted_page',
            message: 'Extension connected but this page blocks content scripts (common on Google, Chrome Web Store, internal pages). Navigate to a different page first.',
            retryable: false
        };
        if (query.correlation_id) {
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', payload, payload.message);
        }
        else {
            sendResult(syncClient, query.id, payload);
        }
        return;
    }
    // Build result wrappers that include target context
    const wrapResult = (result) => {
        if (!target)
            return result;
        return withTargetContext(result, target);
    };
    const wrappedSendResult = (result) => {
        sendResult(syncClient, query.id, wrapResult(result));
    };
    const wrappedSendAsyncResult = (client, queryId, correlationId, status, result, error) => {
        sendAsyncResult(client, queryId, correlationId, status, wrapResult(result), error);
    };
    const ctx = {
        query,
        syncClient,
        tabId,
        params: paramsObj,
        target,
        sendResult: wrappedSendResult,
        sendAsyncResult: wrappedSendAsyncResult,
        actionToast
    };
    try {
        await handler(ctx);
    }
    catch (err) {
        const errMsg = err.message || 'Unexpected error handling query';
        debugLog(DebugCategory.CONNECTION, 'Error handling pending query', {
            type: query.type,
            id: query.id,
            error: errMsg
        });
        if (query.correlation_id) {
            wrappedSendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, errMsg);
        }
        else {
            wrappedSendResult({ error: 'query_handler_error', message: errMsg });
        }
    }
}
//# sourceMappingURL=registry.js.map
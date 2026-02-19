// batcher-instances.ts — Concrete batcher instances for each data type.
// Creates log, WebSocket, enhanced-action, network-body, and performance batchers,
// each wired to the shared circuit breaker and connection-status tracking.
import * as communication from './communication.js';
import * as stateManager from './state-manager.js';
// =============================================================================
// CONNECTION STATUS WRAPPER
// =============================================================================
function withConnectionStatus(deps, sendFn, onSuccess) {
    return async (entries) => {
        try {
            const result = await sendFn(entries);
            deps.setConnectionStatus({ connected: true });
            if (onSuccess)
                onSuccess(entries, result);
            communication.updateBadge(deps.getConnectionStatus());
            return result;
        }
        catch (err) {
            deps.setConnectionStatus({ connected: false });
            communication.updateBadge(deps.getConnectionStatus());
            throw err;
        }
    };
}
/**
 * Create all batcher instances wired to the shared circuit breaker.
 * Called once from index.ts during module initialization.
 */
export function createBatcherInstances(deps, sharedCircuitBreaker) {
    const logBatcherWithCB = communication.createBatcherWithCircuitBreaker(withConnectionStatus(deps, (entries) => {
        stateManager.checkContextAnnotations(entries);
        return communication.sendLogsToServer(deps.getServerUrl(), entries, deps.debugLog);
    }, (entries, result) => {
        const typedResult = result;
        const status = deps.getConnectionStatus();
        deps.setConnectionStatus({
            entries: typedResult.entries || status.entries + entries.length,
            errorCount: status.errorCount + entries.filter((e) => e.level === 'error').length
        });
    }), { sharedCircuitBreaker });
    const wsBatcherWithCB = communication.createBatcherWithCircuitBreaker(withConnectionStatus(deps, (events) => communication.sendWSEventsToServer(deps.getServerUrl(), events, deps.debugLog)), { debounceMs: 200, maxBatchSize: 100, sharedCircuitBreaker });
    const enhancedActionBatcherWithCB = communication.createBatcherWithCircuitBreaker(withConnectionStatus(deps, (actions) => communication.sendEnhancedActionsToServer(deps.getServerUrl(), actions, deps.debugLog)), { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker });
    const networkBodyBatcherWithCB = communication.createBatcherWithCircuitBreaker(withConnectionStatus(deps, (bodies) => communication.sendNetworkBodiesToServer(deps.getServerUrl(), bodies, deps.debugLog)), { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker });
    const perfBatcherWithCB = communication.createBatcherWithCircuitBreaker(withConnectionStatus(deps, (snapshots) => communication.sendPerformanceSnapshotsToServer(deps.getServerUrl(), snapshots, deps.debugLog)), { debounceMs: 500, maxBatchSize: 10, sharedCircuitBreaker });
    return {
        logBatcherWithCB,
        logBatcher: logBatcherWithCB.batcher,
        wsBatcherWithCB,
        wsBatcher: wsBatcherWithCB.batcher,
        enhancedActionBatcherWithCB,
        enhancedActionBatcher: enhancedActionBatcherWithCB.batcher,
        networkBodyBatcherWithCB,
        networkBodyBatcher: networkBodyBatcherWithCB.batcher,
        perfBatcherWithCB,
        perfBatcher: perfBatcherWithCB.batcher
    };
}
//# sourceMappingURL=batcher-instances.js.map
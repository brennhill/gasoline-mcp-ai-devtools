/**
 * @fileoverview Circuit Breaker - Implements circuit breaker pattern with
 * exponential backoff for protecting server communication.
 */
/**
 * Circuit breaker with exponential backoff for server communication.
 * Prevents the extension from hammering a down/slow server.
 */
export function createCircuitBreaker(sendFn, options = {}) {
    const maxFailures = options.maxFailures ?? 5;
    const resetTimeout = options.resetTimeout ?? 30000;
    const initialBackoff = options.initialBackoff ?? 1000;
    const maxBackoff = options.maxBackoff ?? 30000;
    let state = 'closed';
    let consecutiveFailures = 0;
    let totalFailures = 0;
    let totalSuccesses = 0;
    let currentBackoff = 0;
    let lastFailureTime = 0;
    let probeInFlight = false;
    function getState() {
        if (state === 'open' && Date.now() - lastFailureTime >= resetTimeout) {
            state = 'half-open';
        }
        return state;
    }
    function getStats() {
        return {
            state: getState(),
            consecutiveFailures,
            totalFailures,
            totalSuccesses,
            currentBackoff,
        };
    }
    function reset() {
        state = 'closed';
        consecutiveFailures = 0;
        currentBackoff = 0;
        probeInFlight = false;
    }
    function onSuccess() {
        consecutiveFailures = 0;
        currentBackoff = 0;
        totalSuccesses++;
        state = 'closed';
        probeInFlight = false;
    }
    function onFailure() {
        consecutiveFailures++;
        totalFailures++;
        lastFailureTime = Date.now();
        probeInFlight = false;
        if (consecutiveFailures >= maxFailures) {
            state = 'open';
        }
        if (consecutiveFailures > 1) {
            currentBackoff = Math.min(initialBackoff * Math.pow(2, consecutiveFailures - 2), maxBackoff);
        }
        else {
            currentBackoff = 0;
        }
    }
    async function execute(args) {
        const currentState = getState();
        if (currentState === 'open') {
            throw new Error('Circuit breaker is open');
        }
        if (currentState === 'half-open') {
            if (probeInFlight) {
                throw new Error('Circuit breaker is open');
            }
            probeInFlight = true;
        }
        if (currentBackoff > 0) {
            await new Promise((r) => {
                setTimeout(r, currentBackoff);
            });
        }
        try {
            const result = (await sendFn(args));
            onSuccess();
            return result;
        }
        catch (err) {
            onFailure();
            throw err;
        }
    }
    function recordFailure() {
        consecutiveFailures++;
        totalFailures++;
        lastFailureTime = Date.now();
        if (consecutiveFailures >= maxFailures) {
            state = 'open';
        }
        currentBackoff =
            consecutiveFailures >= 2 ? Math.min(initialBackoff * Math.pow(2, consecutiveFailures - 2), maxBackoff) : 0;
    }
    return { execute, getState, getStats, reset, recordFailure };
}
//# sourceMappingURL=circuit-breaker.js.map
/**
 * @fileoverview Connection State Machine - Manages extension-server connection lifecycle
 * Implements formal state machine with invariant enforcement for reliable connection handling.
 */
// =============================================================================
// CONNECTION STATE MACHINE
// =============================================================================
export class ConnectionStateMachine {
    state;
    listeners = [];
    violations = [];
    transitionHistory = [];
    maxHistorySize = 50;
    constructor() {
        this.state = this.getInitialState();
    }
    getInitialState() {
        return {
            server: 'down',
            extension: 'disconnected',
            circuit: 'closed',
            polling: 'stopped',
            pilot: 'disabled',
            tracking: 'none',
            commands: 'none',
            lastHealthCheck: 0,
            lastSuccessfulPoll: 0,
            lastStateChange: Date.now(),
        };
    }
    /**
     * Get current state (immutable copy)
     */
    getState() {
        return { ...this.state };
    }
    /**
     * Register a state change listener
     */
    onStateChange(callback) {
        this.listeners.push(callback);
        return () => {
            const index = this.listeners.indexOf(callback);
            if (index > -1)
                this.listeners.splice(index, 1);
        };
    }
    /**
     * Get recent transition history for debugging
     */
    getTransitionHistory() {
        return [...this.transitionHistory];
    }
    /**
     * Get invariant violations for debugging
     */
    getViolations() {
        return [...this.violations];
    }
    /**
     * Process an event and transition to new state
     */
    transition(event) {
        const oldState = { ...this.state };
        const newState = this.computeNextState(oldState, event);
        // Record transition
        this.transitionHistory.push({ event, timestamp: Date.now() });
        if (this.transitionHistory.length > this.maxHistorySize) {
            this.transitionHistory.shift();
        }
        // Check if state actually changed
        if (this.statesEqual(oldState, newState)) {
            return newState;
        }
        // Update state
        this.state = {
            ...newState,
            lastStateChange: Date.now(),
        };
        // Enforce invariants (may modify state)
        this.enforceInvariants();
        // Notify listeners
        for (const listener of this.listeners) {
            try {
                listener(oldState, this.state, event);
            }
            catch (err) {
                console.error('[ConnectionStateMachine] Listener error:', err);
            }
        }
        return this.state;
    }
    /**
     * Compute the next state based on current state and event
     */
    computeNextState(state, event) {
        const next = { ...state };
        switch (event.type) {
            // Server events
            case 'SERVER_UP':
                next.server = 'up';
                break;
            case 'SERVER_DOWN':
                next.server = 'down';
                next.extension = 'disconnected';
                next.polling = 'stopped';
                break;
            case 'SERVER_BOOTING':
                next.server = 'booting';
                next.extension = 'disconnected';
                break;
            // Health check events
            case 'HEALTH_OK':
                next.lastHealthCheck = Date.now();
                if (next.server !== 'up')
                    next.server = 'up';
                if (next.extension === 'disconnected')
                    next.extension = 'connected';
                break;
            case 'HEALTH_FAIL':
                next.lastHealthCheck = Date.now();
                if (next.extension !== 'disconnected') {
                    next.extension = 'disconnected';
                    next.polling = 'stopped';
                }
                break;
            // Polling events
            case 'POLLING_STARTED':
                next.polling = 'running';
                if (next.extension === 'connected')
                    next.extension = 'active';
                break;
            case 'POLLING_STOPPED':
                next.polling = 'stopped';
                if (next.extension === 'active')
                    next.extension = 'connected';
                break;
            case 'POLL_SUCCESS':
                next.lastSuccessfulPoll = Date.now();
                break;
            case 'POLL_FAIL':
                // Record failure but don't immediately disconnect
                break;
            case 'POLL_STALE':
                // Stale polling detected - force reconnection check
                next.extension = 'connected'; // Downgrade from active
                next.polling = 'stopped';
                break;
            // Circuit breaker events
            case 'CB_OPENED':
                next.circuit = 'open';
                next.polling = 'stopped';
                break;
            case 'CB_HALF_OPEN':
                next.circuit = 'half-open';
                break;
            case 'CB_CLOSED':
                next.circuit = 'closed';
                break;
            case 'CB_PROBE_SUCCESS':
                next.circuit = 'closed';
                break;
            case 'CB_PROBE_FAIL':
                next.circuit = 'open';
                break;
            // User action events - reset circuit breaker
            case 'USER_RESET':
                next.circuit = 'closed';
                break;
            case 'PILOT_ENABLED':
                next.pilot = 'enabled';
                next.circuit = 'closed'; // Reset CB on user action
                break;
            case 'PILOT_DISABLED':
                next.pilot = 'disabled';
                break;
            case 'TRACKING_ENABLED':
                next.tracking = 'tab_tracked';
                next.circuit = 'closed'; // Reset CB on user action
                break;
            case 'TRACKING_DISABLED':
                next.tracking = 'none';
                break;
            // Command events
            case 'COMMAND_QUEUED':
                if (next.commands === 'none')
                    next.commands = 'queued';
                break;
            case 'COMMAND_PROCESSING':
                next.commands = 'processing';
                break;
            case 'COMMAND_COMPLETED':
            case 'COMMAND_TIMEOUT':
                next.commands = 'none';
                break;
        }
        return next;
    }
    /**
     * Enforce invariants, fixing any violations
     */
    enforceInvariants() {
        // INV-1: server=down → extension=disconnected
        if (this.state.server === 'down' && this.state.extension !== 'disconnected') {
            this.recordViolation('INV-1', 'extension=disconnected when server=down', `extension=${this.state.extension}`);
            this.state.extension = 'disconnected';
            this.state.polling = 'stopped';
        }
        // INV-2: extension=disconnected → polling=stopped
        if (this.state.extension === 'disconnected' && this.state.polling !== 'stopped') {
            this.recordViolation('INV-2', 'polling=stopped when extension=disconnected', `polling=${this.state.polling}`);
            this.state.polling = 'stopped';
        }
        // INV-3: extension=active → polling=running
        if (this.state.extension === 'active' && this.state.polling !== 'running') {
            this.recordViolation('INV-3', 'polling=running when extension=active', `polling=${this.state.polling}`);
            // Downgrade to connected instead of forcing polling
            this.state.extension = 'connected';
        }
        // INV-4: circuit=open implies requests should be blocked (informational, no fix needed)
        // INV-5: commands=processing → extension=active
        if (this.state.commands === 'processing' && this.state.extension !== 'active') {
            this.recordViolation('INV-5', 'extension=active when commands=processing', `extension=${this.state.extension}`);
            // Commands timeout naturally, just record violation
            this.state.commands = 'none';
        }
        // INV-6: server=booting → extension=disconnected
        if (this.state.server === 'booting' && this.state.extension !== 'disconnected') {
            this.recordViolation('INV-6', 'extension=disconnected when server=booting', `extension=${this.state.extension}`);
            this.state.extension = 'disconnected';
            this.state.polling = 'stopped';
        }
    }
    /**
     * Record an invariant violation
     */
    recordViolation(invariant, expected, actual) {
        console.warn(`[ConnectionStateMachine] Invariant violation: ${invariant} - expected ${expected}, got ${actual}`);
        this.violations.push({
            invariant,
            expected,
            actual,
            timestamp: Date.now(),
        });
        // Keep only last 20 violations
        if (this.violations.length > 20) {
            this.violations.shift();
        }
    }
    /**
     * Check if two states are equal
     */
    statesEqual(a, b) {
        return (a.server === b.server &&
            a.extension === b.extension &&
            a.circuit === b.circuit &&
            a.polling === b.polling &&
            a.pilot === b.pilot &&
            a.tracking === b.tracking &&
            a.commands === b.commands);
    }
    /**
     * Check if polling is stale (no successful poll in threshold time)
     */
    isPollingStale(thresholdMs = 5000) {
        if (this.state.polling !== 'running')
            return false;
        if (this.state.lastSuccessfulPoll === 0)
            return false;
        return Date.now() - this.state.lastSuccessfulPoll > thresholdMs;
    }
    /**
     * Check if health check is stale
     */
    isHealthStale(thresholdMs = 10000) {
        if (this.state.lastHealthCheck === 0)
            return true;
        return Date.now() - this.state.lastHealthCheck > thresholdMs;
    }
    /**
     * Get a human-readable summary of current state
     */
    getSummary() {
        const s = this.state;
        return `server=${s.server}, ext=${s.extension}, cb=${s.circuit}, poll=${s.polling}, pilot=${s.pilot}, track=${s.tracking}`;
    }
    /**
     * Reset to initial state (for testing or catastrophic recovery)
     */
    reset() {
        const oldState = { ...this.state };
        this.state = this.getInitialState();
        this.violations = [];
        for (const listener of this.listeners) {
            try {
                listener(oldState, this.state, { type: 'USER_RESET' });
            }
            catch (err) {
                console.error('[ConnectionStateMachine] Listener error during reset:', err);
            }
        }
    }
}
// =============================================================================
// SINGLETON INSTANCE
// =============================================================================
let instance = null;
export function getConnectionStateMachine() {
    if (!instance) {
        instance = new ConnectionStateMachine();
    }
    return instance;
}
export function resetConnectionStateMachine() {
    instance = null;
}
//# sourceMappingURL=connection-state.js.map
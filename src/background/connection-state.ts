/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Connection State Machine - Manages extension-server connection lifecycle
 * Implements formal state machine with invariant enforcement for reliable connection handling.
 */

// =============================================================================
// STATE TYPES
// =============================================================================

export type ServerState = 'down' | 'booting' | 'up'
export type ExtensionState = 'disconnected' | 'connected' | 'active'
export type CircuitState = 'closed' | 'open' | 'half-open'
export type PollingState = 'stopped' | 'running'
export type PilotState = 'disabled' | 'enabled'
export type TrackingState = 'none' | 'tab_tracked'
export type CommandState = 'none' | 'queued' | 'processing'

export interface ConnectionState {
  server: ServerState
  extension: ExtensionState
  circuit: CircuitState
  polling: PollingState
  pilot: PilotState
  tracking: TrackingState
  commands: CommandState
  lastHealthCheck: number
  lastSuccessfulPoll: number
  lastStateChange: number
}

// =============================================================================
// EVENTS
// =============================================================================

export type ConnectionEvent =
  // Server events
  | { type: 'SERVER_UP' }
  | { type: 'SERVER_DOWN' }
  | { type: 'SERVER_BOOTING' }
  // Health check events
  | { type: 'HEALTH_OK' }
  | { type: 'HEALTH_FAIL'; error?: string }
  // Polling events
  | { type: 'POLLING_STARTED' }
  | { type: 'POLLING_STOPPED' }
  | { type: 'POLL_SUCCESS' }
  | { type: 'POLL_FAIL'; error?: string }
  | { type: 'POLL_STALE' }
  // Circuit breaker events
  | { type: 'CB_OPENED' }
  | { type: 'CB_HALF_OPEN' }
  | { type: 'CB_CLOSED' }
  | { type: 'CB_PROBE_SUCCESS' }
  | { type: 'CB_PROBE_FAIL' }
  // User action events
  | { type: 'USER_RESET' }
  | { type: 'PILOT_ENABLED' }
  | { type: 'PILOT_DISABLED' }
  | { type: 'TRACKING_ENABLED'; tabId: number }
  | { type: 'TRACKING_DISABLED' }
  // Command events
  | { type: 'COMMAND_QUEUED' }
  | { type: 'COMMAND_PROCESSING' }
  | { type: 'COMMAND_COMPLETED' }
  | { type: 'COMMAND_TIMEOUT' }

// =============================================================================
// STATE CHANGE CALLBACK
// =============================================================================

export type StateChangeCallback = (oldState: ConnectionState, newState: ConnectionState, event: ConnectionEvent) => void

// =============================================================================
// INVARIANT VIOLATIONS
// =============================================================================

export interface InvariantViolation {
  invariant: string
  expected: string
  actual: string
  timestamp: number
}

// =============================================================================
// CONNECTION STATE MACHINE
// =============================================================================

export class ConnectionStateMachine {
  private state: ConnectionState
  private listeners: StateChangeCallback[] = []
  private violations: InvariantViolation[] = []
  private transitionHistory: Array<{ event: ConnectionEvent; timestamp: number }> = []
  private readonly maxHistorySize = 50

  constructor() {
    this.state = this.getInitialState()
  }

  private getInitialState(): ConnectionState {
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
      lastStateChange: Date.now()
    }
  }

  /**
   * Get current state (immutable copy)
   */
  getState(): ConnectionState {
    return { ...this.state }
  }

  /**
   * Register a state change listener
   */
  onStateChange(callback: StateChangeCallback): () => void {
    this.listeners.push(callback)
    return () => {
      const index = this.listeners.indexOf(callback)
      if (index > -1) this.listeners.splice(index, 1)
    }
  }

  /**
   * Get recent transition history for debugging
   */
  getTransitionHistory(): Array<{ event: ConnectionEvent; timestamp: number }> {
    return [...this.transitionHistory]
  }

  /**
   * Get invariant violations for debugging
   */
  getViolations(): InvariantViolation[] {
    return [...this.violations]
  }

  /**
   * Process an event and transition to new state
   */
  transition(event: ConnectionEvent): ConnectionState {
    const oldState = { ...this.state }
    const newState = this.computeNextState(oldState, event)

    // Record transition
    this.transitionHistory.push({ event, timestamp: Date.now() })
    if (this.transitionHistory.length > this.maxHistorySize) {
      this.transitionHistory.shift()
    }

    // Check if state actually changed
    if (this.statesEqual(oldState, newState)) {
      return newState
    }

    // Update state
    this.state = {
      ...newState,
      lastStateChange: Date.now()
    }

    // Enforce invariants (may modify state)
    this.enforceInvariants()

    // Notify listeners
    for (const listener of this.listeners) {
      try {
        listener(oldState, this.state, event)
      } catch (err) {
        console.error('[ConnectionStateMachine] Listener error:', err)
      }
    }

    return this.state
  }

  /** Transition table: each handler mutates the state draft */
  private static readonly transitions: Record<ConnectionEvent['type'], (next: ConnectionState) => void> = {
    // Server events
    SERVER_UP: (n) => {
      n.server = 'up'
    },
    SERVER_DOWN: (n) => {
      n.server = 'down'
      n.extension = 'disconnected'
      n.polling = 'stopped'
    },
    SERVER_BOOTING: (n) => {
      n.server = 'booting'
      n.extension = 'disconnected'
    },
    // Health check events
    HEALTH_OK: (n) => {
      n.lastHealthCheck = Date.now()
      if (n.server !== 'up') n.server = 'up'
      if (n.extension === 'disconnected') n.extension = 'connected'
    },
    HEALTH_FAIL: (n) => {
      n.lastHealthCheck = Date.now()
      if (n.extension !== 'disconnected') {
        n.extension = 'disconnected'
        n.polling = 'stopped'
      }
    },
    // Polling events
    POLLING_STARTED: (n) => {
      n.polling = 'running'
      if (n.extension === 'connected') n.extension = 'active'
    },
    POLLING_STOPPED: (n) => {
      n.polling = 'stopped'
      if (n.extension === 'active') n.extension = 'connected'
    },
    POLL_SUCCESS: (n) => {
      n.lastSuccessfulPoll = Date.now()
    },
    POLL_FAIL: () => {
      /* Record failure but don't immediately disconnect */
    },
    POLL_STALE: (n) => {
      n.extension = 'connected'
      n.polling = 'stopped'
    },
    // Circuit breaker events
    CB_OPENED: (n) => {
      n.circuit = 'open'
      n.polling = 'stopped'
    },
    CB_HALF_OPEN: (n) => {
      n.circuit = 'half-open'
    },
    CB_CLOSED: (n) => {
      n.circuit = 'closed'
    },
    CB_PROBE_SUCCESS: (n) => {
      n.circuit = 'closed'
    },
    CB_PROBE_FAIL: (n) => {
      n.circuit = 'open'
    },
    // User action events
    USER_RESET: (n) => {
      n.circuit = 'closed'
    },
    PILOT_ENABLED: (n) => {
      n.pilot = 'enabled'
      n.circuit = 'closed'
    },
    PILOT_DISABLED: (n) => {
      n.pilot = 'disabled'
    },
    TRACKING_ENABLED: (n) => {
      n.tracking = 'tab_tracked'
      n.circuit = 'closed'
    },
    TRACKING_DISABLED: (n) => {
      n.tracking = 'none'
    },
    // Command events
    COMMAND_QUEUED: (n) => {
      if (n.commands === 'none') n.commands = 'queued'
    },
    COMMAND_PROCESSING: (n) => {
      n.commands = 'processing'
    },
    COMMAND_COMPLETED: (n) => {
      n.commands = 'none'
    },
    COMMAND_TIMEOUT: (n) => {
      n.commands = 'none'
    }
  }

  /**
   * Compute the next state based on current state and event
   */
  private computeNextState(state: ConnectionState, event: ConnectionEvent): ConnectionState {
    const next = { ...state }
    const handler = ConnectionStateMachine.transitions[event.type] // nosemgrep: unsafe-dynamic-method
    if (handler) handler(next)
    return next
  }

  /**
   * Enforce invariants, fixing any violations
   */
  private enforceInvariants(): void {
    // INV-1: server=down → extension=disconnected
    if (this.state.server === 'down' && this.state.extension !== 'disconnected') {
      this.recordViolation('INV-1', 'extension=disconnected when server=down', `extension=${this.state.extension}`)
      this.state.extension = 'disconnected'
      this.state.polling = 'stopped'
    }

    // INV-2: extension=disconnected → polling=stopped
    if (this.state.extension === 'disconnected' && this.state.polling !== 'stopped') {
      this.recordViolation('INV-2', 'polling=stopped when extension=disconnected', `polling=${this.state.polling}`)
      this.state.polling = 'stopped'
    }

    // INV-3: extension=active → polling=running
    if (this.state.extension === 'active' && this.state.polling !== 'running') {
      this.recordViolation('INV-3', 'polling=running when extension=active', `polling=${this.state.polling}`)
      // Downgrade to connected instead of forcing polling
      this.state.extension = 'connected'
    }

    // INV-4: circuit=open implies requests should be blocked (informational, no fix needed)

    // INV-5: commands=processing → extension=active
    if (this.state.commands === 'processing' && this.state.extension !== 'active') {
      this.recordViolation('INV-5', 'extension=active when commands=processing', `extension=${this.state.extension}`)
      // Commands timeout naturally, just record violation
      this.state.commands = 'none'
    }

    // INV-6: server=booting → extension=disconnected
    if (this.state.server === 'booting' && this.state.extension !== 'disconnected') {
      this.recordViolation('INV-6', 'extension=disconnected when server=booting', `extension=${this.state.extension}`)
      this.state.extension = 'disconnected'
      this.state.polling = 'stopped'
    }
  }

  /**
   * Record an invariant violation
   */
  private recordViolation(invariant: string, expected: string, actual: string): void {
    console.warn(`[ConnectionStateMachine] Invariant violation: ${invariant} - expected ${expected}, got ${actual}`)
    this.violations.push({
      invariant,
      expected,
      actual,
      timestamp: Date.now()
    })
    // Keep only last 20 violations
    if (this.violations.length > 20) {
      this.violations.shift()
    }
  }

  /**
   * Check if two states are equal
   */
  private statesEqual(a: ConnectionState, b: ConnectionState): boolean {
    return (
      a.server === b.server &&
      a.extension === b.extension &&
      a.circuit === b.circuit &&
      a.polling === b.polling &&
      a.pilot === b.pilot &&
      a.tracking === b.tracking &&
      a.commands === b.commands
    )
  }

  /**
   * Check if polling is stale (no successful poll in threshold time)
   */
  isPollingStale(thresholdMs: number = 5000): boolean {
    if (this.state.polling !== 'running') return false
    if (this.state.lastSuccessfulPoll === 0) return false
    return Date.now() - this.state.lastSuccessfulPoll > thresholdMs
  }

  /**
   * Check if health check is stale
   */
  isHealthStale(thresholdMs: number = 10000): boolean {
    if (this.state.lastHealthCheck === 0) return true
    return Date.now() - this.state.lastHealthCheck > thresholdMs
  }

  /**
   * Get a human-readable summary of current state
   */
  getSummary(): string {
    const s = this.state
    return `server=${s.server}, ext=${s.extension}, cb=${s.circuit}, poll=${s.polling}, pilot=${s.pilot}, track=${s.tracking}`
  }

  /**
   * Reset to initial state (for testing or catastrophic recovery)
   */
  reset(): void {
    const oldState = { ...this.state }
    this.state = this.getInitialState()
    this.violations = []

    for (const listener of this.listeners) {
      try {
        listener(oldState, this.state, { type: 'USER_RESET' })
      } catch (err) {
        console.error('[ConnectionStateMachine] Listener error during reset:', err)
      }
    }
  }
}

// =============================================================================
// SINGLETON INSTANCE
// =============================================================================

let instance: ConnectionStateMachine | null = null

export function getConnectionStateMachine(): ConnectionStateMachine {
  if (!instance) {
    instance = new ConnectionStateMachine()
  }
  return instance
}

export function resetConnectionStateMachine(): void {
  instance = null
}

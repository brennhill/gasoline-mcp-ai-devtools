/**
 * @fileoverview Unified Sync Client - Replaces multiple polling loops with single /sync endpoint.
 * Features: Simple exponential backoff, binary connection state, self-healing for MV3.
 */
// =============================================================================
// CONSTANTS
// =============================================================================
const BASE_POLL_MS = 1000;
// =============================================================================
// SYNC CLIENT CLASS
// =============================================================================
export class SyncClient {
    serverUrl;
    sessionId;
    callbacks;
    state;
    intervalId = null;
    running = false;
    syncing = false;
    flushRequested = false;
    pendingResults = [];
    extensionVersion;
    constructor(serverUrl, sessionId, callbacks, extensionVersion = '') {
        this.serverUrl = serverUrl;
        this.sessionId = sessionId;
        this.callbacks = callbacks;
        this.extensionVersion = extensionVersion;
        this.state = {
            connected: false,
            lastSyncAt: 0,
            consecutiveFailures: 0,
            lastCommandAck: null
        };
    }
    /** Get current sync state */
    getState() {
        return { ...this.state };
    }
    /** Check if connected */
    isConnected() {
        return this.state.connected;
    }
    /** Start the sync loop */
    start() {
        if (this.running)
            return;
        this.running = true;
        this.log('Starting sync client');
        this.scheduleNextSync(0); // Sync immediately
    }
    /** Stop the sync loop */
    stop() {
        this.running = false;
        if (this.intervalId) {
            clearTimeout(this.intervalId);
            this.intervalId = null;
        }
        this.log('Stopped sync client');
    }
    /** Queue a command result to send on next sync, then flush immediately */
    queueCommandResult(result) {
        this.pendingResults.push(result);
        // Cap queue size to prevent memory leak if server is unreachable
        const MAX_PENDING_RESULTS = 200;
        if (this.pendingResults.length > MAX_PENDING_RESULTS) {
            this.pendingResults.splice(0, this.pendingResults.length - MAX_PENDING_RESULTS);
        }
        this.flush();
    }
    /** Trigger an immediate sync to deliver queued results with minimal latency */
    flush() {
        if (!this.running)
            return;
        if (this.syncing) {
            // Sync in progress — schedule another immediately after it finishes
            this.flushRequested = true;
            return;
        }
        if (this.intervalId) {
            clearTimeout(this.intervalId);
        }
        this.scheduleNextSync(0);
    }
    /** Reset connection state (e.g., when user toggles pilot/tracking) */
    resetConnection() {
        this.state.consecutiveFailures = 0;
        this.log('Connection state reset');
        // Trigger immediate sync if running
        if (this.running && this.intervalId) {
            clearTimeout(this.intervalId);
            this.scheduleNextSync(0);
        }
    }
    /** Update server URL */
    setServerUrl(url) {
        this.serverUrl = url;
    }
    // =============================================================================
    // PRIVATE METHODS
    // =============================================================================
    scheduleNextSync(delayMs) {
        if (!this.running)
            return;
        this.intervalId = setTimeout(() => this.doSync(), delayMs);
    }
    async doSync() {
        if (!this.running)
            return;
        this.syncing = true;
        this.flushRequested = false;
        try {
            // Build request
            const settings = await this.callbacks.getSettings();
            const logs = this.callbacks.getExtensionLogs();
            const request = {
                session_id: this.sessionId,
                extension_version: this.extensionVersion || undefined,
                settings
            };
            // Include logs if any
            if (logs.length > 0) {
                request.extension_logs = logs;
            }
            // Include pending command results
            if (this.pendingResults.length > 0) {
                request.command_results = [...this.pendingResults];
            }
            // Include last command ack
            if (this.state.lastCommandAck) {
                request.last_command_ack = this.state.lastCommandAck;
            }
            // Make request with timeout to prevent hanging forever
            const controller = new AbortController();
            const timeoutId = setTimeout(() => controller.abort(), 3000); // 3s timeout
            const response = await fetch(`${this.serverUrl}/sync`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Gasoline-Client': `gasoline-extension/${this.extensionVersion}`,
                    'X-Gasoline-Extension-Version': this.extensionVersion
                },
                body: JSON.stringify(request),
                signal: controller.signal
            });
            clearTimeout(timeoutId);
            if (!response.ok) {
                throw new Error(`Sync request failed: HTTP ${response.status} ${response.statusText} from ${this.serverUrl}/sync`);
            }
            const data = await response.json();
            // Log sync cycle summary
            this.log('Sync OK', {
                commands: data.commands?.length || 0,
                resultsSent: request.command_results?.length || 0,
                logsSent: request.extension_logs?.length || 0,
                nextPollMs: data.next_poll_ms
            });
            // Success - update state
            this.onSuccess();
            // Check for version mismatch (compare major.minor only, ignore patch)
            if (data.server_version && this.extensionVersion && this.callbacks.onVersionMismatch) {
                const serverMajorMinor = data.server_version.split('.').slice(0, 2).join('.');
                const extensionMajorMinor = this.extensionVersion.split('.').slice(0, 2).join('.');
                if (serverMajorMinor !== extensionMajorMinor) {
                    this.callbacks.onVersionMismatch(this.extensionVersion, data.server_version);
                }
            }
            // Clear sent logs and results
            if (logs.length > 0) {
                this.callbacks.clearExtensionLogs();
            }
            this.pendingResults = [];
            // Process commands
            if (data.commands && data.commands.length > 0) {
                this.log('Received commands', { count: data.commands.length, ids: data.commands.map((c) => c.id) });
                for (const command of data.commands) {
                    this.log('Dispatching command', {
                        id: command.id,
                        type: command.type,
                        correlation_id: command.correlation_id
                    });
                    try {
                        await this.callbacks.onCommand(command);
                        // Track ack only after successful execution
                        this.state.lastCommandAck = command.id;
                        this.log('Command dispatched OK', { id: command.id });
                    }
                    catch (err) {
                        this.log('Command dispatch FAILED', { id: command.id, error: err.message });
                    }
                }
            }
            // Handle capture overrides
            if (data.capture_overrides && this.callbacks.onCaptureOverrides) {
                this.callbacks.onCaptureOverrides(data.capture_overrides);
            }
            // Schedule next sync — flush immediately if results were queued during this sync
            this.syncing = false;
            if (this.flushRequested) {
                this.flushRequested = false;
                this.scheduleNextSync(0);
            }
            else {
                const nextPollMs = data.next_poll_ms || BASE_POLL_MS;
                this.scheduleNextSync(nextPollMs);
            }
        }
        catch (err) {
            // Failure - just retry after 1 second (no exponential backoff needed)
            this.syncing = false;
            this.flushRequested = false;
            this.onFailure();
            this.log('Sync failed, retrying', { error: err.message });
            this.scheduleNextSync(BASE_POLL_MS);
        }
    }
    onSuccess() {
        const wasDisconnected = !this.state.connected;
        this.state.connected = true;
        this.state.lastSyncAt = Date.now();
        this.state.consecutiveFailures = 0;
        if (wasDisconnected) {
            this.log('Connected');
            this.callbacks.onConnectionChange(true);
        }
    }
    onFailure() {
        const wasConnected = this.state.connected;
        this.state.connected = false;
        this.state.consecutiveFailures++;
        if (wasConnected) {
            this.log('Disconnected');
            this.callbacks.onConnectionChange(false);
        }
    }
    log(message, data) {
        if (this.callbacks.debugLog) {
            this.callbacks.debugLog('sync', message, data);
        }
        else {
            console.log(`[SyncClient] ${message}`, data || ''); // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal sync state, not user-controlled
        }
    }
}
// =============================================================================
// FACTORY FUNCTION
// =============================================================================
/**
 * Create a sync client instance
 */
export function createSyncClient(serverUrl, sessionId, callbacks, extensionVersion = '') {
    return new SyncClient(serverUrl, sessionId, callbacks, extensionVersion);
}
//# sourceMappingURL=sync-client.js.map
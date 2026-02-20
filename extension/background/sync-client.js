/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
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
    extSessionId;
    callbacks;
    state;
    intervalId = null;
    running = false;
    syncing = false;
    flushRequested = false;
    pendingResults = [];
    processedCommandIDs = new Set();
    extensionVersion;
    constructor(serverUrl, extSessionId, callbacks, extensionVersion = '') {
        this.serverUrl = serverUrl;
        this.extSessionId = extSessionId;
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
                ext_session_id: this.extSessionId,
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
            const timeoutId = setTimeout(() => controller.abort(), 8000); // 8s: server holds up to 5s + margin
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
            // Process commands — fire-and-forget so the sync loop is never blocked
            if (data.commands && data.commands.length > 0) {
                this.log('Received commands', { count: data.commands.length, ids: data.commands.map((c) => c.id) });
                for (const command of data.commands) {
                    if (command.id && this.processedCommandIDs.has(command.id)) {
                        this.log('Skipping already processed command', { id: command.id });
                        continue;
                    }
                    // Mark processed and ack on RECEIPT — before dispatch
                    if (command.id) {
                        this.processedCommandIDs.add(command.id);
                        const MAX_PROCESSED_COMMANDS = 1000;
                        if (this.processedCommandIDs.size > MAX_PROCESSED_COMMANDS) {
                            const oldest = this.processedCommandIDs.values().next().value;
                            if (oldest !== undefined) {
                                this.processedCommandIDs.delete(oldest);
                            }
                        }
                    }
                    this.state.lastCommandAck = command.id;
                    this.log('Dispatching command (fire-and-forget)', {
                        id: command.id,
                        type: command.type,
                        correlation_id: command.correlation_id
                    });
                    // Fire-and-forget: don't await — sync loop continues immediately
                    try {
                        this.callbacks.onCommand(command).then(() => {
                            this.log('Command completed OK', { id: command.id });
                        }, (err) => {
                            this.log('Command execution FAILED', { id: command.id, error: err.message });
                            this.queueCommandResult({
                                id: command.id,
                                status: 'error',
                                error: err.message || 'Command execution failed'
                            });
                        });
                    }
                    catch (err) {
                        this.log('Command dispatch FAILED (sync throw)', { id: command.id, error: err.message });
                        this.queueCommandResult({
                            id: command.id,
                            status: 'error',
                            error: err.message || 'Command dispatch failed'
                        });
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
        this.state.consecutiveFailures++;
        // Require 2+ consecutive failures before marking disconnected
        // to prevent a single transient timeout from flipping connection state
        if (this.state.consecutiveFailures >= 2 && this.state.connected) {
            this.state.connected = false;
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
export function createSyncClient(serverUrl, extSessionId, callbacks, extensionVersion = '') {
    return new SyncClient(serverUrl, extSessionId, callbacks, extensionVersion);
}
//# sourceMappingURL=sync-client.js.map
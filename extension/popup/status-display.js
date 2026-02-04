/**
 * @fileoverview Status Display Module
 * Updates connection status display in popup
 */
import { formatFileSize } from './ui-utils.js';
const DEFAULT_MAX_ENTRIES = 1000;
/**
 * Update the connection status display
 */
export function updateConnectionStatus(status) {
    const statusEl = document.getElementById('status');
    const entriesEl = document.getElementById('entries-count');
    const errorEl = document.getElementById('error-message');
    const serverUrlEl = document.getElementById('server-url');
    const logFileEl = document.getElementById('log-file-path');
    const errorCountEl = document.getElementById('error-count');
    const troubleshootingEl = document.getElementById('troubleshooting');
    if (status.connected) {
        if (statusEl) {
            statusEl.textContent = 'Connected';
            statusEl.classList.remove('disconnected');
            statusEl.classList.add('connected');
        }
        const entries = status.entries || 0;
        const maxEntries = status.maxEntries || DEFAULT_MAX_ENTRIES;
        if (entriesEl) {
            entriesEl.textContent = `${entries} / ${maxEntries}`;
        }
        if (errorEl) {
            errorEl.textContent = '';
        }
        if (troubleshootingEl) {
            troubleshootingEl.style.display = 'none';
        }
    }
    else {
        if (statusEl) {
            statusEl.textContent = 'Disconnected';
            statusEl.classList.remove('connected');
            statusEl.classList.add('disconnected');
        }
        if (errorEl && status.error) {
            errorEl.textContent = status.error;
        }
        if (troubleshootingEl) {
            troubleshootingEl.style.display = 'block';
        }
    }
    // Version mismatch warning
    const versionWarningEl = document.getElementById('version-mismatch');
    if (versionWarningEl) {
        if (status.versionMismatch && status.serverVersion && status.extensionVersion) {
            versionWarningEl.style.display = 'block';
            const versionDetail = versionWarningEl.querySelector('.version-detail');
            if (versionDetail) {
                versionDetail.textContent = `Server: v${status.serverVersion} / Extension: v${status.extensionVersion}`;
            }
        }
        else {
            versionWarningEl.style.display = 'none';
        }
    }
    if (serverUrlEl && status.serverUrl) {
        serverUrlEl.textContent = status.serverUrl;
    }
    if (logFileEl && status.logFile) {
        logFileEl.textContent = status.logFile;
    }
    if (errorCountEl && status.errorCount !== undefined) {
        errorCountEl.textContent = String(status.errorCount);
    }
    // Log file size
    const fileSizeEl = document.getElementById('log-file-size');
    if (fileSizeEl && status.logFileSize !== undefined) {
        fileSizeEl.textContent = formatFileSize(status.logFileSize);
    }
    // Health indicators (circuit breaker + memory pressure)
    const healthSection = document.getElementById('health-indicators');
    const cbEl = document.getElementById('health-circuit-breaker');
    const mpEl = document.getElementById('health-memory-pressure');
    if (healthSection && cbEl && mpEl) {
        const cbState = status.circuitBreakerState || 'closed';
        const mpState = status.memoryPressure?.memoryPressureLevel || 'normal';
        // Circuit breaker indicator
        cbEl.classList.remove('health-error', 'health-warning');
        if (!status.connected || cbState === 'closed') {
            cbEl.style.display = 'none';
            cbEl.textContent = '';
        }
        else if (cbState === 'open') {
            cbEl.style.display = '';
            cbEl.classList.add('health-error');
            cbEl.textContent = 'Server: open (paused)';
        }
        else if (cbState === 'half-open') {
            cbEl.style.display = '';
            cbEl.classList.add('health-warning');
            cbEl.textContent = 'Server: half-open (probing)';
        }
        // Memory pressure indicator
        mpEl.classList.remove('health-error', 'health-warning');
        if (!status.connected || mpState === 'normal') {
            mpEl.style.display = 'none';
            mpEl.textContent = '';
        }
        else if (mpState === 'soft') {
            mpEl.style.display = '';
            mpEl.classList.add('health-warning');
            mpEl.textContent = 'Memory: elevated (reduced capacities)';
        }
        else if (mpState === 'hard') {
            mpEl.style.display = '';
            mpEl.classList.add('health-error');
            mpEl.textContent = 'Memory: critical (bodies disabled)';
        }
        // Show/hide entire section
        const cbVisible = status.connected && cbState !== 'closed';
        const mpVisible = status.connected && mpState !== 'normal';
        healthSection.style.display = cbVisible || mpVisible ? '' : 'none';
    }
    // Context annotation warning
    const contextWarningEl = document.getElementById('context-warning');
    const contextWarningTextEl = document.getElementById('context-warning-text');
    if (contextWarningEl) {
        if (status.connected && status.contextWarning) {
            contextWarningEl.style.display = 'block';
            if (contextWarningTextEl) {
                contextWarningTextEl.textContent = `${status.contextWarning.count} recent entries have context annotations averaging ${status.contextWarning.sizeKB}KB. This may consume significant AI context window space.`;
            }
        }
        else {
            contextWarningEl.style.display = 'none';
            if (contextWarningTextEl) {
                contextWarningTextEl.textContent = '';
            }
        }
    }
}
//# sourceMappingURL=status-display.js.map
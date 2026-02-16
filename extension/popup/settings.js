/**
 * @fileoverview Settings Module
 * Handles log level, WebSocket mode, and clear logs functionality
 */
/**
 * Handle WebSocket mode change
 */
export function handleWebSocketModeChange(mode) {
    chrome.storage.local.set({ webSocketCaptureMode: mode });
    chrome.runtime.sendMessage({ type: 'setWebSocketCaptureMode', mode });
}
/**
 * Initialize the WebSocket mode selector
 */
export async function initWebSocketModeSelector() {
    const modeSelect = document.getElementById('ws-mode');
    if (!modeSelect)
        return;
    return new Promise((resolve) => {
        chrome.storage.local.get(['webSocketCaptureMode'], (result) => {
            modeSelect.value = result.webSocketCaptureMode || 'medium';
            resolve();
        });
    });
}
// Track clear-logs confirmation state
let clearConfirmPending = false;
let clearConfirmTimer = null;
/**
 * Reset clear confirmation state (exported for testing)
 */
export function resetClearConfirm() {
    clearConfirmPending = false;
    if (clearConfirmTimer) {
        clearTimeout(clearConfirmTimer);
        clearConfirmTimer = null;
    }
}
/**
 * Handle clear logs button click (with confirmation)
 */
export async function handleClearLogs() {
    const clearBtn = document.getElementById('clear-btn');
    const entriesEl = document.getElementById('entries-count');
    // Two-click confirmation: first click changes to "Confirm?", second click clears
    if (clearBtn && !clearConfirmPending) {
        clearConfirmPending = true;
        clearBtn.textContent = 'Confirm Clear?';
        // Reset after 3 seconds if not confirmed
        clearConfirmTimer = setTimeout(() => {
            clearConfirmPending = false;
            if (clearBtn)
                clearBtn.textContent = 'Clear Logs';
        }, 3000);
        return Promise.resolve(null);
    }
    // Second click: actually clear
    clearConfirmPending = false;
    if (clearConfirmTimer) {
        clearTimeout(clearConfirmTimer);
        clearConfirmTimer = null;
    }
    if (clearBtn) {
        clearBtn.disabled = true;
        clearBtn.textContent = 'Clearing...';
    }
    return new Promise((resolve) => {
        chrome.runtime.sendMessage({ type: 'clearLogs' }, (response) => {
            if (clearBtn) {
                clearBtn.disabled = false;
                clearBtn.textContent = 'Clear Logs';
            }
            if (response?.success) {
                if (entriesEl) {
                    entriesEl.textContent = '0 / 1000';
                }
            }
            else if (response?.error) {
                const errorEl = document.getElementById('error-message');
                if (errorEl) {
                    errorEl.textContent = response.error;
                }
            }
            resolve(response || null);
        });
    });
}
//# sourceMappingURL=settings.js.map
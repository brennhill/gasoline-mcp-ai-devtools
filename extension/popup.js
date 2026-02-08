/**
 * @fileoverview popup.ts - Extension popup UI showing connection status and controls.
 * Displays server connection state, entry count, error count, log level selector,
 * and log file path. Polls the background worker for status updates and provides
 * a clear-logs button. Shows troubleshooting hints when disconnected.
 * Design: Pure DOM manipulation, no framework. Communicates with background.js
 * via chrome.runtime.sendMessage for status queries and log-level changes.
 */
import { updateConnectionStatus } from './popup/status-display.js';
import { initFeatureToggles } from './popup/feature-toggles.js';
import { initTrackPageButton } from './popup/tab-tracking.js';
import { initAiWebPilotToggle } from './popup/ai-web-pilot.js';
import { initWebSocketModeSelector, handleWebSocketModeChange, handleClearLogs, resetClearConfirm, } from './popup/settings.js';
// Re-export for testing
export { resetClearConfirm, handleClearLogs };
export { updateConnectionStatus };
export { FEATURE_TOGGLES, initFeatureToggles } from './popup/feature-toggles.js';
export { handleFeatureToggle } from './popup/feature-toggles.js';
export { initAiWebPilotToggle, handleAiWebPilotToggle } from './popup/ai-web-pilot.js';
export { initTrackPageButton, handleTrackPageClick } from './popup/tab-tracking.js';
export { handleWebSocketModeChange } from './popup/settings.js';
export { initWebSocketModeSelector } from './popup/settings.js';
export { isInternalUrl } from './popup/ui-utils.js';
const DEFAULT_MAX_ENTRIES = 1000;
/**
 * Initialize the popup
 */
export async function initPopup() {
    // Request current status from background - may fail if service worker is inactive
    try {
        chrome.runtime.sendMessage({ type: 'getStatus' }, (status) => {
            if (chrome.runtime.lastError) {
                // Background service worker may be inactive or restarting
                updateConnectionStatus({
                    connected: false,
                    entries: 0,
                    maxEntries: DEFAULT_MAX_ENTRIES,
                    errorCount: 0,
                    logFile: '',
                    error: 'Extension restarting - please wait a moment and reopen popup',
                });
                return;
            }
            if (status) {
                updateConnectionStatus(status);
            }
        });
    }
    catch {
        // Extension context invalidated or other critical error
        updateConnectionStatus({
            connected: false,
            entries: 0,
            maxEntries: DEFAULT_MAX_ENTRIES,
            errorCount: 0,
            logFile: '',
            error: 'Extension error - try reloading the extension',
        });
    }
    // Initialize recording UI
    setupRecordingUI();
    // Check for pending audio recording that needs activeTab gesture.
    // When the user clicks the extension icon, activeTab is granted for the active tab.
    // The popup auto-sends RECORDING_GESTURE_GRANTED to unblock the service worker.
    chrome.storage.local.get('gasoline_pending_recording', (result) => {
        if (result.gasoline_pending_recording) {
            chrome.runtime.sendMessage({ type: 'RECORDING_GESTURE_GRANTED' });
            chrome.storage.local.remove('gasoline_pending_recording');
        }
    });
    // Initialize feature toggles
    await initFeatureToggles();
    // Initialize WebSocket mode selector
    await initWebSocketModeSelector();
    // Initialize AI Web Pilot toggle
    await initAiWebPilotToggle();
    // Initialize Track This Page button
    await initTrackPageButton();
    // Show/hide WebSocket mode selector based on toggle
    const wsToggle = document.getElementById('toggle-websocket');
    const wsModeContainer = document.getElementById('ws-mode-container');
    if (wsToggle && wsModeContainer) {
        wsModeContainer.style.display = wsToggle.checked ? 'block' : 'none';
        wsToggle.addEventListener('change', () => {
            wsModeContainer.style.display = wsToggle.checked ? 'block' : 'none';
        });
    }
    // Set up WebSocket mode change handler
    const wsModeSelect = document.getElementById('ws-mode');
    if (wsModeSelect) {
        wsModeSelect.addEventListener('change', (e) => {
            const target = e.target;
            handleWebSocketModeChange(target.value);
        });
    }
    // Show/hide WebSocket messages warning based on mode
    const wsMessagesWarning = document.getElementById('ws-messages-warning');
    if (wsModeSelect && wsMessagesWarning) {
        wsMessagesWarning.style.display = wsModeSelect.value === 'all' ? 'block' : 'none';
        wsModeSelect.addEventListener('change', () => {
            wsMessagesWarning.style.display = wsModeSelect.value === 'all' ? 'block' : 'none';
        });
    }
    // Show/hide toggle warnings when features are enabled
    const toggleWarnings = [
        { toggleId: 'toggle-screenshot', warningId: 'screenshot-warning' },
        { toggleId: 'toggle-network-waterfall', warningId: 'waterfall-warning' },
        { toggleId: 'toggle-performance-marks', warningId: 'perfmarks-warning' },
    ];
    for (const { toggleId, warningId } of toggleWarnings) {
        const toggle = document.getElementById(toggleId);
        const warning = document.getElementById(warningId);
        if (toggle && warning) {
            warning.style.display = toggle.checked ? 'block' : 'none';
            toggle.addEventListener('change', () => {
                warning.style.display = toggle.checked ? 'block' : 'none';
            });
        }
    }
    // Set up clear button handler
    const clearBtn = document.getElementById('clear-btn');
    if (clearBtn) {
        clearBtn.addEventListener('click', handleClearLogs);
    }
    // Listen for status updates
    chrome.runtime.onMessage.addListener((message) => {
        if (message.type === 'statusUpdate' && message.status) {
            updateConnectionStatus(message.status);
        }
        else if (message.type === 'pilotStatusChanged') {
            // Update toggle to reflect confirmed state from background
            const toggle = document.getElementById('aiWebPilotEnabled');
            if (toggle) {
                toggle.checked = message.enabled === true;
                console.log('[Gasoline] Pilot status confirmed:', message.enabled);
            }
        }
    });
    // Listen for storage changes (e.g., tracked tab URL updates)
    chrome.storage.onChanged.addListener((changes, areaName) => {
        if (areaName === 'local' && changes.trackedTabUrl) {
            const urlEl = document.getElementById('tracked-url');
            if (urlEl && changes.trackedTabUrl.newValue) {
                urlEl.textContent = changes.trackedTabUrl.newValue;
                console.log('[Gasoline] Tracked tab URL updated in popup:', changes.trackedTabUrl.newValue);
            }
        }
    });
}
/**
 * Set up recording row: single clickable row toggles between idle/recording states.
 * Syncs with chrome.storage.local for MCP-initiated recordings.
 */
function setupRecordingUI() {
    const row = document.getElementById('record-row');
    const label = document.getElementById('record-label');
    const statusEl = document.getElementById('recording-status');
    if (!row || !label || !statusEl)
        return;
    let timerInterval = null;
    let isRecording = false;
    function showRecording(name, startTime) {
        isRecording = true;
        row.classList.add('is-recording');
        label.textContent = 'Stop';
        statusEl.textContent = '';
        const opts = document.getElementById('record-options');
        if (opts)
            opts.style.display = 'none';
        if (timerInterval)
            clearInterval(timerInterval);
        timerInterval = setInterval(() => {
            const elapsed = Math.round((Date.now() - startTime) / 1000);
            const mins = Math.floor(elapsed / 60);
            const secs = elapsed % 60;
            statusEl.textContent = `${mins}:${secs.toString().padStart(2, '0')}`;
        }, 1000);
    }
    function showIdle() {
        isRecording = false;
        row.classList.remove('is-recording');
        label.textContent = 'Record';
        statusEl.textContent = '';
        const opts = document.getElementById('record-options');
        if (opts)
            opts.style.display = 'block';
        if (timerInterval) {
            clearInterval(timerInterval);
            timerInterval = null;
        }
    }
    // Check if a recording is already active (e.g., started via MCP)
    chrome.storage.local.get('gasoline_recording', (result) => {
        const rec = result.gasoline_recording;
        console.log('[Gasoline REC] Popup: gasoline_recording from storage:', rec);
        if (rec?.active && rec.name && rec.startTime) {
            console.log('[Gasoline REC] Popup: resuming recording UI for', rec.name);
            showRecording(rec.name, rec.startTime);
        }
    });
    // Listen for recording state changes (MCP-initiated start/stop)
    chrome.storage.onChanged.addListener((changes, areaName) => {
        if (areaName === 'local' && changes.gasoline_recording) {
            const rec = changes.gasoline_recording.newValue;
            console.log('[Gasoline REC] Popup: gasoline_recording changed:', rec);
            if (rec?.active && rec.name && rec.startTime) {
                showRecording(rec.name, rec.startTime);
            }
            else {
                showIdle();
            }
        }
    });
    // If user just granted mic permission, pre-select the audio mode they intended.
    // They just need to click Record — mic permission is already granted.
    chrome.storage.local.get('gasoline_pending_mic_recording', (result) => {
        const intent = result.gasoline_pending_mic_recording;
        console.log('[Gasoline REC] Popup: pending_mic_recording intent:', intent);
        if (!intent?.audioMode)
            return;
        console.log('[Gasoline REC] Popup: consuming mic intent, pre-selecting audioMode:', intent.audioMode);
        chrome.storage.local.remove('gasoline_pending_mic_recording');
        // Dismiss the guidance toast on the active tab
        chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
            if (tabs[0]?.id) {
                chrome.tabs.sendMessage(tabs[0].id, {
                    type: 'GASOLINE_ACTION_TOAST',
                    text: '',
                    detail: '',
                    state: 'success',
                    duration_ms: 1,
                }).catch(() => { });
            }
        });
        // Pre-select the audio mode so user just needs to click Record
        const audioSelect = document.getElementById('record-audio-mode');
        if (audioSelect)
            audioSelect.value = intent.audioMode;
    });
    // Hide options when recording, show when idle
    const optionsEl = document.getElementById('record-options');
    const saveInfoEl = document.getElementById('record-save-info');
    row.addEventListener('click', () => {
        console.log('[Gasoline REC] Popup: record row clicked, isRecording:', isRecording);
        if (isRecording) {
            row.classList.remove('is-recording');
            label.textContent = 'Saving...';
            console.log('[Gasoline REC] Popup: sending record_stop');
            chrome.runtime.sendMessage({ type: 'record_stop' }, (resp) => {
                console.log('[Gasoline REC] Popup: record_stop response:', resp);
                if (chrome.runtime.lastError) {
                    console.error('[Gasoline REC] Popup: record_stop lastError:', chrome.runtime.lastError.message);
                }
                showIdle();
                if (resp?.status === 'saved' && resp.name && saveInfoEl) {
                    const displayName = resp.name.replace(/--\d{4}-\d{2}-\d{2}-\d{4}(-\d+)?$/, '');
                    if (resp.path) {
                        saveInfoEl.textContent = 'Saved: ';
                        const link = document.createElement('a');
                        link.href = '#';
                        link.id = 'reveal-recording';
                        link.textContent = displayName;
                        link.style.color = '#58a6ff';
                        link.style.textDecoration = 'underline';
                        link.style.cursor = 'pointer';
                        saveInfoEl.appendChild(link);
                        const linkEl = document.getElementById('reveal-recording');
                        if (linkEl) {
                            linkEl.addEventListener('click', (e) => {
                                e.preventDefault();
                                chrome.runtime.sendMessage({ type: 'REVEAL_FILE', path: resp.path }, (result) => {
                                    if (result?.error && saveInfoEl) {
                                        saveInfoEl.textContent = `Could not open folder: ${result.error}`;
                                        saveInfoEl.style.color = '#f85149';
                                        setTimeout(() => { saveInfoEl.style.display = 'none'; }, 5000);
                                    }
                                });
                            });
                        }
                    }
                    else {
                        saveInfoEl.textContent = `Saved: ${displayName}`;
                    }
                    saveInfoEl.style.display = 'block';
                    setTimeout(() => { saveInfoEl.style.display = 'none'; }, 12000);
                }
            });
        }
        else {
            const audioSelect = document.getElementById('record-audio-mode');
            const audioMode = audioSelect?.value ?? '';
            if (optionsEl)
                optionsEl.style.display = 'none';
            if (saveInfoEl)
                saveInfoEl.style.display = 'none';
            label.textContent = 'Starting...';
            const sendStart = () => {
                console.log('[Gasoline REC] Popup: sendStart() called, sending record_start with audio:', audioMode);
                chrome.runtime.sendMessage({ type: 'record_start', audio: audioMode }, (resp) => {
                    console.log('[Gasoline REC] Popup: record_start response:', resp);
                    if (chrome.runtime.lastError) {
                        console.error('[Gasoline REC] Popup: record_start lastError:', chrome.runtime.lastError.message);
                    }
                    if (resp?.status === 'recording' && resp.name) {
                        showRecording(resp.name, resp.startTime ?? Date.now());
                    }
                    else {
                        showIdle();
                        if (resp?.error && saveInfoEl) {
                            saveInfoEl.textContent = resp.error;
                            saveInfoEl.style.display = 'block';
                            saveInfoEl.style.background = 'rgba(248, 81, 73, 0.1)';
                            saveInfoEl.style.color = '#f85149';
                            setTimeout(() => {
                                saveInfoEl.style.display = 'none';
                                saveInfoEl.style.background = 'rgba(63, 185, 80, 0.1)';
                                saveInfoEl.style.color = '#3fb950';
                            }, 5000);
                        }
                    }
                });
            };
            // Mic modes need permission granted via a full extension page.
            // Chrome popups can't reliably show the browser permission dialog.
            if (audioMode === 'mic' || audioMode === 'both') {
                console.log('[Gasoline REC] Popup: mic/both mode — checking gasoline_mic_granted');
                // Verify mic permission is actually granted (not just cached flag).
                // The cached flag can become stale after extension reload/update.
                const tryMicOrShowPermissionPage = () => {
                    console.log('[Gasoline REC] Popup: trying getUserMedia from popup...');
                    navigator.mediaDevices
                        .getUserMedia({ audio: true })
                        .then((micStream) => {
                        console.log('[Gasoline REC] Popup: getUserMedia succeeded from popup');
                        micStream.getTracks().forEach((t) => t.stop());
                        chrome.storage.local.set({ gasoline_mic_granted: true });
                        sendStart();
                    })
                        .catch((err) => {
                        console.log('[Gasoline REC] Popup: getUserMedia FAILED:', err.name, err.message);
                        // Clear stale flag so next attempt goes through permission page
                        chrome.storage.local.remove('gasoline_mic_granted');
                        showIdle();
                        if (saveInfoEl) {
                            // Store recording intent + current tab so we can return after permission grant
                            chrome.tabs.query({ active: true, currentWindow: true }, (activeTabs) => {
                                chrome.storage.local.set({
                                    gasoline_pending_mic_recording: { audioMode, returnTabId: activeTabs[0]?.id },
                                });
                            });
                            saveInfoEl.innerHTML =
                                'Microphone access needed. <a href="#" id="grant-mic-link" style="color: #58a6ff; text-decoration: underline; cursor: pointer">Grant access</a>';
                            saveInfoEl.style.display = 'block';
                            saveInfoEl.style.background = 'rgba(248, 81, 73, 0.1)';
                            saveInfoEl.style.color = '#f85149';
                            const link = document.getElementById('grant-mic-link');
                            if (link) {
                                link.addEventListener('click', (e) => {
                                    e.preventDefault();
                                    chrome.tabs.create({ url: chrome.runtime.getURL('mic-permission.html') });
                                });
                            }
                        }
                    });
                };
                tryMicOrShowPermissionPage();
            }
            else {
                sendStart();
            }
        }
    });
}
// Initialize when DOM is ready
if (typeof document !== 'undefined' && typeof globalThis.process === 'undefined') {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initPopup);
    }
    else {
        initPopup();
    }
}
//# sourceMappingURL=popup.js.map
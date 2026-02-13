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
import { initWebSocketModeSelector, handleWebSocketModeChange, handleClearLogs, resetClearConfirm } from './popup/settings.js';
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
 * Bind a toggle element to show/hide a target element based on a condition.
 * Sets initial display state and adds a change listener.
 */
function bindToggleVisibility(toggle, target, isVisible) {
    target.style.display = isVisible() ? 'block' : 'none';
    toggle.addEventListener('change', () => { target.style.display = isVisible() ? 'block' : 'none'; });
}
// #lizard forgives
function setupWebSocketUI() {
    const wsToggle = document.getElementById('toggle-websocket');
    const wsModeContainer = document.getElementById('ws-mode-container');
    if (wsToggle && wsModeContainer) {
        bindToggleVisibility(wsToggle, wsModeContainer, () => wsToggle.checked);
    }
    const wsModeSelect = document.getElementById('ws-mode');
    if (wsModeSelect) {
        wsModeSelect.addEventListener('change', (e) => {
            handleWebSocketModeChange(e.target.value);
        });
    }
    const wsMessagesWarning = document.getElementById('ws-messages-warning');
    if (wsModeSelect && wsMessagesWarning) {
        bindToggleVisibility(wsModeSelect, wsMessagesWarning, () => wsModeSelect.value === 'all');
    }
}
function setupToggleWarnings() {
    const toggleWarnings = [
        { toggleId: 'toggle-screenshot', warningId: 'screenshot-warning' },
        { toggleId: 'toggle-network-waterfall', warningId: 'waterfall-warning' },
        { toggleId: 'toggle-performance-marks', warningId: 'perfmarks-warning' }
    ];
    for (const { toggleId, warningId } of toggleWarnings) {
        const toggle = document.getElementById(toggleId);
        const warning = document.getElementById(warningId);
        if (toggle && warning) {
            warning.style.display = toggle.checked ? 'block' : 'none';
            toggle.addEventListener('change', () => { warning.style.display = toggle.checked ? 'block' : 'none'; });
        }
    }
}
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
                    error: 'Extension restarting - please wait a moment and reopen popup'
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
            error: 'Extension error - try reloading the extension'
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
    setupWebSocketUI();
    setupToggleWarnings();
    const clearBtn = document.getElementById('clear-btn');
    if (clearBtn)
        clearBtn.addEventListener('click', handleClearLogs);
    // Initialize draw mode button
    setupDrawModeButton();
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
            const urlEl = document.getElementById('tracking-bar-url');
            if (urlEl && changes.trackedTabUrl.newValue) {
                urlEl.textContent = changes.trackedTabUrl.newValue;
                console.log('[Gasoline] Tracked tab URL updated in popup:', changes.trackedTabUrl.newValue);
            }
        }
    });
}
// #lizard forgives
function showRecording(els, state, name, startTime) {
    state.isRecording = true;
    els.row.classList.add('is-recording');
    els.label.textContent = 'Stop';
    els.statusEl.textContent = '';
    if (els.optionsEl)
        els.optionsEl.style.display = 'none';
    if (state.timerInterval)
        clearInterval(state.timerInterval);
    state.timerInterval = setInterval(() => {
        const elapsed = Math.round((Date.now() - startTime) / 1000);
        const mins = Math.floor(elapsed / 60);
        const secs = elapsed % 60;
        els.statusEl.textContent = `${mins}:${secs.toString().padStart(2, '0')}`;
    }, 1000);
}
function showIdle(els, state) {
    state.isRecording = false;
    els.row.classList.remove('is-recording');
    els.label.textContent = 'Record';
    els.statusEl.textContent = '';
    if (els.optionsEl)
        els.optionsEl.style.display = 'block';
    if (state.timerInterval) {
        clearInterval(state.timerInterval);
        state.timerInterval = null;
    }
}
function showSavedLink(saveInfoEl, displayName, filePath) {
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
            chrome.runtime.sendMessage({ type: 'REVEAL_FILE', path: filePath }, (result) => {
                if (result?.error) {
                    saveInfoEl.textContent = `Could not open folder: ${result.error}`;
                    saveInfoEl.style.color = '#f85149';
                    setTimeout(() => { saveInfoEl.style.display = 'none'; }, 5000);
                }
            });
        });
    }
}
function showSaveResult(saveInfoEl, resp) {
    if (resp?.status !== 'saved' || !resp.name || !saveInfoEl)
        return;
    const displayName = resp.name.replace(/--\d{4}-\d{2}-\d{2}-\d{4}(-\d+)?$/, '');
    if (resp.path) {
        showSavedLink(saveInfoEl, displayName, resp.path);
    }
    else {
        saveInfoEl.textContent = `Saved: ${displayName}`;
    }
    saveInfoEl.style.display = 'block';
    setTimeout(() => { saveInfoEl.style.display = 'none'; }, 12000);
}
function showStartError(saveInfoEl, errorText) {
    if (!saveInfoEl)
        return;
    saveInfoEl.textContent = errorText;
    saveInfoEl.style.display = 'block';
    saveInfoEl.style.background = 'rgba(248, 81, 73, 0.1)';
    saveInfoEl.style.color = '#f85149';
    setTimeout(() => {
        saveInfoEl.style.display = 'none';
        saveInfoEl.style.background = 'rgba(63, 185, 80, 0.1)';
        saveInfoEl.style.color = '#3fb950';
    }, 5000);
}
function showDrawModeError(label, message) {
    label.textContent = message;
    label.style.color = '#f85149';
    setTimeout(() => {
        label.textContent = 'Draw';
        label.style.color = '';
    }, 3000);
}
function setupDrawModeButton() {
    const row = document.getElementById('draw-mode-row');
    const label = document.getElementById('draw-mode-label');
    if (!row || !label)
        return;
    row.addEventListener('click', () => {
        chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
            const tab = tabs[0];
            if (!tab?.id) {
                showDrawModeError(label, 'No active tab');
                return;
            }
            if (tab.url?.startsWith('chrome://') || tab.url?.startsWith('about:') || tab.url?.startsWith('chrome-extension://')) {
                showDrawModeError(label, 'Cannot draw on internal pages');
                return;
            }
            label.textContent = 'Starting...';
            chrome.tabs.sendMessage(tab.id, { type: 'GASOLINE_DRAW_MODE_START', started_by: 'user' }, (resp) => {
                if (chrome.runtime.lastError) {
                    showDrawModeError(label, 'Content script not loaded — try refreshing the page');
                    return;
                }
                if (resp?.error) {
                    showDrawModeError(label, resp.message || 'Draw mode failed');
                    return;
                }
                // Close popup so user can interact with the page
                window.close();
            });
        });
    });
}
function handleStopClick(els, state) {
    els.row.classList.remove('is-recording');
    els.label.textContent = 'Saving...';
    console.log('[Gasoline REC] Popup: sending record_stop');
    chrome.runtime.sendMessage({ type: 'record_stop' }, (resp) => {
        console.log('[Gasoline REC] Popup: record_stop response:', resp);
        if (chrome.runtime.lastError) {
            console.error('[Gasoline REC] Popup: record_stop lastError:', chrome.runtime.lastError.message);
        }
        showIdle(els, state);
        showSaveResult(els.saveInfoEl, resp);
    });
}
function sendRecordStart(els, state, audioMode) {
    console.log('[Gasoline REC] Popup: sendStart() called, sending record_start with audio:', audioMode);
    chrome.runtime.sendMessage({ type: 'record_start', audio: audioMode }, (resp) => {
        console.log('[Gasoline REC] Popup: record_start response:', resp);
        if (chrome.runtime.lastError) {
            console.error('[Gasoline REC] Popup: record_start lastError:', chrome.runtime.lastError.message);
        }
        if (resp?.status === 'recording' && resp.name) {
            showRecording(els, state, resp.name, resp.startTime ?? Date.now());
        }
        else {
            showIdle(els, state);
            if (resp?.error)
                showStartError(els.saveInfoEl, resp.error);
        }
    });
}
function showMicPermissionPrompt(saveInfoEl, audioMode) {
    chrome.tabs.query({ active: true, currentWindow: true }, (activeTabs) => {
        chrome.storage.local.set({
            gasoline_pending_mic_recording: { audioMode, returnTabId: activeTabs[0]?.id }
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
// #lizard forgives
function tryMicPermissionThenStart(els, state, audioMode) {
    console.log('[Gasoline REC] Popup: trying getUserMedia from popup...');
    navigator.mediaDevices
        .getUserMedia({ audio: true })
        .then((micStream) => {
        console.log('[Gasoline REC] Popup: getUserMedia succeeded from popup');
        micStream.getTracks().forEach((t) => t.stop());
        chrome.storage.local.set({ gasoline_mic_granted: true });
        sendRecordStart(els, state, audioMode);
    })
        .catch((err) => {
        console.log('[Gasoline REC] Popup: getUserMedia FAILED:', err.name, err.message);
        chrome.storage.local.remove('gasoline_mic_granted');
        showIdle(els, state);
        if (els.saveInfoEl)
            showMicPermissionPrompt(els.saveInfoEl, audioMode);
    });
}
function handleStartClick(els, state) {
    const audioSelect = document.getElementById('record-audio-mode');
    const audioMode = audioSelect?.value ?? '';
    // Save preference for next time
    chrome.storage.local.set({ gasoline_record_audio_pref: audioMode });
    if (els.optionsEl)
        els.optionsEl.style.display = 'none';
    if (els.saveInfoEl)
        els.saveInfoEl.style.display = 'none';
    els.label.textContent = 'Starting...';
    if (audioMode === 'mic' || audioMode === 'both') {
        console.log('[Gasoline REC] Popup: mic/both mode — checking gasoline_mic_granted');
        tryMicPermissionThenStart(els, state, audioMode);
    }
    else {
        sendRecordStart(els, state, audioMode);
    }
}
function setupRecordingUI() {
    const row = document.getElementById('record-row');
    const label = document.getElementById('record-label');
    const statusEl = document.getElementById('recording-status');
    if (!row || !label || !statusEl)
        return;
    const els = {
        row,
        label,
        statusEl,
        optionsEl: document.getElementById('record-options'),
        saveInfoEl: document.getElementById('record-save-info')
    };
    const state = { isRecording: false, timerInterval: null };
    chrome.storage.local.get('gasoline_recording', (result) => {
        const rec = result.gasoline_recording;
        console.log('[Gasoline REC] Popup: gasoline_recording from storage:', rec);
        if (rec?.active && rec.name && rec.startTime) {
            console.log('[Gasoline REC] Popup: resuming recording UI for', rec.name);
            showRecording(els, state, rec.name, rec.startTime);
        }
    });
    chrome.storage.onChanged.addListener((changes, areaName) => {
        if (areaName === 'local' && changes.gasoline_recording) {
            const rec = changes.gasoline_recording.newValue;
            console.log('[Gasoline REC] Popup: gasoline_recording changed:', rec);
            if (rec?.active && rec.name && rec.startTime) {
                showRecording(els, state, rec.name, rec.startTime);
            }
            else {
                showIdle(els, state);
            }
        }
    });
    chrome.storage.local.get('gasoline_pending_mic_recording', (result) => {
        const intent = result.gasoline_pending_mic_recording;
        console.log('[Gasoline REC] Popup: pending_mic_recording intent:', intent);
        if (!intent?.audioMode)
            return;
        console.log('[Gasoline REC] Popup: consuming mic intent, pre-selecting audioMode:', intent.audioMode);
        chrome.storage.local.remove('gasoline_pending_mic_recording');
        chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
            if (tabs[0]?.id) {
                chrome.tabs
                    .sendMessage(tabs[0].id, {
                    type: 'GASOLINE_ACTION_TOAST',
                    text: '',
                    detail: '',
                    state: 'success',
                    duration_ms: 1
                })
                    .catch(() => { });
            }
        });
        const audioSelect = document.getElementById('record-audio-mode');
        if (audioSelect)
            audioSelect.value = intent.audioMode;
    });
    // Restore saved audio mode preference
    chrome.storage.local.get('gasoline_record_audio_pref', (result) => {
        const saved = result.gasoline_record_audio_pref;
        if (saved) {
            const audioSelect = document.getElementById('record-audio-mode');
            if (audioSelect)
                audioSelect.value = saved;
        }
    });
    row.addEventListener('click', () => {
        console.log('[Gasoline REC] Popup: record row clicked, isRecording:', state.isRecording);
        if (state.isRecording) {
            handleStopClick(els, state);
        }
        else {
            handleStartClick(els, state);
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
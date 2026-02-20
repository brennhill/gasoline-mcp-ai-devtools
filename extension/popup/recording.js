/**
 * Purpose: Owns recording.ts runtime behavior and integration logic.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Recording UI Module for Popup
 * Manages recording controls, timer display, and mic permission flow.
 */
import { StorageKey } from '../lib/constants.js';
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
                    setTimeout(() => {
                        saveInfoEl.style.display = 'none';
                    }, 5000);
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
    setTimeout(() => {
        saveInfoEl.style.display = 'none';
    }, 12000);
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
function showMicPermissionPrompt(saveInfoEl, audioMode) {
    chrome.tabs.query({ active: true, currentWindow: true }, (activeTabs) => {
        chrome.storage.local.set({
            [StorageKey.PENDING_MIC_RECORDING]: { audioMode, returnTabId: activeTabs[0]?.id }
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
// #lizard forgives
function tryMicPermissionThenStart(els, state, audioMode) {
    console.log('[Gasoline REC] Popup: trying getUserMedia from popup...');
    navigator.mediaDevices
        .getUserMedia({ audio: true })
        .then((micStream) => {
        console.log('[Gasoline REC] Popup: getUserMedia succeeded from popup');
        micStream.getTracks().forEach((t) => t.stop());
        chrome.storage.local.set({ [StorageKey.MIC_GRANTED]: true });
        sendRecordStart(els, state, audioMode);
    })
        .catch((err) => {
        console.log('[Gasoline REC] Popup: getUserMedia FAILED:', err.name, err.message);
        chrome.storage.local.remove(StorageKey.MIC_GRANTED);
        showIdle(els, state);
        if (els.saveInfoEl)
            showMicPermissionPrompt(els.saveInfoEl, audioMode);
    });
}
function handleStartClick(els, state) {
    const audioSelect = document.getElementById('record-audio-mode');
    const audioMode = audioSelect?.value ?? '';
    chrome.storage.local.set({ [StorageKey.RECORD_AUDIO_PREF]: audioMode });
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
export function setupRecordingUI() {
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
    row.style.visibility = 'hidden';
    chrome.storage.local.get(StorageKey.RECORDING, (result) => {
        const rec = result[StorageKey.RECORDING];
        console.log('[Gasoline REC] Popup: gasoline_recording from storage:', rec);
        if (rec?.active && rec.name && rec.startTime) {
            console.log('[Gasoline REC] Popup: resuming recording UI for', rec.name);
            showRecording(els, state, rec.name, rec.startTime);
        }
        row.style.visibility = 'visible';
    });
    chrome.storage.onChanged.addListener((changes, areaName) => {
        if (areaName === 'local' && changes[StorageKey.RECORDING]) {
            const rec = changes[StorageKey.RECORDING].newValue;
            console.log('[Gasoline REC] Popup: gasoline_recording changed:', rec);
            if (rec?.active && rec.name && rec.startTime) {
                showRecording(els, state, rec.name, rec.startTime);
            }
            else {
                showIdle(els, state);
            }
        }
    });
    chrome.storage.local.get(StorageKey.PENDING_MIC_RECORDING, (result) => {
        const intent = result[StorageKey.PENDING_MIC_RECORDING];
        console.log('[Gasoline REC] Popup: pending_mic_recording intent:', intent);
        if (!intent?.audioMode)
            return;
        console.log('[Gasoline REC] Popup: consuming mic intent, pre-selecting audioMode:', intent.audioMode);
        chrome.storage.local.remove(StorageKey.PENDING_MIC_RECORDING);
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
    chrome.storage.local.get(StorageKey.RECORD_AUDIO_PREF, (result) => {
        const saved = result[StorageKey.RECORD_AUDIO_PREF];
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
//# sourceMappingURL=recording.js.map
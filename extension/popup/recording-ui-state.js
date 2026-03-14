/**
 * Purpose: Pure recording UI state rendering — recording/idle/error/notice display functions.
 * Why: Separates stateless UI rendering from the complex setup/wiring in recording.ts.
 * Docs: docs/features/feature/playback-engine/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 */
const START_LABEL = 'Record screen';
const STOP_LABEL = 'Stop recording';
const HIGHLIGHT_LABEL = '\u25CF \u00AB Click here to record';
const RECENT_RECORDING_START_MS = 8000;
const TOP_NOTICE_DURATION_MS = 4000;
const AUDIO_LABELS = {
    '': 'Video only',
    tab: 'Video + tab audio',
    mic: 'Video + microphone',
    both: 'Video + tab + mic'
};
let topNoticeTimer = null;
export function getRecordSection(els) {
    const closest = els.row.closest;
    if (typeof closest !== 'function')
        return null;
    return closest.call(els.row, '.section');
}
export function applyRecordHighlight(els) {
    const section = getRecordSection(els);
    if (section)
        section.classList.add('record-highlight');
    els.label.textContent = HIGHLIGHT_LABEL;
}
export function removeRecordHighlight(els) {
    const section = getRecordSection(els);
    if (section)
        section.classList.remove('record-highlight');
    if (els.label.textContent === HIGHLIGHT_LABEL) {
        els.label.textContent = START_LABEL;
    }
}
// #lizard forgives
export function showRecording(els, state, name, startTime) {
    const wasRecording = state.isRecording;
    removeRecordHighlight(els);
    state.isRecording = true;
    els.row.classList.add('is-recording');
    els.label.textContent = STOP_LABEL;
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
    if (!wasRecording && Date.now() - startTime <= RECENT_RECORDING_START_MS) {
        showTopNotice(els, 'Recording started');
    }
}
export function showIdle(els, state) {
    state.isRecording = false;
    removeRecordHighlight(els);
    els.row.classList.remove('is-recording');
    els.label.textContent = START_LABEL;
    els.statusEl.textContent = '';
    if (els.optionsEl)
        els.optionsEl.style.display = 'block';
    if (state.timerInterval) {
        clearInterval(state.timerInterval);
        state.timerInterval = null;
    }
}
export function describePendingRecording(pending) {
    const parts = [];
    if (pending.name)
        parts.push(`Name: ${pending.name}`);
    if (typeof pending.fps === 'number')
        parts.push(`FPS: ${pending.fps}`);
    const audioLabel = AUDIO_LABELS[pending.audio ?? ''] ?? AUDIO_LABELS[''];
    parts.push(`Mode: ${audioLabel}`);
    return parts.join(' \u00b7 ');
}
export function setApprovalPendingState(els, approvalEls, state, pending) {
    const setRowAriaDisabled = (value) => {
        const setAttr = els.row.setAttribute;
        const removeAttr = els.row.removeAttribute;
        if (value !== null) {
            if (typeof setAttr === 'function')
                setAttr.call(els.row, 'aria-disabled', value);
            return;
        }
        if (typeof removeAttr === 'function')
            removeAttr.call(els.row, 'aria-disabled');
    };
    const approvalPending = Boolean(pending && !pending.highlight && !state.isRecording);
    if (approvalPending) {
        if (approvalEls.detail && pending)
            approvalEls.detail.textContent = describePendingRecording(pending);
        if (approvalEls.card)
            approvalEls.card.style.display = 'block';
        els.row.classList.add('is-disabled');
        setRowAriaDisabled('true');
        if (els.optionsEl)
            els.optionsEl.style.display = 'none';
        return;
    }
    if (approvalEls.detail)
        approvalEls.detail.textContent = '';
    if (approvalEls.card)
        approvalEls.card.style.display = 'none';
    els.row.classList.remove('is-disabled');
    setRowAriaDisabled(null);
    if (!state.isRecording && els.optionsEl)
        els.optionsEl.style.display = 'block';
}
export function showTopNotice(els, text) {
    const notice = els.topNoticeEl;
    if (!notice)
        return;
    notice.textContent = text;
    notice.style.display = 'block';
    if (topNoticeTimer)
        clearTimeout(topNoticeTimer);
    topNoticeTimer = setTimeout(() => {
        notice.style.display = 'none';
    }, TOP_NOTICE_DURATION_MS);
}
export function showSavedLink(saveInfoEl, displayName, filePath) {
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
            chrome.runtime.sendMessage({ type: 'reveal_file', path: filePath }, (result) => {
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
export function showSaveResult(saveInfoEl, resp) {
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
export function showStartError(saveInfoEl, errorText) {
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
//# sourceMappingURL=recording-ui-state.js.map
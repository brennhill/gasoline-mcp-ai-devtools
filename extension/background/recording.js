/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
// recording.ts — Recording lifecycle management (start/stop) and state.
// Delegates tab capture / offscreen plumbing to recording-capture.ts and
// chrome runtime listener registration to recording-listeners.ts.
import { getServerUrl } from './state.js';
import { pingContentScript, waitForTabLoad } from './event-listeners.js';
import { scaleTimeout } from '../lib/timeouts.js';
import { StorageKey } from '../lib/constants.js';
import { ensureOffscreenDocument, getStreamIdWithRecovery, requestRecordingGesture } from './recording-capture.js';
import { installRecordingListeners } from './recording-listeners.js';
const defaultState = {
    active: false,
    name: '',
    startTime: 0,
    fps: 15,
    audioMode: '',
    tabId: 0,
    url: '',
    queryId: ''
};
let recordingState = { ...defaultState };
const LOG = '[Gasoline REC]';
/** Listener to re-send watermark when recording tab navigates or content script re-injects. */
let tabUpdateListener = null;
// Clear stale recording state from previous session (e.g., browser crash during recording)
if (typeof chrome !== 'undefined' && chrome.storage?.local?.remove) {
    console.log(LOG, 'Module loaded, clearing stale gasoline_recording from storage');
    chrome.storage.local.remove(StorageKey.RECORDING).catch(() => { });
}
// =============================================================================
// STATE QUERIES
// =============================================================================
/** Returns whether a recording is currently active. */
export function isRecording() {
    return recordingState.active;
}
/** Returns current recording info for popup sync. */
export function getRecordingInfo() {
    return {
        active: recordingState.active,
        name: recordingState.name,
        startTime: recordingState.startTime
    };
}
// =============================================================================
// INTERNAL HELPERS
// =============================================================================
async function clearRecordingState() {
    recordingState = { ...defaultState };
    if (tabUpdateListener) {
        chrome.tabs.onUpdated.removeListener(tabUpdateListener);
        tabUpdateListener = null;
    }
    await chrome.storage.local.remove(StorageKey.RECORDING);
}
// =============================================================================
// LIFECYCLE — START
// =============================================================================
/**
 * Start recording a target tab (or the active tab when no target is provided).
 * @param name — Pre-generated filename from the Go server (e.g., "checkout-bug--2026-02-07-1423")
 * @param fps — Framerate (5–60, default 15)
 * @param queryId — PendingQuery ID for result resolution
 * @param audio — Audio mode: 'tab', 'mic', 'both', or '' (no audio)
 * @param fromPopup — true when initiated from popup (activeTab already granted, skip reload)
 */
// #lizard forgives
export async function startRecording(name, fps = 15, queryId = '', audio = '', fromPopup = false, targetTabId) {
    console.log(LOG, 'startRecording called', {
        name,
        fps,
        queryId,
        audio,
        fromPopup,
        targetTabId: targetTabId ?? null,
        currentlyActive: recordingState.active
    });
    if (recordingState.active) {
        console.warn(LOG, 'START BLOCKED: already recording', { currentState: { ...recordingState } });
        return { status: 'error', name: '', error: 'RECORD_START: Already recording. Stop current recording first.' };
    }
    // Mark active immediately to prevent TOCTOU race across awaits
    recordingState.active = true; // eslint-disable-line require-atomic-updates
    console.log(LOG, 'Marked active=true (TOCTOU guard)');
    // Clamp fps
    fps = Math.max(5, Math.min(60, fps));
    try {
        // Resolve target tab. MCP flows may provide an explicit tab_id.
        let tab;
        if (targetTabId && targetTabId > 0) {
            try {
                tab = await chrome.tabs.get(targetTabId);
            }
            catch {
                recordingState.active = false; // eslint-disable-line require-atomic-updates
                console.error(LOG, 'START FAILED: target tab not found', { targetTabId });
                return { status: 'error', name: '', error: `RECORD_START: Target tab ${targetTabId} not found.` };
            }
        }
        else {
            const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
            tab = tabs[0];
        }
        console.log(LOG, 'Resolved recording tab:', {
            requestedTabId: targetTabId ?? null,
            resolvedTabId: tab?.id,
            url: tab?.url?.substring(0, 80),
            title: tab?.title?.substring(0, 40)
        });
        if (!tab?.id) {
            recordingState.active = false; // eslint-disable-line require-atomic-updates
            console.error(LOG, 'START FAILED: No active tab found');
            return { status: 'error', name: '', error: 'RECORD_START: No active tab found.' };
        }
        // Auto-enable tab tracking if not already tracked
        const storage = await chrome.storage.local.get(StorageKey.TRACKED_TAB_ID);
        console.log(LOG, 'Tracked tab:', { trackedTabId: storage[StorageKey.TRACKED_TAB_ID], willAutoTrack: !storage[StorageKey.TRACKED_TAB_ID] });
        if (!storage[StorageKey.TRACKED_TAB_ID]) {
            await chrome.storage.local.set({
                [StorageKey.TRACKED_TAB_ID]: tab.id,
                [StorageKey.TRACKED_TAB_URL]: tab.url ?? '',
                [StorageKey.TRACKED_TAB_TITLE]: tab.title ?? ''
            });
        }
        // Ensure content script is responsive (needed for toasts + watermark).
        // Skip when from popup — tab reload would close the popup.
        if (!fromPopup) {
            console.log(LOG, 'Pinging content script on tab', tab.id);
            const alive = await pingContentScript(tab.id);
            console.log(LOG, 'Content script alive:', alive);
            if (!alive) {
                console.log(LOG, 'Reloading tab for content script injection');
                chrome.tabs.reload(tab.id);
                await waitForTabLoad(tab.id, scaleTimeout(5000));
            }
            // Add extra delay to ensure extension is fully initialized for tabCapture
            console.log(LOG, 'Waiting for extension to fully initialize...');
            await new Promise((r) => setTimeout(r, scaleTimeout(1000)));
        }
        else {
            console.log(LOG, 'Skipping content script ping (fromPopup=true)');
        }
        let streamId;
        if (fromPopup) {
            // Popup click grants activeTab — get stream directly with targetTabId
            console.log(LOG, 'Getting stream ID via fromPopup path (targetTabId:', tab.id, ')');
            streamId = await getStreamIdWithRecovery(tab.id);
        }
        else {
            // MCP-initiated: requires activeTab via user gesture
            const mediaType = audio ? 'Audio' : 'Video';
            const gestureResult = await requestRecordingGesture(tab, name, fps, audio, mediaType);
            if (gestureResult.error) {
                recordingState.active = false; // eslint-disable-line require-atomic-updates
                return gestureResult;
            }
            streamId = await new Promise((resolve, reject) => {
                chrome.tabCapture.getMediaStreamId({ targetTabId: tab.id }, (id) => {
                    if (chrome.runtime.lastError) {
                        reject(new Error(chrome.runtime.lastError.message ?? 'getMediaStreamId failed'));
                    }
                    else {
                        resolve(id);
                    }
                });
            });
        }
        if (!streamId) {
            recordingState.active = false; // eslint-disable-line require-atomic-updates
            console.error(LOG, 'START FAILED: streamId is empty');
            return {
                status: 'error',
                name: '',
                error: 'RECORD_START: getMediaStreamId returned empty. Check tabCapture permission.'
            };
        }
        // Ensure the offscreen document is running
        console.log(LOG, 'Ensuring offscreen document exists');
        await ensureOffscreenDocument();
        console.log(LOG, 'Offscreen document ready, sending START command');
        // Send start command to offscreen document and wait for confirmation (10s timeout)
        const startResult = await new Promise((resolve) => {
            const timeout = setTimeout(() => {
                chrome.runtime.onMessage.removeListener(listener);
                resolve({
                    target: 'background',
                    type: 'OFFSCREEN_RECORDING_STARTED',
                    success: false,
                    error: 'RECORD_START: Offscreen document timed out.'
                });
            }, scaleTimeout(10000));
            const listener = (message) => {
                if (message.target === 'background' && message.type === 'OFFSCREEN_RECORDING_STARTED') {
                    clearTimeout(timeout);
                    chrome.runtime.onMessage.removeListener(listener);
                    resolve(message);
                }
            };
            chrome.runtime.onMessage.addListener(listener);
            chrome.runtime.sendMessage({
                target: 'offscreen',
                type: 'OFFSCREEN_START_RECORDING',
                streamId,
                serverUrl: getServerUrl(),
                name,
                fps,
                audioMode: audio,
                tabId: tab.id,
                url: tab.url ?? ''
            });
        });
        console.log(LOG, 'Offscreen START result:', { success: startResult.success, error: startResult.error });
        if (!startResult.success) {
            recordingState.active = false; // eslint-disable-line require-atomic-updates
            console.error(LOG, 'START FAILED: offscreen rejected:', startResult.error);
            return {
                status: 'error',
                name: '',
                error: startResult.error ?? 'RECORD_START: Offscreen document failed to start recording.'
            };
        }
        /* eslint-disable require-atomic-updates */
        recordingState = {
            active: true,
            name,
            startTime: Date.now(),
            fps,
            audioMode: audio,
            tabId: tab.id,
            url: tab.url ?? '',
            queryId
        };
        /* eslint-enable require-atomic-updates */
        // Persist state flag for popup sync
        await chrome.storage.local.set({
            [StorageKey.RECORDING]: { active: true, name, startTime: Date.now() }
        });
        // Show "Recording started" toast (fades after 2s)
        chrome.tabs
            .sendMessage(tab.id, {
            type: 'GASOLINE_ACTION_TOAST',
            text: 'Recording started',
            state: 'success',
            duration_ms: scaleTimeout(2000)
        })
            .catch(() => { });
        // Show recording watermark overlay in the page
        chrome.tabs.sendMessage(tab.id, { type: 'GASOLINE_RECORDING_WATERMARK', visible: true }).catch(() => { });
        // Re-send watermark when recording tab navigates or content script re-injects
        if (tabUpdateListener)
            chrome.tabs.onUpdated.removeListener(tabUpdateListener);
        tabUpdateListener = (updatedTabId, changeInfo) => {
            if (updatedTabId === recordingState.tabId && changeInfo.status === 'complete' && recordingState.active) {
                chrome.tabs.sendMessage(updatedTabId, { type: 'GASOLINE_RECORDING_WATERMARK', visible: true }).catch(() => { });
            }
        };
        chrome.tabs.onUpdated.addListener(tabUpdateListener);
        console.log(LOG, 'Recording STARTED successfully', { name, tabId: tab.id, audioMode: audio, fps });
        return { status: 'recording', name, startTime: recordingState.startTime };
    }
    catch (err) {
        recordingState.active = false; // eslint-disable-line require-atomic-updates
        console.error(LOG, 'START EXCEPTION:', err.message, err.stack);
        return {
            status: 'error',
            name: '',
            error: `RECORD_START: ${err.message || 'Failed to start recording.'}`
        };
    }
}
// =============================================================================
// LIFECYCLE — STOP
// =============================================================================
/**
 * Stop recording and save the video.
 * @param truncated — true if auto-stopped due to memory guard or tab close
 */
// #lizard forgives
export async function stopRecording(truncated = false) {
    console.log(LOG, 'stopRecording called', {
        currentlyActive: recordingState.active,
        name: recordingState.name,
        tabId: recordingState.tabId,
        truncated
    });
    if (!recordingState.active) {
        // Clean up stale storage in case of zombie recording state (e.g., service worker restarted)
        console.warn(LOG, 'STOP: No active recording in memory — cleaning up zombie storage');
        chrome.storage.local.remove(StorageKey.RECORDING).catch(() => { });
        return { status: 'error', name: '', error: 'RECORD_STOP: No active recording.' };
    }
    const { tabId } = recordingState;
    // Mark as no longer active immediately to prevent double-stop
    recordingState.active = false;
    console.log(LOG, 'Marked active=false, sending STOP to offscreen');
    // Hide recording watermark overlay
    if (tabId) {
        chrome.tabs.sendMessage(tabId, { type: 'GASOLINE_RECORDING_WATERMARK', visible: false }).catch(() => { });
    }
    try {
        // Send stop command to offscreen document and wait for result (30s timeout for upload)
        const stopResult = await new Promise((resolve) => {
            const timeout = setTimeout(() => {
                chrome.runtime.onMessage.removeListener(listener);
                resolve({
                    target: 'background',
                    type: 'OFFSCREEN_RECORDING_STOPPED',
                    status: 'error',
                    name: recordingState.name || '',
                    error: 'RECORD_STOP: Offscreen document timed out during save.'
                });
            }, scaleTimeout(30000));
            const listener = (message) => {
                if (message.target === 'background' && message.type === 'OFFSCREEN_RECORDING_STOPPED') {
                    clearTimeout(timeout);
                    chrome.runtime.onMessage.removeListener(listener);
                    resolve(message);
                }
            };
            chrome.runtime.onMessage.addListener(listener);
            chrome.runtime.sendMessage({
                target: 'offscreen',
                type: 'OFFSCREEN_STOP_RECORDING'
            });
        });
        console.log(LOG, 'Offscreen STOP result:', {
            status: stopResult.status,
            name: stopResult.name,
            error: stopResult.error,
            size: stopResult.size_bytes,
            path: stopResult.path
        });
        await clearRecordingState();
        // Show save toast on the recorded tab
        if (tabId && stopResult.status === 'saved') {
            const sizeMB = stopResult.size_bytes ? (stopResult.size_bytes / (1024 * 1024)).toFixed(1) : '?';
            chrome.tabs
                .sendMessage(tabId, {
                type: 'GASOLINE_ACTION_TOAST',
                text: 'Recording saved',
                detail: `${stopResult.path ?? stopResult.name} (${sizeMB} MB)`,
                state: 'success',
                duration_ms: scaleTimeout(5000)
            })
                .catch(() => { });
        }
        return {
            status: stopResult.status,
            name: stopResult.name,
            duration_seconds: stopResult.duration_seconds,
            size_bytes: stopResult.size_bytes,
            truncated: stopResult.truncated,
            path: stopResult.path,
            error: stopResult.error
        };
    }
    catch (err) {
        console.error(LOG, 'STOP EXCEPTION:', err.message, err.stack);
        await clearRecordingState();
        return {
            status: 'error',
            name: recordingState.name || '',
            error: `RECORD_STOP: ${err.message || 'Failed to stop recording.'}`
        };
    }
}
// =============================================================================
// CHROME RUNTIME LISTENERS (delegated to recording-listeners.ts)
// =============================================================================
// Guard: all top-level event listeners require chrome runtime (not available in test contexts)
if (typeof chrome !== 'undefined' && chrome.runtime?.onMessage) {
    installRecordingListeners({
        startRecording,
        stopRecording,
        isActive: () => recordingState.active,
        getTabId: () => recordingState.tabId,
        setInactive: () => {
            recordingState.active = false;
        },
        clearRecordingState,
        getServerUrl: () => getServerUrl()
    });
}
//# sourceMappingURL=recording.js.map
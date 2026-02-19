// recording-capture.ts — Tab capture stream acquisition, offscreen document management, and user gesture flow.
// Extracted from recording.ts to separate media plumbing from recording lifecycle.
import { scaleTimeout } from '../lib/timeouts.js';
const LOG = '[Gasoline REC]';
/** Ensure the offscreen document exists for recording. */
export async function ensureOffscreenDocument() {
    // Check if an offscreen document already exists
    const contexts = await chrome.runtime.getContexts({
        contextTypes: [chrome.runtime.ContextType.OFFSCREEN_DOCUMENT]
    });
    if (contexts.length > 0)
        return;
    await chrome.offscreen.createDocument({
        url: 'offscreen.html',
        reasons: [chrome.offscreen.Reason.USER_MEDIA],
        justification: 'Tab video recording via MediaRecorder'
    });
}
/**
 * Get a media stream ID, recovering from "active stream" errors by closing the
 * stale offscreen document (which releases leaked streams) and retrying once.
 */
export async function getStreamIdWithRecovery(tabId) {
    try {
        return await getStreamId(tabId);
    }
    catch (err) {
        if (err.message?.includes('active stream')) {
            console.warn(LOG, 'Active stream detected — closing offscreen document to release leaked streams');
            try {
                await chrome.offscreen.closeDocument();
            }
            catch {
                /* might not exist */
            }
            // Brief pause to let Chrome release the capture
            await new Promise((r) => setTimeout(r, scaleTimeout(200)));
            console.log(LOG, 'Retrying getMediaStreamId after cleanup');
            return await getStreamId(tabId);
        }
        throw err;
    }
}
/** Wrapper around chrome.tabCapture.getMediaStreamId with logging. */
function getStreamId(tabId) {
    return new Promise((resolve, reject) => {
        chrome.tabCapture.getMediaStreamId({ targetTabId: tabId }, (id) => {
            if (chrome.runtime.lastError) {
                console.error(LOG, 'getMediaStreamId FAILED:', chrome.runtime.lastError.message);
                reject(new Error(chrome.runtime.lastError.message ?? 'getMediaStreamId failed'));
            }
            else {
                console.log(LOG, 'Got stream ID:', id?.substring(0, 20) + '...');
                resolve(id);
            }
        });
    });
}
/**
 * Request user gesture for recording permission (used for MCP-initiated recordings).
 * Shows a toast prompting the user to click the Gasoline icon.
 */
export async function requestRecordingGesture(tab, name, fps, audio, mediaType) {
    chrome.tabs.update(tab.id, { active: true });
    chrome.tabs
        .sendMessage(tab.id, {
        type: 'GASOLINE_ACTION_TOAST',
        text: `\u2191 Click Gasoline Icon`,
        detail: `Grant ${mediaType.toLowerCase()} recording permission`,
        state: 'audio',
        duration_ms: scaleTimeout(30000)
    })
        .catch(() => { });
    await chrome.storage.local.set({ gasoline_pending_recording: { name, fps, audio, tabId: tab.id, url: tab.url } });
    const gestureGranted = await waitForRecordingGesture(scaleTimeout(30000));
    await chrome.storage.local.remove('gasoline_pending_recording');
    if (!gestureGranted) {
        console.log(LOG, 'GESTURE_TIMEOUT: User did not click the Gasoline icon within 30s');
        chrome.tabs
            .sendMessage(tab.id, {
            type: 'GASOLINE_ACTION_TOAST',
            text: `\u2191 Click Gasoline Icon`,
            detail: `Grant ${mediaType.toLowerCase()} recording permission`,
            state: 'audio',
            duration_ms: scaleTimeout(8000)
        })
            .catch(() => { });
        return {
            status: 'error',
            name: '',
            error: `RECORD_START: ${mediaType} recording requires permission. Click the Gasoline extension icon to grant ${mediaType.toLowerCase()} recording permission, then try again.`
        };
    }
    chrome.tabs
        .sendMessage(tab.id, {
        type: 'GASOLINE_ACTION_TOAST',
        text: 'Recording',
        detail: 'Recording started',
        state: 'success',
        duration_ms: scaleTimeout(2000)
    })
        .catch(() => { });
    return { status: 'ok', name };
}
/** Wait for user to click extension icon (popup sends RECORDING_GESTURE_GRANTED). */
function waitForRecordingGesture(timeoutMs) {
    return new Promise((resolve) => {
        const timeout = setTimeout(() => {
            chrome.runtime.onMessage.removeListener(listener);
            resolve(false);
        }, timeoutMs);
        const listener = (message) => {
            if (message.type === 'RECORDING_GESTURE_GRANTED') {
                clearTimeout(timeout);
                chrome.runtime.onMessage.removeListener(listener);
                resolve(true);
            }
        };
        chrome.runtime.onMessage.addListener(listener);
    });
}
//# sourceMappingURL=recording-capture.js.map
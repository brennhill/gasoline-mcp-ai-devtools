// recording-listeners.ts — Chrome runtime message listeners for recording.
// Handles popup-initiated record start/stop, auto-stop from offscreen memory guard,
// mic permission grant flow, and file reveal requests.
// Deps are injected to avoid circular imports with recording.ts.
import { scaleTimeout } from '../lib/timeouts.js';
const LOG = '[Gasoline REC]';
/**
 * Install all chrome.runtime.onMessage listeners for recording.
 * Must be called once at module load time, guarded by chrome runtime availability.
 */
export function installRecordingListeners(deps) {
    /**
     * Listen for unsolicited messages from offscreen (auto-stop from memory guard or tab close).
     */
    chrome.runtime.onMessage.addListener((message, sender) => {
        // Only accept messages from the extension itself
        if (sender.id !== chrome.runtime.id)
            return;
        if (message.target !== 'background' || message.type !== 'OFFSCREEN_RECORDING_STOPPED')
            return;
        // Only handle if we think we're still recording (auto-stop case)
        if (!deps.isActive())
            return;
        console.log(LOG, 'Auto-stop from offscreen (memory guard or tab close)', {
            status: message.status,
            name: message.name
        });
        deps.setInactive();
        const tabId = deps.getTabId();
        if (tabId) {
            chrome.tabs
                .sendMessage(tabId, { type: 'GASOLINE_RECORDING_WATERMARK', visible: false })
                .catch(() => { });
        }
        deps.clearRecordingState().catch(() => { });
    });
    /**
     * Handle popup-initiated record_start / record_stop messages.
     * These are direct chrome.runtime messages from the popup, not MCP pending queries.
     */
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        // Only accept messages from the extension itself (popup)
        if (sender.id !== chrome.runtime.id)
            return false;
        if (message.type === 'record_start') {
            console.log(LOG, 'Popup record_start received', { audio: message.audio });
            chrome.tabs.query({ active: true, currentWindow: true }).then((tabs) => {
                let slug = 'recording';
                try {
                    const hostname = new URL(tabs[0]?.url ?? '').hostname.replace(/^www\./, '');
                    slug =
                        hostname
                            .replace(/[^a-z0-9]/gi, '-')
                            .replace(/-+/g, '-')
                            .replace(/^-|-$/g, '') || 'recording';
                }
                catch {
                    /* use default */
                }
                const audio = message.audio ?? '';
                console.log(LOG, 'Popup record_start \u2192 startRecording', {
                    slug,
                    audio,
                    tabUrl: tabs[0]?.url?.substring(0, 60)
                });
                deps
                    .startRecording(slug, 15, '', audio, true)
                    .then((result) => {
                    console.log(LOG, 'Popup record_start result:', result);
                    sendResponse(result);
                })
                    .catch((err) => {
                    console.error(LOG, 'Popup record_start EXCEPTION:', err);
                    sendResponse({ status: 'error' });
                });
            });
            return true; // async response
        }
        if (message.type === 'record_stop') {
            console.log(LOG, 'Popup record_stop received');
            deps
                .stopRecording()
                .then((result) => {
                console.log(LOG, 'Popup record_stop result:', result);
                sendResponse(result);
            })
                .catch((err) => {
                console.error(LOG, 'Popup record_stop EXCEPTION:', err);
                sendResponse({ status: 'error' });
            });
            return true; // async response
        }
        return false;
    });
    /**
     * Handle MIC_GRANTED_CLOSE_TAB from the mic-permission page.
     * Closes the permission tab, activates the original tab, and shows a guidance toast.
     */
    // #lizard forgives
    chrome.runtime.onMessage.addListener((message, sender) => {
        // Only accept messages from the extension itself
        if (sender.id !== chrome.runtime.id)
            return false;
        if (message.type !== 'MIC_GRANTED_CLOSE_TAB')
            return false;
        console.log(LOG, 'MIC_GRANTED_CLOSE_TAB received from tab', sender.tab?.id);
        // Read the stored return tab before closing the permission tab
        chrome.storage.local.get('gasoline_pending_mic_recording', (result) => {
            const returnTabId = result.gasoline_pending_mic_recording?.returnTabId;
            console.log(LOG, 'Pending mic recording intent:', result.gasoline_pending_mic_recording, 'returnTabId:', returnTabId);
            // Close the permission tab
            if (sender.tab?.id) {
                console.log(LOG, 'Closing permission tab', sender.tab.id);
                chrome.tabs.remove(sender.tab.id).catch(() => { });
            }
            // Activate the original tab and show guidance toast
            if (returnTabId) {
                console.log(LOG, 'Activating return tab', returnTabId);
                chrome.tabs
                    .update(returnTabId, { active: true })
                    .then(() => {
                    console.log(LOG, 'Return tab activated, sending toast in 300ms');
                    // Short delay to let the tab activation settle before sending message
                    setTimeout(() => {
                        console.log(LOG, 'Sending guidance toast to tab', returnTabId);
                        chrome.tabs
                            .sendMessage(returnTabId, {
                            type: 'GASOLINE_ACTION_TOAST',
                            text: 'Mic permission granted',
                            detail: 'Open Gasoline and click Record',
                            state: 'success',
                            duration_ms: scaleTimeout(8000)
                        })
                            .catch((err) => {
                            console.error(LOG, 'Toast send FAILED to tab', returnTabId, ':', err.message);
                        });
                    }, scaleTimeout(300));
                })
                    .catch((err) => {
                    console.error(LOG, 'Tab activation FAILED for tab', returnTabId, ':', err.message);
                });
            }
            else {
                console.warn(LOG, 'No returnTabId found — cannot activate tab or show toast');
            }
        });
        return false;
    });
    /**
     * Handle REVEAL_FILE — opens the file in the OS file manager via the Go server.
     */
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        // Only accept messages from the extension itself
        if (sender.id !== chrome.runtime.id)
            return false;
        if (message.type !== 'REVEAL_FILE' || !message.path)
            return false;
        fetch(`${deps.getServerUrl()}/recordings/reveal`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-Gasoline-Client': 'gasoline-extension' },
            body: JSON.stringify({ path: message.path })
        })
            .then((r) => r.json())
            .then((result) => sendResponse(result))
            .catch((err) => sendResponse({ error: err.message }));
        return true; // async response
    });
}
//# sourceMappingURL=recording-listeners.js.map
/**
 * Purpose: Keyboard shortcut listeners for draw mode, action-sequence recording, and screen recording.
 * Split from event-listeners.ts to keep files under 800 LOC.
 */
// =============================================================================
// RECORDING SHORTCUT TYPES & HELPERS
// =============================================================================
import { errorMessage } from '../lib/error-utils.js';
import { getActiveTab } from './event-listeners.js';
export function buildActionSequenceRecordingName(now = new Date()) {
    const yyyy = now.getFullYear();
    const mm = String(now.getMonth() + 1).padStart(2, '0');
    const dd = String(now.getDate()).padStart(2, '0');
    const hh = String(now.getHours()).padStart(2, '0');
    const min = String(now.getMinutes()).padStart(2, '0');
    const ss = String(now.getSeconds()).padStart(2, '0');
    return `action-sequence--${yyyy}-${mm}-${dd}-${hh}${min}${ss}`;
}
async function sendRecordingShortcutToast(tabId, text, detail, state = 'warning') {
    try {
        await chrome.tabs.sendMessage(tabId, {
            type: 'GASOLINE_ACTION_TOAST',
            text,
            detail,
            state,
            duration_ms: 3500
        });
    }
    catch {
        // Tab may not have content script yet.
    }
}
function buildScreenRecordingSlug(url) {
    try {
        const hostname = new URL(url ?? '').hostname.replace(/^www\./, '');
        return (hostname
            .replace(/[^a-z0-9]/gi, '-')
            .replace(/-+/g, '-')
            .replace(/^-|-$/g, '') || 'recording');
    }
    catch {
        return 'recording';
    }
}
export async function toggleScreenRecording(handlers, tab, logFn) {
    if (handlers.isRecording()) {
        const result = await handlers.stopRecording();
        if (result.status === 'saved') {
            try {
                await chrome.tabs.sendMessage(tab.id, {
                    type: 'GASOLINE_ACTION_TOAST',
                    text: 'Recording saved',
                    detail: result.name || '',
                    state: 'success',
                    duration_ms: 3000
                });
            }
            catch { /* content script may not be loaded */ }
        }
        return;
    }
    const slug = buildScreenRecordingSlug(tab.url);
    const result = await handlers.startRecording(slug, 15, '', '', true, tab.id);
    if (result.status !== 'recording' && tab.id) {
        try {
            await chrome.tabs.sendMessage(tab.id, {
                type: 'GASOLINE_ACTION_TOAST',
                text: 'Recording failed',
                detail: result.error || 'Could not start screen recording',
                state: 'error',
                duration_ms: 4000
            });
        }
        catch { /* content script may not be loaded */ }
        if (logFn)
            logFn(`Screen recording start failed: ${result.error}`);
    }
}
// =============================================================================
// DRAW MODE KEYBOARD SHORTCUT
// =============================================================================
/**
 * Install keyboard shortcut listener for draw mode toggle (Ctrl+Shift+D / Cmd+Shift+D).
 * Sends GASOLINE_DRAW_MODE_START or GASOLINE_DRAW_MODE_STOP to the active tab's content script.
 */
export function installDrawModeCommandListener(logFn) {
    if (typeof chrome === 'undefined' || !chrome.commands)
        return;
    chrome.commands.onCommand.addListener(async (command) => {
        if (command !== 'toggle_draw_mode')
            return;
        try {
            const tab = await getActiveTab();
            if (!tab?.id)
                return;
            try {
                const result = (await chrome.tabs.sendMessage(tab.id, {
                    type: 'GASOLINE_GET_ANNOTATIONS'
                }));
                if (result?.draw_mode_active) {
                    await chrome.tabs.sendMessage(tab.id, { type: 'GASOLINE_DRAW_MODE_STOP' });
                }
                else {
                    await chrome.tabs.sendMessage(tab.id, {
                        type: 'GASOLINE_DRAW_MODE_START',
                        started_by: 'user'
                    });
                }
            }
            catch {
                // Content script not loaded -- try activating anyway
                try {
                    await chrome.tabs.sendMessage(tab.id, {
                        type: 'GASOLINE_DRAW_MODE_START',
                        started_by: 'user'
                    });
                }
                catch {
                    if (logFn)
                        logFn('Cannot reach content script for draw mode toggle');
                    try {
                        await chrome.tabs.sendMessage(tab.id, {
                            type: 'GASOLINE_ACTION_TOAST',
                            text: 'Draw mode unavailable',
                            detail: 'Refresh the page and try again',
                            state: 'error',
                            duration_ms: 3000
                        });
                    }
                    catch {
                        // Tab truly unreachable
                    }
                }
            }
        }
        catch (err) {
            if (logFn)
                logFn(`Draw mode keyboard shortcut error: ${errorMessage(err)}`);
        }
    });
}
// =============================================================================
// ACTION-SEQUENCE RECORDING SHORTCUT
// =============================================================================
/**
 * Install keyboard shortcut listener for action-sequence recording toggle.
 * Shortcut is defined in manifest as `toggle_action_sequence_recording`.
 */
export function installRecordingShortcutCommandListener(handlers, logFn) {
    if (typeof chrome === 'undefined' || !chrome.commands)
        return;
    chrome.commands.onCommand.addListener(async (command) => {
        if (command !== 'toggle_action_sequence_recording')
            return;
        try {
            const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
            const tab = tabs[0];
            if (!tab?.id)
                return;
            if (handlers.isRecording()) {
                const stopResult = await handlers.stopRecording(false);
                if (stopResult.status !== 'saved' && stopResult.status !== 'stopped') {
                    await sendRecordingShortcutToast(tab.id, 'Stop recording failed', stopResult.error || 'Could not stop action sequence recording', 'error');
                }
                return;
            }
            const name = buildActionSequenceRecordingName();
            const startResult = await handlers.startRecording(name, 15, '', '', true, tab.id);
            if (startResult.status !== 'recording') {
                await sendRecordingShortcutToast(tab.id, 'Start recording failed', startResult.error || 'Open the extension popup and try Record action sequence', 'error');
            }
        }
        catch (err) {
            if (logFn)
                logFn(`Recording shortcut error: ${errorMessage(err)}`);
        }
    });
}
// =============================================================================
// SCREEN RECORDING KEYBOARD SHORTCUT
// =============================================================================
/**
 * Install keyboard shortcut listener for screen recording toggle (Alt+Shift+R).
 */
export function installScreenRecordingCommandListener(handlers, logFn) {
    if (typeof chrome === 'undefined' || !chrome.commands)
        return;
    chrome.commands.onCommand.addListener(async (command) => {
        if (command !== 'toggle_screen_recording')
            return;
        try {
            const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
            const tab = tabs[0];
            if (!tab?.id)
                return;
            await toggleScreenRecording(handlers, tab, logFn);
        }
        catch (err) {
            if (logFn)
                logFn(`Screen recording shortcut error: ${errorMessage(err)}`);
        }
    });
}
//# sourceMappingURL=keyboard-shortcuts.js.map
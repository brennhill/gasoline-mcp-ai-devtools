// runtime-message-listener.ts — Message routing between background and content contexts.
import { SettingName } from '../lib/constants.js';
import { isValidBackgroundSender, handlePing, handleToggleMessage, forwardHighlightMessage, handleStateCommand, handleExecuteJs, handleExecuteQuery, handleA11yQuery, handleDomQuery, handleGetNetworkWaterfall, handleLinkHealthQuery, handleComputedStylesQuery, handleFormDiscoveryQuery } from './message-handlers.js';
import { showActionToast } from './ui/toast.js';
import { showSubtitle, toggleRecordingWatermark } from './ui/subtitle.js';
// Toggle state caches — updated by forwarded setting messages from background
let actionToastsEnabled = true;
let subtitlesEnabled = true;
/**
 * Initialize runtime message listener
 * Listens for messages from background (feature toggles and pilot commands)
 */
export function initRuntimeMessageListener() {
    // Load overlay toggle states from storage
    chrome.storage.local.get(['actionToastsEnabled', 'subtitlesEnabled'], (result) => {
        if (result.actionToastsEnabled !== undefined)
            actionToastsEnabled = result.actionToastsEnabled;
        if (result.subtitlesEnabled !== undefined)
            subtitlesEnabled = result.subtitlesEnabled;
    });
    const syncHandlers = {
        GASOLINE_PING: () => {
            /* handled below via sendResponse */
        },
        GASOLINE_ACTION_TOAST: (msg) => {
            if (!actionToastsEnabled)
                return false;
            const m = msg;
            if (m.text)
                showActionToast(m.text, m.detail, m.state || 'trying', m.duration_ms);
            return false;
        },
        GASOLINE_RECORDING_WATERMARK: (msg) => {
            toggleRecordingWatermark(msg.visible ?? false);
            return false;
        },
        GASOLINE_SUBTITLE: (msg) => {
            if (!subtitlesEnabled)
                return false;
            showSubtitle(msg.text ?? '');
            return false;
        },
        [SettingName.ACTION_TOASTS]: (msg) => {
            actionToastsEnabled = msg.enabled;
            return false;
        },
        [SettingName.SUBTITLES]: (msg) => {
            subtitlesEnabled = msg.enabled;
            return false;
        }
    };
    const delegatedHandlers = {
        GASOLINE_DRAW_MODE_START: (msg, sr) => {
            const m = msg;
            import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
                .then((mod) => {
                const result = mod.activateDrawMode(m.started_by || 'user', m.annot_session_name || '', m.correlation_id || '');
                sr(result);
            })
                .catch((e) => sr({ error: 'draw_mode_load_failed', message: e.message }));
            return true;
        },
        GASOLINE_DRAW_MODE_STOP: (_msg, sr) => {
            import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
                .then((mod) => {
                const result = mod.deactivateAndSendResults?.() || mod.deactivateDrawMode?.();
                sr(result || { status: 'stopped' });
            })
                .catch((e) => sr({ error: 'draw_mode_load_failed', message: e.message }));
            return true;
        },
        GASOLINE_GET_ANNOTATIONS: (_msg, sr) => {
            import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
                .then((mod) => {
                sr({ draw_mode_active: mod.isDrawModeActive?.() ?? false });
            })
                .catch(() => sr({ draw_mode_active: false }));
            return true;
        },
        GASOLINE_HIGHLIGHT: (msg, sr) => {
            forwardHighlightMessage({ params: msg.params })
                .then((r) => sr(r))
                .catch((e) => sr({ success: false, error: e.message }));
            return true;
        },
        GASOLINE_MANAGE_STATE: (msg, sr) => {
            handleStateCommand(msg.params)
                .then((r) => sr(r))
                .catch((e) => sr({ error: e.message }));
            return true;
        },
        GASOLINE_EXECUTE_JS: (msg, sr) => handleExecuteJs(msg.params || {}, sr),
        GASOLINE_EXECUTE_QUERY: (msg, sr) => handleExecuteQuery((msg.params || {}), sr),
        A11Y_QUERY: (msg, sr) => handleA11yQuery((msg.params || {}), sr),
        DOM_QUERY: (msg, sr) => handleDomQuery((msg.params || {}), sr),
        GET_NETWORK_WATERFALL: (_msg, sr) => handleGetNetworkWaterfall(sr),
        LINK_HEALTH_QUERY: (msg, sr) => handleLinkHealthQuery((msg.params ?? {}), sr),
        COMPUTED_STYLES_QUERY: (msg, sr) => handleComputedStylesQuery((msg.params ?? {}), sr),
        FORM_DISCOVERY_QUERY: (msg, sr) => handleFormDiscoveryQuery((msg.params ?? {}), sr)
    };
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        if (!isValidBackgroundSender(sender)) {
            console.warn('[Gasoline] Rejected message from untrusted sender:', sender.id);
            return false;
        }
        // Ping is special: sync handler that needs sendResponse
        if (message.type === 'GASOLINE_PING')
            return handlePing(sendResponse);
        // Try sync handlers first
        const syncHandler = syncHandlers[message.type]; // nosemgrep: unsafe-dynamic-method
        if (syncHandler) {
            syncHandler(message);
            return false;
        }
        // Handle toggle messages (no dispatch needed, always runs)
        handleToggleMessage(message);
        // Try delegated handlers
        const delegated = delegatedHandlers[message.type]; // nosemgrep: unsafe-dynamic-method
        if (delegated)
            return delegated(message, sendResponse);
        return undefined;
    });
}
//# sourceMappingURL=runtime-message-listener.js.map
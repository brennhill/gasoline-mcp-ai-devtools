// runtime-message-listener.ts — Message routing between background and content contexts.
import { SettingName } from '../lib/constants.js';
import { isValidBackgroundSender, handlePing, handleToggleMessage, forwardHighlightMessage, handleStateCommand, handleExecuteJs, handleExecuteQuery, handleA11yQuery, handleDomQuery, handleGetNetworkWaterfall, handleLinkHealthQuery, handleComputedStylesQuery, handleFormDiscoveryQuery, handleFormStateQuery, handleDataTableQuery, handleGetReadable, handleGetMarkdown, handlePageSummary } from './message-handlers.js';
import { getLocals } from '../lib/storage-utils.js';
import { showActionToast } from './ui/toast.js';
import { showSubtitle, toggleRecordingWatermark } from './ui/subtitle.js';
import { toggleChatWidget } from './ui/chat-widget.js';
// Toggle state caches — updated by forwarded setting messages from background
let actionToastsEnabled = true;
let subtitlesEnabled = true;
/**
 * Initialize runtime message listener
 * Listens for messages from background (feature toggles and pilot commands)
 */
export async function initRuntimeMessageListener() {
    // Load overlay toggle states from storage
    const result = await getLocals(['actionToastsEnabled', 'subtitlesEnabled']);
    if (result.actionToastsEnabled !== undefined)
        actionToastsEnabled = result.actionToastsEnabled;
    if (result.subtitlesEnabled !== undefined)
        subtitlesEnabled = result.subtitlesEnabled;
    const syncHandlers = {
        gasoline_ping: () => {
            /* handled below via sendResponse */
        },
        gasoline_action_toast: (msg) => {
            if (!actionToastsEnabled)
                return false;
            const m = msg;
            if (m.text)
                showActionToast(m.text, m.detail, m.state || 'trying', m.duration_ms);
            return false;
        },
        gasoline_toggle_chat: (msg) => {
            toggleChatWidget(msg.client_name);
            return false;
        },
        gasoline_recording_watermark: (msg) => {
            toggleRecordingWatermark(msg.visible ?? false);
            return false;
        },
        gasoline_subtitle: (msg) => {
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
        gasoline_draw_mode_start: (msg, sr) => {
            const m = msg;
            import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
                .then((mod) => {
                const result = mod.activateDrawMode(m.started_by || 'user', m.annot_session_name || '', m.correlation_id || '');
                sr(result);
            })
                .catch((e) => sr({ error: 'draw_mode_load_failed', message: e.message }));
            return true;
        },
        gasoline_draw_mode_stop: (_msg, sr) => {
            import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
                .then((mod) => {
                const result = mod.deactivateAndSendResults?.() || mod.deactivateDrawMode?.();
                sr(result || { status: 'stopped' });
            })
                .catch((e) => sr({ error: 'draw_mode_load_failed', message: e.message }));
            return true;
        },
        gasoline_get_annotations: (_msg, sr) => {
            import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
                .then((mod) => {
                sr({ draw_mode_active: mod.isDrawModeActive?.() ?? false });
            })
                .catch(() => sr({ draw_mode_active: false }));
            return true;
        },
        gasoline_highlight: (msg, sr) => {
            forwardHighlightMessage({ params: msg.params })
                .then((r) => sr(r))
                .catch((e) => sr({ success: false, error: e.message }));
            return true;
        },
        gasoline_manage_state: (msg, sr) => {
            handleStateCommand(msg.params)
                .then((r) => sr(r))
                .catch((e) => sr({ error: e.message }));
            return true;
        },
        gasoline_execute_js: (msg, sr) => handleExecuteJs(msg.params || {}, sr),
        gasoline_execute_query: (msg, sr) => handleExecuteQuery((msg.params || {}), sr),
        a11y_query: (msg, sr) => handleA11yQuery((msg.params || {}), sr),
        dom_query: (msg, sr) => handleDomQuery((msg.params || {}), sr),
        get_network_waterfall: (_msg, sr) => handleGetNetworkWaterfall(sr),
        link_health_query: (msg, sr) => handleLinkHealthQuery((msg.params ?? {}), sr),
        computed_styles_query: (msg, sr) => handleComputedStylesQuery((msg.params ?? {}), sr),
        form_discovery_query: (msg, sr) => handleFormDiscoveryQuery((msg.params ?? {}), sr),
        form_state_query: (msg, sr) => handleFormStateQuery((msg.params ?? {}), sr),
        data_table_query: (msg, sr) => handleDataTableQuery((msg.params ?? {}), sr),
        gasoline_get_readable: (_msg, sr) => handleGetReadable(sr),
        gasoline_get_markdown: (_msg, sr) => handleGetMarkdown(sr),
        gasoline_page_summary: (_msg, sr) => handlePageSummary(sr)
    };
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        if (!isValidBackgroundSender(sender)) {
            console.warn('[Gasoline] Rejected message from untrusted sender:', sender.id);
            return false;
        }
        // Ping is special: sync handler that needs sendResponse
        if (message.type === 'gasoline_ping')
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
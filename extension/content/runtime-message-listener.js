/**
 * @fileoverview Runtime Message Listener Module
 * Handles chrome.runtime messages from background script
 */
import { isValidBackgroundSender, handlePing, handleToggleMessage, forwardHighlightMessage, handleStateCommand, handleExecuteJs, handleExecuteQuery, handleA11yQuery, handleDomQuery, handleGetNetworkWaterfall, handleLinkHealthQuery, } from './message-handlers.js';
import { activateDrawMode, deactivateDrawMode, getAnnotations, getElementDetail, clearAnnotations, isDrawModeActive } from './draw-mode.js';
/** Color themes for each toast state */
const TOAST_THEMES = {
    trying: { bg: 'linear-gradient(135deg, #3b82f6 0%, #2563eb 100%)', shadow: 'rgba(59, 130, 246, 0.4)' },
    success: { bg: 'linear-gradient(135deg, #22c55e 0%, #16a34a 100%)', shadow: 'rgba(34, 197, 94, 0.4)' },
    warning: { bg: 'linear-gradient(135deg, #f59e0b 0%, #d97706 100%)', shadow: 'rgba(245, 158, 11, 0.4)' },
    error: { bg: 'linear-gradient(135deg, #ef4444 0%, #dc2626 100%)', shadow: 'rgba(239, 68, 68, 0.4)' },
    audio: { bg: 'linear-gradient(135deg, #f97316 0%, #ea580c 100%)', shadow: 'rgba(249, 115, 22, 0.5)' },
};
/** Add animation keyframes to document */
function injectToastAnimationStyles() {
    if (document.getElementById('gasoline-toast-animations'))
        return;
    const style = document.createElement('style');
    style.id = 'gasoline-toast-animations';
    style.textContent = `
    @keyframes gasolineArrowBounce {
      0%, 100% { transform: translateY(0) translateX(0); opacity: 1; }
      50% { transform: translateY(-4px) translateX(4px); opacity: 0.7; }
    }
    @keyframes gasolineArrowBounceUp {
      0%, 100% { transform: translateY(0); opacity: 1; }
      50% { transform: translateY(-6px); opacity: 0.7; }
    }
    @keyframes gasolineToastPulse {
      0%, 100% { box-shadow: 0 4px 20px var(--toast-shadow); }
      50% { box-shadow: 0 8px 32px var(--toast-shadow-intense); }
    }
    .gasoline-toast-arrow {
      display: inline-block;
      margin-left: 8px;
      animation: gasolineArrowBounce 1.5s ease-in-out infinite;
    }
    @media (max-width: 767px) {
      .gasoline-toast-arrow {
        animation: gasolineArrowBounceUp 1.5s ease-in-out infinite;
      }
    }
    .gasoline-toast-pulse {
      animation: gasolineToastPulse 2s ease-in-out infinite;
    }
  `;
    document.head.appendChild(style);
}
/** Truncate text to maxLen characters with ellipsis */
function truncateText(text, maxLen) {
    if (text.length <= maxLen)
        return text;
    return text.slice(0, maxLen - 1) + '\u2026';
}
/**
 * Show a brief visual toast overlay for AI actions.
 * Supports color-coded states and structured content with truncation.
 * For audio-related toasts, adds animated arrow pointing to extension icon.
 */
function showActionToast(text, detail, state = 'trying', durationMs = 3000) {
    // Remove existing toast
    const existing = document.getElementById('gasoline-action-toast');
    if (existing)
        existing.remove();
    // Inject animation styles once
    injectToastAnimationStyles();
    const theme = TOAST_THEMES[state] ?? TOAST_THEMES.trying;
    const isAudioPrompt = state === 'audio' || (detail && detail.toLowerCase().includes('audio') && detail.toLowerCase().includes('click'));
    // Detect screen size: small screens < 768px (mobile) vs larger
    const isSmallScreen = typeof window !== 'undefined' && window.innerWidth < 768;
    const arrowChar = isSmallScreen ? '↑' : '↗';
    const toast = document.createElement('div');
    toast.id = 'gasoline-action-toast';
    if (isAudioPrompt) {
        toast.className = 'gasoline-toast-pulse';
    }
    // Build content: label + truncated detail
    const label = document.createElement('span');
    label.textContent = truncateText(text, 30);
    Object.assign(label.style, { fontWeight: '700' });
    toast.appendChild(label);
    if (detail) {
        const sep = document.createElement('span');
        sep.textContent = '  ';
        Object.assign(sep.style, { opacity: '0.6', margin: '0 4px' });
        toast.appendChild(sep);
        const det = document.createElement('span');
        det.textContent = truncateText(detail, 50);
        Object.assign(det.style, { fontWeight: '400', opacity: '0.9' });
        toast.appendChild(det);
    }
    // Add animated arrow for audio prompts (↗ on large screens, ↑ on small screens)
    if (isAudioPrompt) {
        const arrow = document.createElement('span');
        arrow.className = 'gasoline-toast-arrow';
        arrow.textContent = arrowChar;
        Object.assign(arrow.style, {
            fontSize: '16px',
            fontWeight: '700',
            marginLeft: '12px',
            display: 'inline-block',
        });
        toast.appendChild(arrow);
    }
    Object.assign(toast.style, {
        position: 'fixed',
        top: '16px',
        right: isAudioPrompt && !isSmallScreen ? '16px' : 'auto',
        left: isAudioPrompt && isSmallScreen ? '50%' : (isAudioPrompt ? 'auto' : '50%'),
        transform: isAudioPrompt && isSmallScreen ? 'translateX(-50%)' : (isAudioPrompt ? 'none' : 'translateX(-50%)'),
        padding: isAudioPrompt ? '12px 24px' : '8px 20px',
        background: theme.bg,
        color: '#fff',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        fontSize: isAudioPrompt ? '14px' : '13px',
        fontWeight: isAudioPrompt ? '600' : '400',
        borderRadius: '8px',
        boxShadow: `0 4px 20px ${theme.shadow}`,
        zIndex: '2147483647',
        pointerEvents: 'none',
        opacity: '0',
        transition: 'opacity 0.2s ease-in',
        maxWidth: isAudioPrompt ? '320px' : '500px',
        whiteSpace: isAudioPrompt ? 'normal' : 'nowrap',
        overflow: isAudioPrompt ? 'visible' : 'hidden',
        display: 'flex',
        alignItems: 'center',
        gap: '0',
        '--toast-shadow': theme.shadow,
        '--toast-shadow-intense': theme.shadow.replace('0.4)', '0.7)'),
    });
    const target = document.body || document.documentElement;
    if (!target)
        return;
    target.appendChild(toast);
    // Fade in
    requestAnimationFrame(() => {
        toast.style.opacity = '1';
    });
    // Fade out and remove
    setTimeout(() => {
        toast.style.opacity = '0';
        setTimeout(() => toast.remove(), 300);
    }, durationMs);
}
// Toggle state caches — updated by forwarded setting messages from background
let actionToastsEnabled = true;
let subtitlesEnabled = true;
/**
 * Show or update a persistent subtitle bar at the bottom of the viewport.
 * Empty text clears the subtitle.
 */
function showSubtitle(text) {
    const ELEMENT_ID = 'gasoline-subtitle';
    if (!text) {
        // Clear: remove existing element
        const existing = document.getElementById(ELEMENT_ID);
        if (existing) {
            existing.style.opacity = '0';
            setTimeout(() => existing.remove(), 200);
        }
        return;
    }
    let bar = document.getElementById(ELEMENT_ID);
    if (!bar) {
        bar = document.createElement('div');
        bar.id = ELEMENT_ID;
        Object.assign(bar.style, {
            position: 'fixed',
            bottom: '24px',
            left: '50%',
            transform: 'translateX(-50%)',
            width: 'auto',
            maxWidth: '80%',
            padding: '12px 20px',
            background: 'rgba(0, 0, 0, 0.85)',
            color: '#fff',
            fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
            fontSize: '16px',
            lineHeight: '1.4',
            textAlign: 'center',
            borderRadius: '4px',
            zIndex: '2147483646',
            pointerEvents: 'none',
            opacity: '0',
            transition: 'opacity 0.2s ease-in',
            maxHeight: '4.2em', // ~3 lines
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            boxSizing: 'border-box',
        });
        const target = document.body || document.documentElement;
        if (!target)
            return;
        target.appendChild(bar);
    }
    bar.textContent = text;
    // Force reflow so the browser registers opacity:0, then set to 1
    // for the CSS transition. No timer needed — avoids rAF (paused in
    // background tabs) and setTimeout (throttled to 1s in background tabs).
    void bar.offsetHeight;
    bar.style.opacity = '1';
}
/**
 * Show or hide a recording watermark (Gasoline flame icon) in the bottom-right corner.
 * The icon renders at 64x64px with 50% opacity, captured in the tab video.
 */
function toggleRecordingWatermark(visible) {
    const ELEMENT_ID = 'gasoline-recording-watermark';
    if (!visible) {
        const existing = document.getElementById(ELEMENT_ID);
        if (existing) {
            existing.style.opacity = '0';
            setTimeout(() => existing.remove(), 300);
        }
        return;
    }
    // Don't create a duplicate
    if (document.getElementById(ELEMENT_ID))
        return;
    const container = document.createElement('div');
    container.id = ELEMENT_ID;
    Object.assign(container.style, {
        position: 'fixed',
        bottom: '16px',
        right: '16px',
        width: '64px',
        height: '64px',
        opacity: '0',
        transition: 'opacity 0.3s ease-in',
        zIndex: '2147483645',
        pointerEvents: 'none',
    });
    const img = document.createElement('img');
    img.src = chrome.runtime.getURL('icons/icon.svg');
    Object.assign(img.style, { width: '100%', height: '100%', opacity: '0.5' });
    container.appendChild(img);
    const target = document.body || document.documentElement;
    if (!target)
        return;
    target.appendChild(container);
    // Trigger reflow then fade in
    void container.offsetHeight;
    container.style.opacity = '1';
}
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
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        // SECURITY: Validate sender is from the extension background, not from page context
        if (!isValidBackgroundSender(sender)) {
            console.warn('[Gasoline] Rejected message from untrusted sender:', sender.id);
            return false;
        }
        // Handle ping to check if content script is loaded
        if (message.type === 'GASOLINE_PING') {
            return handlePing(sendResponse);
        }
        // Show AI action toast overlay (gated by toggle)
        if (message.type === 'GASOLINE_ACTION_TOAST') {
            if (!actionToastsEnabled)
                return false;
            const msg = message;
            if (msg.text)
                showActionToast(msg.text, msg.detail, msg.state || 'trying', msg.duration_ms);
            return false;
        }
        // Show/hide recording watermark overlay
        if (message.type === 'GASOLINE_RECORDING_WATERMARK') {
            const msg = message;
            toggleRecordingWatermark(msg.visible ?? false);
            return false;
        }
        // Show subtitle overlay (gated by toggle)
        if (message.type === 'GASOLINE_SUBTITLE') {
            if (!subtitlesEnabled)
                return false;
            const msg = message;
            showSubtitle(msg.text ?? '');
            return false;
        }
        // Handle overlay toggle updates from background
        if (message.type === 'setActionToastsEnabled') {
            actionToastsEnabled = message.enabled;
            return false;
        }
        if (message.type === 'setSubtitlesEnabled') {
            subtitlesEnabled = message.enabled;
            return false;
        }
        // Handle toggle messages
        handleToggleMessage(message);
        // Handle GASOLINE_HIGHLIGHT from background
        if (message.type === 'GASOLINE_HIGHLIGHT') {
            forwardHighlightMessage(message)
                .then((result) => {
                sendResponse(result);
            })
                .catch((err) => {
                sendResponse({ success: false, error: err.message });
            });
            return true; // Will respond asynchronously
        }
        // Handle state management commands from background
        if (message.type === 'GASOLINE_MANAGE_STATE') {
            // message.params contains action, state, include_url from the manage_state tool
            // handleStateCommand accepts params with optional action (StateAction), name, state, include_url
            handleStateCommand(message.params)
                .then((result) => sendResponse(result))
                .catch((err) => sendResponse({ error: err.message }));
            return true; // Keep channel open for async response
        }
        // Handle GASOLINE_EXECUTE_JS from background (direct pilot command)
        if (message.type === 'GASOLINE_EXECUTE_JS') {
            const params = message.params || {};
            return handleExecuteJs(params, sendResponse);
        }
        // Handle GASOLINE_EXECUTE_QUERY from background (polling system)
        if (message.type === 'GASOLINE_EXECUTE_QUERY') {
            return handleExecuteQuery(message.params || {}, sendResponse);
        }
        // Handle A11Y_QUERY from background (run accessibility audit in page context)
        if (message.type === 'A11Y_QUERY') {
            return handleA11yQuery(message.params || {}, sendResponse);
        }
        // Handle DOM_QUERY from background (execute CSS selector query in page context)
        if (message.type === 'DOM_QUERY') {
            return handleDomQuery(message.params || {}, sendResponse);
        }
        // Handle GET_NETWORK_WATERFALL from background (collect PerformanceResourceTiming data)
        if (message.type === 'GET_NETWORK_WATERFALL') {
            return handleGetNetworkWaterfall(sendResponse);
        }
        // Handle LINK_HEALTH_QUERY from background (check all links on the page)
        if (message.type === 'LINK_HEALTH_QUERY') {
            const params = message.params || {};
            return handleLinkHealthQuery(params, sendResponse);
        }
        // Draw Mode handlers
        if (message.type === 'GASOLINE_DRAW_MODE_START') {
            const result = activateDrawMode(message.started_by || 'llm', message.session_name || '');
            sendResponse(result);
            return false;
        }
        if (message.type === 'GASOLINE_DRAW_MODE_STOP') {
            const result = deactivateDrawMode();
            sendResponse(result);
            return false;
        }
        if (message.type === 'GASOLINE_GET_ANNOTATIONS') {
            sendResponse({
                annotations: getAnnotations(),
                draw_mode_active: isDrawModeActive(),
                viewport: { width: window.innerWidth, height: window.innerHeight },
            });
            return false;
        }
        if (message.type === 'GASOLINE_GET_ANNOTATION_DETAIL') {
            const detail = getElementDetail(message.correlation_id);
            sendResponse(detail ? { found: true, detail } : { found: false });
            return false;
        }
        if (message.type === 'GASOLINE_CLEAR_ANNOTATIONS') {
            clearAnnotations();
            sendResponse({ success: true });
            return false;
        }
        return undefined;
    });
}
//# sourceMappingURL=runtime-message-listener.js.map
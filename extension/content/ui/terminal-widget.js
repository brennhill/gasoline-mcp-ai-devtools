/**
 * Purpose: In-browser terminal widget that embeds a PTY-backed terminal via iframe.
 * Why: Provides a Lovable-like experience — chat with any CLI (claude, codex, aider) from
 * a browser overlay while seeing code edits reflected via hot reload on the tracked page.
 * Docs: docs/features/feature/terminal/index.md
 */
import { DEFAULT_SERVER_URL, StorageKey } from '../../lib/constants.js';
const WIDGET_ID = 'gasoline-terminal-widget';
const IFRAME_ID = 'gasoline-terminal-iframe';
const HEADER_ID = 'gasoline-terminal-header';
let widgetEl = null;
let iframeEl = null;
let resizeHandleEl = null;
let sessionState = null;
let visible = false;
let minimized = false;
let savedHeight = '';
let serverUrl = DEFAULT_SERVER_URL;
function getServerUrl() {
    return new Promise((resolve) => {
        try {
            chrome.storage.local.get([StorageKey.SERVER_URL], (result) => {
                if (chrome.runtime.lastError) {
                    resolve(DEFAULT_SERVER_URL); // Storage read failed — fall back to default
                    return;
                }
                const url = result[StorageKey.SERVER_URL] || DEFAULT_SERVER_URL;
                serverUrl = url;
                resolve(url);
            });
        }
        catch {
            resolve(DEFAULT_SERVER_URL); // Extension context invalidated
        }
    });
}
function getTerminalConfig() {
    return new Promise((resolve) => {
        try {
            chrome.storage.local.get([StorageKey.TERMINAL_CONFIG], (result) => {
                if (chrome.runtime.lastError) {
                    resolve({}); // Storage read failed — use defaults
                    return;
                }
                const config = result[StorageKey.TERMINAL_CONFIG] || {};
                resolve(config);
            });
        }
        catch {
            resolve({}); // Extension context invalidated
        }
    });
}
function saveTerminalConfig(config) {
    try {
        chrome.storage.local.set({ [StorageKey.TERMINAL_CONFIG]: config }, () => {
            void chrome.runtime.lastError; // Best-effort persistence
        });
    }
    catch {
        // Extension context invalidated — config won't persist but session still works
    }
}
function getTerminalAICommand() {
    return new Promise((resolve) => {
        try {
            chrome.storage.local.get([StorageKey.TERMINAL_AI_COMMAND], (result) => {
                if (chrome.runtime.lastError) {
                    resolve('claude');
                    return;
                }
                const cmd = result[StorageKey.TERMINAL_AI_COMMAND] || 'claude';
                resolve(cmd);
            });
        }
        catch {
            resolve('claude');
        }
    });
}
function getTerminalDevRoot() {
    return new Promise((resolve) => {
        try {
            chrome.storage.local.get([StorageKey.TERMINAL_DEV_ROOT], (result) => {
                if (chrome.runtime.lastError) {
                    resolve('');
                    return;
                }
                resolve(result[StorageKey.TERMINAL_DEV_ROOT] || '');
            });
        }
        catch {
            resolve('');
        }
    });
}
function persistSession(state) {
    try {
        chrome.storage.session.set({ [StorageKey.TERMINAL_SESSION]: state }, () => {
            void chrome.runtime.lastError;
        });
    }
    catch { /* extension context invalidated */ }
}
function clearPersistedSession() {
    try {
        chrome.storage.session.remove([StorageKey.TERMINAL_SESSION, StorageKey.TERMINAL_UI_STATE], () => {
            void chrome.runtime.lastError;
        });
    }
    catch { /* extension context invalidated */ }
}
function persistUIState(uiState) {
    try {
        chrome.storage.session.set({ [StorageKey.TERMINAL_UI_STATE]: uiState }, () => {
            void chrome.runtime.lastError;
        });
    }
    catch { /* extension context invalidated */ }
}
function loadPersistedSession() {
    return new Promise((resolve) => {
        try {
            chrome.storage.session.get([StorageKey.TERMINAL_SESSION, StorageKey.TERMINAL_UI_STATE], (result) => {
                if (chrome.runtime.lastError) {
                    resolve({ session: null, uiState: 'closed' });
                    return;
                }
                const session = result[StorageKey.TERMINAL_SESSION];
                const uiState = result[StorageKey.TERMINAL_UI_STATE] || 'closed';
                resolve({ session: session || null, uiState });
            });
        }
        catch {
            resolve({ session: null, uiState: 'closed' });
        }
    });
}
/** Validate that a persisted token is still alive on the daemon. */
async function validateSession(token) {
    try {
        const url = await getServerUrl();
        const resp = await fetch(`${url}/terminal/validate?token=${encodeURIComponent(token)}`, { signal: AbortSignal.timeout(2000) });
        if (!resp.ok)
            return false;
        const data = await resp.json();
        return data.valid === true;
    }
    catch {
        return false;
    }
}
async function startSession(config) {
    const url = await getServerUrl();
    const aiCommand = await getTerminalAICommand();
    const devRoot = await getTerminalDevRoot();
    try {
        // Build init_command: unset CLAUDECODE to avoid nesting detection, then launch the AI tool.
        const initCommand = aiCommand ? `unset CLAUDECODE 2>/dev/null; ${aiCommand}` : '';
        const resp = await fetch(`${url}/terminal/start`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                cmd: config.cmd || '',
                args: config.args || [],
                dir: config.dir || devRoot || '',
                init_command: initCommand
            })
        });
        if (!resp.ok) {
            const body = await resp.json();
            // Sandbox restriction — show actionable instructions to the user.
            if (resp.status === 503 && body.error === 'sandbox_restricted') {
                showSandboxError(body.message ?? '', body.instruction ?? '', body.command ?? '');
                return null;
            }
            // Session already exists — reconnect using the returned token.
            if (resp.status === 409 && body.token) {
                const state = { sessionId: body.session_id ?? 'default', token: body.token };
                persistSession(state);
                return state;
            }
            console.warn('[Gasoline] Terminal session rejected (HTTP ' + resp.status + '): ' +
                (body.error ?? 'unknown') + '. Check the daemon logs for details.');
            return null;
        }
        const data = await resp.json();
        const state = { sessionId: data.session_id, token: data.token };
        persistSession(state);
        return state;
    }
    catch (err) {
        console.warn('[Gasoline] Terminal session start failed: ' +
            (err instanceof Error ? err.message : String(err)) +
            '. Is the Gasoline daemon running? Start it with: npx gasoline-agentic-browser');
        return null;
    }
}
function showSandboxError(message, instruction, command) {
    // Remove any existing widget/error overlay
    const existing = document.getElementById(WIDGET_ID);
    if (existing)
        existing.remove();
    const overlay = document.createElement('div');
    overlay.id = WIDGET_ID;
    Object.assign(overlay.style, {
        position: 'fixed',
        bottom: '16px',
        right: '16px',
        width: '420px',
        maxWidth: 'calc(100vw - 32px)',
        zIndex: '2147483644',
        background: '#1a1b26',
        border: '1px solid #f7768e',
        borderRadius: '12px',
        padding: '20px',
        boxShadow: '0 8px 32px rgba(0, 0, 0, 0.4)',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        color: '#a9b1d6'
    });
    const title = document.createElement('div');
    title.textContent = 'Terminal Unavailable';
    Object.assign(title.style, {
        fontSize: '14px',
        fontWeight: '600',
        color: '#f7768e',
        marginBottom: '8px'
    });
    const msg = document.createElement('div');
    msg.textContent = message;
    Object.assign(msg.style, {
        fontSize: '12px',
        color: '#787c99',
        marginBottom: '12px',
        lineHeight: '1.4'
    });
    const inst = document.createElement('div');
    inst.textContent = instruction;
    Object.assign(inst.style, {
        fontSize: '12px',
        color: '#a9b1d6',
        marginBottom: '8px'
    });
    const cmdBox = document.createElement('div');
    Object.assign(cmdBox.style, {
        background: '#16161e',
        border: '1px solid #292e42',
        borderRadius: '6px',
        padding: '10px 12px',
        fontFamily: '"SF Mono", "Fira Code", Menlo, Monaco, monospace',
        fontSize: '12px',
        color: '#9ece6a',
        cursor: 'pointer',
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        marginBottom: '12px'
    });
    const cmdText = document.createElement('span');
    cmdText.textContent = command;
    cmdText.style.flex = '1';
    const copyIcon = document.createElement('span');
    copyIcon.textContent = 'Copy';
    Object.assign(copyIcon.style, {
        fontSize: '11px',
        color: '#565f89',
        flexShrink: '0'
    });
    cmdBox.appendChild(cmdText);
    cmdBox.appendChild(copyIcon);
    cmdBox.addEventListener('click', () => {
        void navigator.clipboard.writeText(command).then(() => {
            copyIcon.textContent = 'Copied!';
            copyIcon.style.color = '#9ece6a';
            setTimeout(() => {
                copyIcon.textContent = 'Copy';
                copyIcon.style.color = '#565f89';
            }, 2000);
        }).catch(() => {
            copyIcon.textContent = 'Select & copy manually';
            copyIcon.style.color = '#f7768e';
        });
    });
    const closeBtn = document.createElement('button');
    closeBtn.textContent = 'Dismiss';
    closeBtn.type = 'button';
    Object.assign(closeBtn.style, {
        background: '#292e42',
        border: 'none',
        borderRadius: '6px',
        padding: '6px 16px',
        color: '#a9b1d6',
        fontSize: '12px',
        cursor: 'pointer',
        width: '100%'
    });
    closeBtn.addEventListener('click', () => {
        overlay.remove();
        widgetEl = null;
        visible = false;
    });
    overlay.appendChild(title);
    overlay.appendChild(msg);
    overlay.appendChild(inst);
    overlay.appendChild(cmdBox);
    overlay.appendChild(closeBtn);
    widgetEl = overlay;
    visible = true;
    const target = document.body || document.documentElement;
    if (target)
        target.appendChild(overlay);
}
function createWidget(token) {
    const widget = document.createElement('div');
    widget.id = WIDGET_ID;
    Object.assign(widget.style, {
        position: 'fixed',
        bottom: '0',
        right: '0',
        width: '50vw',
        height: '40vh',
        minWidth: '400px',
        minHeight: '250px',
        maxWidth: '100vw',
        maxHeight: '80vh',
        zIndex: '2147483644',
        display: 'flex',
        flexDirection: 'column',
        borderRadius: '12px 0 0 0',
        overflow: 'hidden',
        boxShadow: '0 -4px 24px rgba(0, 0, 0, 0.3)',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        transition: 'opacity 200ms ease, transform 200ms ease',
        transformOrigin: 'bottom right'
    });
    // Resize handle (top-left corner)
    const resizeHandle = document.createElement('div');
    Object.assign(resizeHandle.style, {
        position: 'absolute',
        top: '0',
        left: '0',
        width: '12px',
        height: '12px',
        cursor: 'nw-resize',
        zIndex: '10'
    });
    setupResize(resizeHandle, widget);
    resizeHandleEl = resizeHandle;
    widget.appendChild(resizeHandle);
    // Header bar
    const header = document.createElement('div');
    header.id = HEADER_ID;
    Object.assign(header.style, {
        height: '32px',
        background: '#16161e',
        display: 'flex',
        alignItems: 'center',
        padding: '0 8px 0 12px',
        gap: '8px',
        borderBottom: '1px solid #292e42',
        cursor: 'default',
        flexShrink: '0'
    });
    // Connection status dot
    const statusDot = document.createElement('span');
    statusDot.className = 'gasoline-terminal-status-dot';
    Object.assign(statusDot.style, {
        width: '8px',
        height: '8px',
        borderRadius: '50%',
        background: '#565f89',
        flexShrink: '0',
        transition: 'background 200ms ease'
    });
    const titleSpan = document.createElement('span');
    titleSpan.textContent = 'Terminal';
    Object.assign(titleSpan.style, {
        color: '#787c99',
        fontSize: '12px',
        fontWeight: '600',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap',
        userSelect: 'none'
    });
    // Minimize button
    const minimizeBtn = document.createElement('button');
    minimizeBtn.textContent = '\u2581'; // ▁
    minimizeBtn.title = 'Minimize terminal';
    minimizeBtn.type = 'button';
    Object.assign(minimizeBtn.style, {
        width: '24px',
        height: '24px',
        border: 'none',
        background: 'transparent',
        color: '#565f89',
        fontSize: '14px',
        cursor: 'pointer',
        borderRadius: '4px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: '0'
    });
    minimizeBtn.addEventListener('mouseenter', () => {
        minimizeBtn.style.background = '#292e42';
        minimizeBtn.style.color = '#a9b1d6';
    });
    minimizeBtn.addEventListener('mouseleave', () => {
        minimizeBtn.style.background = 'transparent';
        minimizeBtn.style.color = '#565f89';
    });
    minimizeBtn.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        toggleMinimize(widget, minimizeBtn, header);
    });
    // Exit session button — kills the PTY. Placed left, next to title, glows red.
    const exitBtn = document.createElement('button');
    exitBtn.textContent = '\u23FB'; // ⏻ power symbol
    exitBtn.title = 'Exit AI session';
    exitBtn.type = 'button';
    Object.assign(exitBtn.style, {
        width: '24px',
        height: '24px',
        border: 'none',
        background: 'transparent',
        color: '#f7768e',
        fontSize: '12px',
        cursor: 'pointer',
        borderRadius: '4px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: '0',
        opacity: '0.7',
        transition: 'opacity 150ms ease, background 150ms ease, box-shadow 150ms ease'
    });
    exitBtn.addEventListener('mouseenter', () => {
        exitBtn.style.background = '#3b1219';
        exitBtn.style.opacity = '1';
        exitBtn.style.boxShadow = '0 0 8px rgba(247, 118, 142, 0.4)';
    });
    exitBtn.addEventListener('mouseleave', () => {
        exitBtn.style.background = 'transparent';
        exitBtn.style.opacity = '0.7';
        exitBtn.style.boxShadow = 'none';
    });
    exitBtn.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        void exitTerminalSession();
    });
    // Spacer pushes minimize/close to the right
    const spacer = document.createElement('div');
    spacer.style.flex = '1';
    const closeBtn = document.createElement('button');
    closeBtn.textContent = '\u2715';
    closeBtn.title = 'Close terminal';
    closeBtn.type = 'button';
    Object.assign(closeBtn.style, {
        width: '24px',
        height: '24px',
        border: 'none',
        background: 'transparent',
        color: '#565f89',
        fontSize: '14px',
        cursor: 'pointer',
        borderRadius: '4px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: '0'
    });
    closeBtn.addEventListener('mouseenter', () => {
        closeBtn.style.background = '#292e42';
        closeBtn.style.color = '#a9b1d6';
    });
    closeBtn.addEventListener('mouseleave', () => {
        closeBtn.style.background = 'transparent';
        closeBtn.style.color = '#565f89';
    });
    closeBtn.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        hideTerminal();
    });
    // Title bar click restores when minimized
    header.addEventListener('click', () => {
        if (!minimized)
            return;
        toggleMinimize(widget, minimizeBtn, header);
    });
    header.appendChild(statusDot);
    header.appendChild(titleSpan);
    header.appendChild(exitBtn);
    header.appendChild(spacer);
    header.appendChild(minimizeBtn);
    header.appendChild(closeBtn);
    // Iframe
    const iframe = document.createElement('iframe');
    iframe.id = IFRAME_ID;
    iframe.src = `${serverUrl}/terminal?token=${encodeURIComponent(token)}`;
    Object.assign(iframe.style, {
        flex: '1',
        width: '100%',
        border: 'none',
        background: '#1a1b26'
    });
    iframe.setAttribute('allow', 'clipboard-write');
    widget.appendChild(header);
    widget.appendChild(iframe);
    iframeEl = iframe;
    // Listen for messages from the terminal iframe
    window.addEventListener('message', handleIframeMessage);
    return widget;
}
function updateStatusDot(state) {
    const dot = widgetEl?.querySelector('.gasoline-terminal-status-dot');
    if (!dot)
        return;
    switch (state) {
        case 'connected':
            dot.style.background = '#9ece6a'; // green
            break;
        case 'disconnected':
            dot.style.background = '#e0af68'; // orange
            break;
        case 'exited':
            dot.style.background = '#f7768e'; // red
            break;
    }
}
function handleIframeMessage(event) {
    if (!event.data || event.data.source !== 'gasoline-terminal')
        return;
    // Only accept messages from the daemon's origin (localhost)
    try {
        const origin = new URL(serverUrl).origin;
        if (event.origin !== origin)
            return;
    }
    catch {
        return; // Malformed serverUrl — reject all messages
    }
    // Handle terminal connection lifecycle events
    switch (event.data.event) {
        case 'connected':
            updateStatusDot('connected');
            break;
        case 'disconnected':
            updateStatusDot('disconnected');
            break;
        case 'exited':
            updateStatusDot('exited');
            break;
    }
}
function setupResize(handle, widget) {
    let startX = 0;
    let startY = 0;
    let startWidth = 0;
    let startHeight = 0;
    function onMouseDown(e) {
        e.preventDefault();
        startX = e.clientX;
        startY = e.clientY;
        startWidth = widget.offsetWidth;
        startHeight = widget.offsetHeight;
        document.addEventListener('mousemove', onMouseMove);
        document.addEventListener('mouseup', onMouseUp);
        // Prevent iframe from stealing mouse events during resize
        if (iframeEl)
            iframeEl.style.pointerEvents = 'none';
    }
    function onMouseMove(e) {
        const newWidth = startWidth - (e.clientX - startX);
        const newHeight = startHeight - (e.clientY - startY);
        widget.style.width = Math.max(400, Math.min(window.innerWidth, newWidth)) + 'px';
        widget.style.height = Math.max(250, Math.min(window.innerHeight * 0.8, newHeight)) + 'px';
    }
    function onMouseUp() {
        document.removeEventListener('mousemove', onMouseMove);
        document.removeEventListener('mouseup', onMouseUp);
        if (iframeEl)
            iframeEl.style.pointerEvents = 'auto';
        // Notify iframe to refit terminal
        notifyIframe('resize');
    }
    handle.addEventListener('mousedown', onMouseDown);
}
function toggleMinimize(widget, btn, header) {
    if (minimized) {
        // Restore
        minimized = false;
        widget.style.height = savedHeight || '40vh';
        widget.style.minHeight = '250px';
        if (iframeEl)
            iframeEl.style.display = 'block';
        if (resizeHandleEl)
            resizeHandleEl.style.display = 'block';
        btn.textContent = '\u2581'; // ▁
        btn.title = 'Minimize terminal';
        header.style.cursor = 'default';
        header.style.borderBottom = '1px solid #292e42';
        notifyIframe('resize');
        persistUIState('open');
    }
    else {
        // Minimize
        minimized = true;
        savedHeight = widget.style.height || '40vh';
        widget.style.height = '32px';
        widget.style.minHeight = '32px';
        if (iframeEl)
            iframeEl.style.display = 'none';
        if (resizeHandleEl)
            resizeHandleEl.style.display = 'none';
        btn.textContent = '\u25A1'; // □
        btn.title = 'Restore terminal';
        header.style.cursor = 'pointer';
        header.style.borderBottom = 'none';
        persistUIState('minimized');
    }
}
function notifyIframe(command, data) {
    if (!iframeEl?.contentWindow)
        return;
    let origin = '*';
    try {
        origin = new URL(serverUrl).origin;
    }
    catch { /* fall back to wildcard */ }
    iframeEl.contentWindow.postMessage({
        target: 'gasoline-terminal',
        command,
        ...data
    }, origin);
}
export function hideTerminal() {
    if (!widgetEl)
        return;
    visible = false;
    widgetEl.style.opacity = '0';
    widgetEl.style.transform = 'translateY(20px) scale(0.98)';
    widgetEl.style.pointerEvents = 'none';
    persistUIState('closed');
    // Session stays alive — can reconnect via toggle or page reload
}
/** Kill the PTY session on the daemon and tear down the widget completely. */
export async function exitTerminalSession() {
    // Stop the PTY on the daemon (with timeout so the UI never hangs).
    if (sessionState) {
        try {
            await fetch(`${serverUrl}/terminal/stop`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id: sessionState.sessionId }),
                signal: AbortSignal.timeout(3000)
            });
        }
        catch { /* daemon unreachable or timeout — tear down locally */ }
    }
    clearPersistedSession();
    unmountTerminal();
}
export function showTerminal() {
    if (!widgetEl)
        return;
    visible = true;
    widgetEl.style.opacity = '1';
    widgetEl.style.transform = 'translateY(0) scale(1)';
    widgetEl.style.pointerEvents = 'auto';
    notifyIframe('focus');
    persistUIState(minimized ? 'minimized' : 'open');
}
export function isTerminalVisible() {
    return visible;
}
export async function toggleTerminal() {
    if (visible && widgetEl) {
        hideTerminal();
        return;
    }
    // If widget exists but hidden, just show it
    if (widgetEl && sessionState) {
        showTerminal();
        return;
    }
    // Try to reconnect to a persisted session first
    await getServerUrl();
    const persisted = await loadPersistedSession();
    if (persisted.session) {
        const alive = await validateSession(persisted.session.token);
        if (alive) {
            sessionState = persisted.session;
            mountWidget(persisted.session.token, persisted.uiState === 'minimized');
            return;
        }
        // Session died — clear stale state and start fresh
        clearPersistedSession();
    }
    // Start a new session
    const config = await getTerminalConfig();
    const state = await startSession(config);
    if (!state)
        return;
    sessionState = state;
    mountWidget(state.token, false);
}
/** Restore terminal on page load if it was previously open/minimized. */
export async function restoreTerminalIfNeeded() {
    const persisted = await loadPersistedSession();
    if (!persisted.session || persisted.uiState === 'closed')
        return;
    await getServerUrl();
    const alive = await validateSession(persisted.session.token);
    if (!alive) {
        clearPersistedSession();
        return;
    }
    sessionState = persisted.session;
    mountWidget(persisted.session.token, persisted.uiState === 'minimized');
}
function mountWidget(token, startMinimized) {
    if (widgetEl) {
        widgetEl.remove();
        widgetEl = null;
    }
    widgetEl = createWidget(token);
    const target = document.body || document.documentElement;
    if (!target)
        return;
    target.appendChild(widgetEl);
    // Animate in
    widgetEl.style.opacity = '0';
    widgetEl.style.transform = 'translateY(20px) scale(0.98)';
    requestAnimationFrame(() => {
        showTerminal();
        // Apply minimized state after show animation
        if (startMinimized) {
            const header = widgetEl?.querySelector('#' + HEADER_ID);
            const minimizeBtn = header?.querySelector('button');
            if (widgetEl && header && minimizeBtn) {
                toggleMinimize(widgetEl, minimizeBtn, header);
            }
        }
    });
}
function unmountTerminal() {
    window.removeEventListener('message', handleIframeMessage);
    if (widgetEl) {
        widgetEl.remove();
        widgetEl = null;
    }
    iframeEl = null;
    resizeHandleEl = null;
    sessionState = null;
    visible = false;
    minimized = false;
    savedHeight = '';
}
/** Write text to the terminal PTY stdin via the iframe postMessage bridge, then press Enter to submit. */
export function writeToTerminal(text) {
    if (!visible || !iframeEl)
        return;
    // Strip trailing whitespace/newlines — we'll send our own Enter to submit.
    const trimmed = text.replace(/[\r\n\s]+$/, '');
    notifyIframe('write', { text: trimmed });
    // Delay must be long enough for the AI CLI's TUI to finish processing the pasted
    // text (especially large multi-line annotations). Send \r (carriage return) which
    // is the actual Enter keypress byte in a terminal — more reliable than \n for TUIs.
    setTimeout(() => {
        notifyIframe('write', { text: '\r' });
    }, 600);
}
// Re-export for launcher integration
export { saveTerminalConfig };
//# sourceMappingURL=terminal-widget.js.map
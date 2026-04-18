/**
 * Purpose: Side panel host for the Kaboom terminal.
 * Why: Removes the terminal from page context so CSP on arbitrary sites cannot
 * interfere with the xterm host, while keeping the session and reconnect model intact.
 * Docs: docs/features/feature/terminal/index.md
 */
import { StorageKey } from './lib/constants.js';
import { onStorageChanged } from './lib/storage-utils.js';
import { state, resetAllState, getTerminalServerUrl, MINIMIZED_WIDGET_HEIGHT, TERMINAL_WRITE_SUBMIT_DELAY_MS, TERMINAL_TYPING_IDLE_MS, TERMINAL_GUARD_POLL_MS, TERMINAL_GUARD_TOAST_INTERVAL_MS } from './content/ui/terminal-widget-types.js';
import { getServerUrl, getTerminalConfig, persistUIState, loadPersistedSession, clearPersistedSession, validateSession, startSession } from './content/ui/terminal-widget-session.js';
import { showActionToast } from './content/ui/toast.js';
import { createWorkspaceShell } from './sidepanel/workspace-shell.js';
import { createWorkspaceTerminalPane } from './sidepanel/workspace-terminal-pane.js';
import { renderWorkspaceStatus } from './sidepanel/workspace-status.js';
import { createWorkspaceContextController } from './sidepanel/workspace-context.js';
import { requestWorkspaceAudit, requestWorkspaceNoteMode, requestWorkspaceScreenshot, toggleWorkspaceRecording } from './lib/workspace-actions.js';
// =============================================================================
// WRITE GUARD — defer queued writes while user is typing in the terminal
// =============================================================================
function resetWriteGuardState() {
    state.queuedWrites = [];
    state.terminalFocused = false;
    state.lastTypingAt = 0;
    state.queuedWriteInFlight = false;
    state.lastGuardToastAt = 0;
    if (state.queuedWriteFlushTimer !== null) {
        clearTimeout(state.queuedWriteFlushTimer);
        state.queuedWriteFlushTimer = null;
    }
    if (state.queuedSubmitTimer !== null) {
        clearTimeout(state.queuedSubmitTimer);
        state.queuedSubmitTimer = null;
    }
}
function shouldDeferQueuedWrite(nowMs = Date.now()) {
    if (!state.terminalFocused)
        return false;
    return nowMs - state.lastTypingAt < TERMINAL_TYPING_IDLE_MS;
}
function maybeShowQueuedWriteToast(nowMs = Date.now()) {
    if (nowMs - state.lastGuardToastAt < TERMINAL_GUARD_TOAST_INTERVAL_MS)
        return;
    state.lastGuardToastAt = nowMs;
    showActionToast('waiting for user to stop typing', 'Queued terminal action', 'warning', 1800);
}
function scheduleQueuedWriteFlush(delayMs = 0) {
    if (state.queuedWriteFlushTimer !== null)
        clearTimeout(state.queuedWriteFlushTimer);
    state.queuedWriteFlushTimer = setTimeout(() => {
        state.queuedWriteFlushTimer = null;
        flushQueuedWrites();
    }, delayMs);
}
function scheduleQueuedSubmit(delayMs) {
    if (state.queuedSubmitTimer !== null)
        clearTimeout(state.queuedSubmitTimer);
    state.queuedSubmitTimer = setTimeout(() => {
        state.queuedSubmitTimer = null;
        if (!state.visible || !state.iframeEl) {
            resetWriteGuardState();
            return;
        }
        if (!state.terminalConnected) {
            scheduleQueuedSubmit(TERMINAL_GUARD_POLL_MS);
            return;
        }
        if (shouldDeferQueuedWrite()) {
            maybeShowQueuedWriteToast();
            scheduleQueuedSubmit(TERMINAL_GUARD_POLL_MS);
            return;
        }
        notifyIframe('write', { text: '\r' });
        notifyIframe('focus');
        state.queuedWriteInFlight = false;
        if (state.queuedWrites.length > 0) {
            scheduleQueuedWriteFlush(0);
        }
    }, delayMs);
}
function flushQueuedWrites() {
    if (!state.visible || !state.iframeEl) {
        resetWriteGuardState();
        return;
    }
    if (!state.terminalConnected) {
        scheduleQueuedWriteFlush(TERMINAL_GUARD_POLL_MS);
        return;
    }
    if (state.queuedWriteInFlight)
        return;
    if (state.queuedWrites.length === 0) {
        state.lastGuardToastAt = 0;
        return;
    }
    if (shouldDeferQueuedWrite()) {
        maybeShowQueuedWriteToast();
        scheduleQueuedWriteFlush(TERMINAL_GUARD_POLL_MS);
        return;
    }
    const nextWrite = state.queuedWrites.shift();
    if (!nextWrite)
        return;
    state.lastGuardToastAt = 0;
    state.queuedWriteInFlight = true;
    notifyIframe('redraw');
    notifyIframe('write', { text: nextWrite });
    scheduleQueuedSubmit(TERMINAL_WRITE_SUBMIT_DELAY_MS);
}
// =============================================================================
// TERMINAL PANEL STATE
// =============================================================================
let rootEl = null;
let terminalShellEl = null;
let terminalBodyEl = null;
let statusDotEl = null;
let minimizeButtonEl = null;
let workspaceSummaryStripEl = null;
let workspaceStatusAreaEl = null;
let currentWorkspaceSnapshot = null;
let workspaceContextMessage = null;
let workspaceContextController = null;
let currentWorkspaceHostTabId;
let runtimeListenerInstalled = false;
let storageListenerInstalled = false;
let unloadListenerInstalled = false;
let panelReady = false;
let pendingSandboxError = null;
let panelCloseIntent = null;
let workspaceBootGeneration = 0;
let workspaceStatusRequestVersion = 0;
let workspaceActionRequestVersion = 0;
function getHostTabIdFromLocation() {
    try {
        const raw = new URLSearchParams(globalThis.location?.search ?? '').get('tabId');
        if (!raw)
            return undefined;
        const parsed = Number(raw);
        return Number.isFinite(parsed) ? parsed : undefined;
    }
    catch {
        return undefined;
    }
}
async function getHostTabId() {
    const fromLocation = getHostTabIdFromLocation();
    if (fromLocation !== undefined)
        return fromLocation;
    if (!chrome.tabs?.query)
        return undefined;
    try {
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        return tab?.id;
    }
    catch {
        return undefined;
    }
}
async function closeBrowserSidePanel() {
    if (!chrome.sidePanel?.close)
        return;
    const tabId = await getHostTabId();
    if (tabId === undefined)
        return;
    try {
        await chrome.sidePanel.close({ tabId });
    }
    catch {
        // Best effort.
    }
}
function setPanelVisible(visible) {
    state.visible = visible;
    if (!rootEl)
        return;
    rootEl.style.opacity = visible ? '1' : '0';
    rootEl.style.pointerEvents = visible ? 'auto' : 'none';
}
function setTerminalBodyVisible(visible) {
    if (!terminalBodyEl || !terminalShellEl || !minimizeButtonEl)
        return;
    terminalBodyEl.style.display = visible ? 'block' : 'none';
    terminalShellEl.style.height = visible ? '100%' : `${MINIMIZED_WIDGET_HEIGHT}px`;
    terminalShellEl.style.minHeight = visible ? '0' : `${MINIMIZED_WIDGET_HEIGHT}px`;
    terminalShellEl.style.flex = visible ? '1 1 auto' : `0 0 ${MINIMIZED_WIDGET_HEIGHT}px`;
    minimizeButtonEl.textContent = visible ? '\u2581' : '\u25A1';
    minimizeButtonEl.title = visible ? 'Minimize terminal' : 'Restore terminal';
}
function showSandboxError(message, instruction, command) {
    if (!terminalBodyEl)
        return;
    pendingSandboxError = { message, instruction, command };
    terminalBodyEl.replaceChildren();
    const overlay = document.createElement('div');
    Object.assign(overlay.style, {
        display: 'flex',
        flexDirection: 'column',
        gap: '10px',
        padding: '16px',
        borderRadius: '12px',
        background: '#1a1b26',
        border: '1px solid #f7768e',
        color: '#a9b1d6',
        margin: '16px'
    });
    const title = document.createElement('div');
    title.textContent = 'Terminal unavailable';
    Object.assign(title.style, {
        color: '#f7768e',
        fontWeight: '600',
        fontSize: '14px'
    });
    const msg = document.createElement('div');
    msg.textContent = message;
    Object.assign(msg.style, {
        fontSize: '12px',
        color: '#787c99'
    });
    const inst = document.createElement('div');
    inst.textContent = instruction;
    inst.style.fontSize = '12px';
    const cmdBox = document.createElement('div');
    Object.assign(cmdBox.style, {
        background: '#16161e',
        border: '1px solid #292e42',
        borderRadius: '8px',
        padding: '10px 12px',
        fontFamily: '"SF Mono", "Fira Code", Menlo, Monaco, monospace',
        fontSize: '12px',
        color: '#9ece6a'
    });
    cmdBox.textContent = command;
    overlay.appendChild(title);
    overlay.appendChild(msg);
    overlay.appendChild(inst);
    overlay.appendChild(cmdBox);
    terminalBodyEl.appendChild(overlay);
}
function updateStatusDot(dotState) {
    if (!statusDotEl)
        return;
    switch (dotState) {
        case 'connected':
            statusDotEl.style.background = '#9ece6a';
            break;
        case 'disconnected':
            statusDotEl.style.background = '#e0af68';
            break;
        case 'exited':
            statusDotEl.style.background = '#f7768e';
            break;
    }
}
function isWorkspaceStatusSnapshot(value) {
    if (typeof value !== 'object' || value === null)
        return false;
    return 'seo' in value && 'accessibility' in value && 'performance' in value && 'session' in value && 'page' in value;
}
function shouldApplyWorkspaceStatus(snapshot, requestVersion) {
    if (requestVersion !== workspaceStatusRequestVersion)
        return false;
    if (!currentWorkspaceSnapshot)
        return true;
    if (currentWorkspaceSnapshot.mode === 'audit' && snapshot.mode === 'live') {
        return currentWorkspaceSnapshot.page.url !== snapshot.page.url;
    }
    return true;
}
function applyWorkspaceStatus(snapshot, requestVersion = workspaceStatusRequestVersion) {
    if (!workspaceSummaryStripEl || !workspaceStatusAreaEl)
        return;
    if (!shouldApplyWorkspaceStatus(snapshot, requestVersion))
        return;
    currentWorkspaceSnapshot = snapshot;
    renderWorkspaceStatus(workspaceSummaryStripEl, workspaceStatusAreaEl, snapshot, workspaceContextMessage);
    workspaceContextController?.setSnapshot(snapshot);
}
function setWorkspaceContextUi(message) {
    workspaceContextMessage = message;
    if (!currentWorkspaceSnapshot || !workspaceSummaryStripEl || !workspaceStatusAreaEl)
        return;
    renderWorkspaceStatus(workspaceSummaryStripEl, workspaceStatusAreaEl, currentWorkspaceSnapshot, workspaceContextMessage);
}
function supersedeWorkspaceContextUi(message) {
    workspaceActionRequestVersion += 1;
    setWorkspaceContextUi(message);
}
function startWorkspaceAction(message) {
    const actionVersion = ++workspaceActionRequestVersion;
    setWorkspaceContextUi(message);
    return actionVersion;
}
function finishWorkspaceAction(actionVersion, message) {
    if (actionVersion !== workspaceActionRequestVersion)
        return;
    setWorkspaceContextUi(message);
}
function describeWorkspaceActionFailure(error) {
    if (typeof error === 'string' && error.trim())
        return error.trim();
    if (error instanceof Error && error.message.trim())
        return error.message.trim();
    if (typeof error === 'object' && error !== null) {
        const maybeError = error.error;
        if (typeof maybeError === 'string' && maybeError.trim())
            return maybeError.trim();
        const maybeMessage = error.message;
        if (typeof maybeMessage === 'string' && maybeMessage.trim())
            return maybeMessage.trim();
    }
    return 'Try again from the workspace.';
}
function ensureWorkspaceActionSucceeded(response) {
    if (typeof response !== 'object' || response === null)
        return;
    if ('success' in response && response.success === false) {
        throw new Error(describeWorkspaceActionFailure(response));
    }
    if ('error' in response && typeof response.error === 'string' && response.error.trim()) {
        throw new Error(describeWorkspaceActionFailure(response));
    }
}
function runWorkspaceAction(options) {
    const actionVersion = startWorkspaceAction(options.pendingMessage);
    void options.run()
        .then((response) => {
        ensureWorkspaceActionSucceeded(response);
        finishWorkspaceAction(actionVersion, options.successMessage);
    })
        .catch((error) => {
        finishWorkspaceAction(actionVersion, `${options.failurePrefix} ${describeWorkspaceActionFailure(error)}`);
    });
}
async function refreshWorkspaceStatus(mode = 'live') {
    const requestVersion = ++workspaceStatusRequestVersion;
    try {
        const tabId = await getHostTabId();
        const response = await chrome.runtime.sendMessage({
            type: 'get_workspace_status',
            mode,
            tab_id: tabId
        });
        if (isWorkspaceStatusSnapshot(response)) {
            applyWorkspaceStatus(response, requestVersion);
            if (!shouldApplyWorkspaceStatus(response, requestVersion))
                return undefined;
            return response;
        }
    }
    catch {
        // Best effort. The workspace shell still renders without live status.
    }
    return undefined;
}
function notifyIframe(command, data = {}) {
    if (!state.iframeEl?.contentWindow)
        return;
    state.iframeEl.contentWindow.postMessage({ command, ...data }, '*');
}
function handleIframeMessage(event) {
    if (!event.data || event.data.source !== 'kaboom-terminal')
        return;
    try {
        const termOrigin = getTerminalServerUrl(state.serverUrl);
        if (event.origin !== termOrigin)
            return;
    }
    catch {
        return;
    }
    switch (event.data.event) {
        case 'connected':
            updateStatusDot('connected');
            state.terminalConnected = true;
            if (state.queuedWrites.length > 0 && !state.queuedWriteInFlight) {
                scheduleQueuedWriteFlush(0);
            }
            break;
        case 'disconnected':
            updateStatusDot('disconnected');
            state.terminalConnected = false;
            state.terminalFocused = false;
            break;
        case 'exited':
            updateStatusDot('exited');
            state.terminalConnected = false;
            state.terminalFocused = false;
            resetWriteGuardState();
            break;
        case 'focus':
            state.terminalFocused = Boolean(event.data.data?.focused);
            if (state.terminalFocused) {
                state.lastTypingAt = Date.now();
            }
            else if (state.queuedWrites.length > 0 && !state.queuedWriteInFlight) {
                scheduleQueuedWriteFlush(0);
            }
            break;
        case 'typing': {
            const rawAt = event.data.data?.at;
            const parsedAt = typeof rawAt === 'number' && Number.isFinite(rawAt) ? rawAt : Date.now();
            state.terminalFocused = true;
            state.lastTypingAt = parsedAt;
            break;
        }
    }
}
function createPanelShell(token) {
    const terminalPane = createWorkspaceTerminalPane({
        token,
        serverUrl: state.serverUrl,
        onDisconnect: (e) => {
            e.preventDefault();
            e.stopPropagation();
            void exitTerminalSession();
        },
        onRedraw: (e) => {
            e.preventDefault();
            e.stopPropagation();
            redrawTerminal();
        },
        onMinimize: (e) => {
            e.preventDefault();
            e.stopPropagation();
            void minimizePanel();
        }
    });
    const workspaceShell = createWorkspaceShell(terminalPane.shellEl, {
        onToggleRecording: () => {
            const recordingActive = currentWorkspaceSnapshot?.session.recording_active === true;
            runWorkspaceAction({
                pendingMessage: recordingActive ? 'Stopping recording from the workspace...' : 'Starting recording from the workspace...',
                successMessage: recordingActive ? 'Recording stopped from the workspace.' : 'Recording started from the workspace.',
                failurePrefix: recordingActive ? 'Failed to stop recording.' : 'Failed to start recording.',
                run: () => toggleWorkspaceRecording(recordingActive)
            });
        },
        onScreenshot: () => {
            runWorkspaceAction({
                pendingMessage: 'Capturing screenshot from the workspace...',
                successMessage: 'Screenshot captured for the workspace session.',
                failurePrefix: 'Screenshot capture failed.',
                run: () => requestWorkspaceScreenshot()
            });
        },
        onRunAudit: () => {
            runWorkspaceAction({
                pendingMessage: 'Requesting audit in the workspace terminal...',
                successMessage: 'Audit requested in the workspace terminal. Waiting for results.',
                failurePrefix: 'Audit request failed.',
                run: () => requestWorkspaceAudit(currentWorkspaceSnapshot?.page.url)
            });
        },
        onAddNote: () => {
            runWorkspaceAction({
                pendingMessage: 'Starting note mode on the tracked page...',
                successMessage: 'Note mode started on the tracked page.',
                failurePrefix: 'Failed to start note mode.',
                run: async () => {
                    const tabId = await getHostTabId();
                    if (tabId === undefined) {
                        throw new Error('No tracked tab is available.');
                    }
                    return await requestWorkspaceNoteMode(tabId);
                }
            });
        },
        onInjectContext: () => {
            workspaceActionRequestVersion += 1;
            workspaceContextController?.injectCurrentContext();
        },
        onResetWorkspace: () => {
            workspaceActionRequestVersion += 1;
            workspaceContextController?.reset();
        }
    });
    terminalShellEl = terminalPane.shellEl;
    terminalBodyEl = terminalPane.bodyEl;
    statusDotEl = terminalPane.statusDotEl;
    minimizeButtonEl = terminalPane.minimizeButtonEl;
    workspaceSummaryStripEl = workspaceShell.summaryStripEl;
    workspaceStatusAreaEl = workspaceShell.statusAreaEl;
    state.iframeEl = terminalPane.iframeEl;
    state.widgetEl = workspaceShell.rootEl;
    return workspaceShell.rootEl;
}
function mountPanel(root) {
    if (rootEl)
        return;
    rootEl = root;
    const target = document.body || document.documentElement;
    if (!target)
        return;
    target.appendChild(rootEl);
    setPanelVisible(true);
    state.visible = true;
    window.addEventListener('message', handleIframeMessage);
}
function unmountPanel() {
    workspaceBootGeneration += 1;
    workspaceContextController?.dispose();
    workspaceContextController = null;
    if (rootEl) {
        rootEl.remove();
        rootEl = null;
    }
    terminalShellEl = null;
    terminalBodyEl = null;
    statusDotEl = null;
    minimizeButtonEl = null;
    workspaceSummaryStripEl = null;
    workspaceStatusAreaEl = null;
    currentWorkspaceHostTabId = undefined;
    currentWorkspaceSnapshot = null;
    workspaceContextMessage = null;
    state.widgetEl = null;
    state.iframeEl = null;
    panelReady = false;
    setPanelVisible(false);
    window.removeEventListener('message', handleIframeMessage);
}
function redrawTerminal() {
    if (!state.widgetEl || !state.iframeEl)
        return;
    const currentToken = state.sessionState?.token;
    if (!currentToken)
        return;
    const iframe = state.iframeEl;
    iframe.src = `${getTerminalServerUrl(state.serverUrl)}/terminal?token=${encodeURIComponent(currentToken)}`;
    setTerminalBodyVisible(true);
    state.minimized = false;
    persistUIState('open');
}
async function exitTerminalSession() {
    panelCloseIntent = 'clear';
    if (state.sessionState) {
        try {
            const termUrl = getTerminalServerUrl(state.serverUrl);
            await fetch(`${termUrl}/terminal/stop`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id: state.sessionState.sessionId }),
                signal: AbortSignal.timeout(3000)
            });
        }
        catch {
            // daemon unreachable or timeout — tear down locally
        }
    }
    clearPersistedSession();
    resetAllState();
    resetWriteGuardState();
    unmountPanel();
    await closeBrowserSidePanel();
}
async function minimizePanel() {
    panelCloseIntent = 'minimized';
    persistUIState('minimized');
    resetWriteGuardState();
    unmountPanel();
    await closeBrowserSidePanel();
}
function writeToTerminal(text) {
    if (!state.visible || !state.iframeEl)
        return;
    if (shouldDeferQueuedWrite()) {
        if (state.queuedWrites.length >= 200) {
            state.queuedWrites.shift();
        }
        state.queuedWrites.push(text);
        maybeShowQueuedWriteToast();
        scheduleQueuedWriteFlush(TERMINAL_GUARD_POLL_MS);
        return;
    }
    if (state.queuedWriteInFlight) {
        if (state.queuedWrites.length >= 200) {
            state.queuedWrites.shift();
        }
        state.queuedWrites.push(text);
        return;
    }
    state.queuedWriteInFlight = true;
    notifyIframe('redraw');
    notifyIframe('write', { text });
    scheduleQueuedSubmit(TERMINAL_WRITE_SUBMIT_DELAY_MS);
}
function installRuntimeListener() {
    if (runtimeListenerInstalled)
        return;
    runtimeListenerInstalled = true;
    chrome.runtime.onMessage.addListener((message, sender) => {
        if (sender.id !== chrome.runtime.id)
            return false;
        if (message.type === 'terminal_panel_write') {
            if (typeof message.text === 'string')
                writeToTerminal(message.text);
            return false;
        }
        if (message.type === 'workspace_status_updated' &&
            sender.tab === undefined &&
            isWorkspaceStatusSnapshot(message.snapshot)) {
            const hostTabId = message.host_tab_id;
            if (hostTabId !== undefined && hostTabId !== currentWorkspaceHostTabId) {
                return false;
            }
            const snapshot = message.snapshot;
            workspaceStatusRequestVersion += 1;
            applyWorkspaceStatus(snapshot);
            workspaceContextController?.handleAuditSnapshot(snapshot);
            return false;
        }
        return false;
    });
}
function installStorageListener() {
    if (storageListenerInstalled)
        return;
    storageListenerInstalled = true;
    onStorageChanged((changes, areaName) => {
        if (areaName !== 'session')
            return;
        const change = changes[StorageKey.TERMINAL_UI_STATE];
        if (!change)
            return;
        const uiState = change.newValue;
        if (uiState === 'closed') {
            state.visible = false;
            if (rootEl)
                rootEl.style.opacity = '0';
            return;
        }
        state.visible = true;
        if (rootEl)
            rootEl.style.opacity = '1';
    });
}
function installUnloadListener() {
    if (unloadListenerInstalled)
        return;
    unloadListenerInstalled = true;
    window.addEventListener('pagehide', () => {
        if (panelCloseIntent !== null)
            return;
        persistUIState('closed');
    });
}
async function ensureTerminalSession() {
    const persisted = await loadPersistedSession();
    if (persisted.session) {
        const alive = await validateSession(persisted.session.token);
        if (alive) {
            state.sessionState = persisted.session;
            state.minimized = false;
            return;
        }
        clearPersistedSession();
    }
    const config = await getTerminalConfig();
    const ss = await startSession(config, showSandboxError);
    if (!ss)
        return;
    state.sessionState = ss;
    state.minimized = false;
}
async function bootTerminalPanel(forceFresh = false) {
    const bootGeneration = ++workspaceBootGeneration;
    if (panelReady && !forceFresh)
        return;
    panelReady = true;
    panelCloseIntent = null;
    pendingSandboxError = null;
    state.serverUrl = await getServerUrl();
    installRuntimeListener();
    installStorageListener();
    installUnloadListener();
    if (forceFresh) {
        resetAllState();
        state.serverUrl = await getServerUrl();
    }
    await ensureTerminalSession();
    if (bootGeneration !== workspaceBootGeneration)
        return;
    const token = state.sessionState?.token;
    const root = createPanelShell(token ?? '');
    if (bootGeneration !== workspaceBootGeneration)
        return;
    mountPanel(root);
    const hostTabId = await getHostTabId();
    if (bootGeneration !== workspaceBootGeneration || root !== rootEl)
        return;
    currentWorkspaceHostTabId = hostTabId;
    workspaceContextController?.dispose();
    const contextController = createWorkspaceContextController({
        hostTabId,
        writeToTerminal,
        shouldDeferWrite: () => shouldDeferQueuedWrite(),
        onUiStateChange: ({ message }) => {
            supersedeWorkspaceContextUi(message);
        },
        refreshWorkspaceStatus
    });
    workspaceContextController = contextController;
    setTerminalBodyVisible(true);
    persistUIState('open');
    const liveSnapshot = await refreshWorkspaceStatus('live');
    if (bootGeneration !== workspaceBootGeneration || workspaceContextController !== contextController)
        return;
    contextController.handleWorkspaceOpen(liveSnapshot);
    if (!token) {
        const error = pendingSandboxError;
        if (error) {
            showSandboxError(error.message, error.instruction, error.command);
        }
        else if (terminalBodyEl) {
            terminalBodyEl.replaceChildren();
            const fallback = document.createElement('div');
            fallback.textContent = 'Terminal unavailable. Start the KaBOOM! daemon and reopen the panel.';
            Object.assign(fallback.style, {
                color: '#fca5a5',
                padding: '16px',
                fontSize: '12px'
            });
            terminalBodyEl.appendChild(fallback);
        }
    }
}
if (typeof document !== 'undefined' && typeof globalThis.process === 'undefined') {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            void bootTerminalPanel();
        });
    }
    else {
        void bootTerminalPanel();
    }
}
export const _terminalPanelForTests = {
    bootTerminalPanel,
    writeToTerminal,
    exitTerminalSession,
    redrawTerminal
};
//# sourceMappingURL=sidepanel.js.map
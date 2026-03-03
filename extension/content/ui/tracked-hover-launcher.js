/**
 * Purpose: Renders a tracked-tab hover launcher for fast annotate/record/screenshot actions.
 * Why: Reduces popup churn by exposing common capture actions directly on tracked pages.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
import { RuntimeMessageName, StorageKey } from '../../lib/constants.js';
const ROOT_ID = 'gasoline-tracked-hover-launcher';
const PANEL_ID = 'gasoline-tracked-hover-panel';
const TOGGLE_ID = 'gasoline-tracked-hover-toggle';
const SETTINGS_MENU_ID = 'gasoline-tracked-hover-settings-menu';
const STORAGE_AREA_LOCAL = 'local';
let rootEl = null;
let panelEl = null;
let settingsMenuEl = null;
let recordButtonEl = null;
let recordingActive = false;
let panelPinned = false;
let settingsMenuOpen = false;
let trackedEnabled = false;
let hiddenUntilPopupOpen = false;
let hideTimer = null;
let recordingStorageListener = null;
let runtimeListenerInstalled = false;
function clearHideTimer() {
    if (!hideTimer)
        return;
    clearTimeout(hideTimer);
    hideTimer = null;
}
function setPanelOpen(open) {
    if (!panelEl)
        return;
    panelEl.style.opacity = open ? '1' : '0';
    panelEl.style.transform = open ? 'translateX(0) scale(1)' : 'translateX(12px) scale(0.96)';
    panelEl.style.pointerEvents = open ? 'auto' : 'none';
}
function setSettingsMenuOpen(open) {
    settingsMenuOpen = open;
    if (!settingsMenuEl)
        return;
    settingsMenuEl.style.opacity = open ? '1' : '0';
    settingsMenuEl.style.transform = open ? 'translateY(0) scale(1)' : 'translateY(-8px) scale(0.96)';
    settingsMenuEl.style.pointerEvents = open ? 'auto' : 'none';
}
function updateRecordButtonState(active) {
    recordingActive = active;
    if (!recordButtonEl)
        return;
    recordButtonEl.textContent = active ? 'Stop' : 'Rec';
    recordButtonEl.title = active ? 'Stop recording' : 'Start recording';
    recordButtonEl.style.background = active ? '#c0392b' : '#f3f4f6';
    recordButtonEl.style.color = active ? '#fff' : '#1f2937';
    recordButtonEl.style.borderColor = active ? '#a93226' : '#d1d5db';
}
function readRecordingActive(value) {
    if (!value || typeof value !== 'object')
        return false;
    return Boolean(value.active);
}
function syncRecordingStateFromStorage() {
    chrome.storage.local.get([StorageKey.RECORDING], (result) => {
        updateRecordButtonState(readRecordingActive(result[StorageKey.RECORDING]));
    });
}
function syncHiddenStateFromStorage(onSynced) {
    chrome.storage.local.get([StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN], (result) => {
        hiddenUntilPopupOpen = Boolean(result[StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN]);
        onSynced();
    });
}
function persistHiddenState(hidden) {
    if (hidden) {
        chrome.storage.local.set({ [StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN]: true }, () => {
            void chrome.runtime.lastError;
        });
        return;
    }
    chrome.storage.local.remove(StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN, () => {
        void chrome.runtime.lastError;
    });
}
function installRecordingStorageSync() {
    if (recordingStorageListener)
        return;
    recordingStorageListener = (changes, areaName) => {
        if (areaName !== STORAGE_AREA_LOCAL)
            return;
        const recordingChange = changes[StorageKey.RECORDING];
        if (!recordingChange)
            return;
        updateRecordButtonState(readRecordingActive(recordingChange.newValue));
    };
    chrome.storage.onChanged.addListener(recordingStorageListener);
}
function uninstallRecordingStorageSync() {
    if (!recordingStorageListener)
        return;
    chrome.storage.onChanged.removeListener(recordingStorageListener);
    recordingStorageListener = null;
}
function hideLauncherUntilPopupReopen() {
    hiddenUntilPopupOpen = true;
    persistHiddenState(true);
    setSettingsMenuOpen(false);
    unmountLauncher();
}
function handleReshowRequest() {
    hiddenUntilPopupOpen = false;
    persistHiddenState(false);
    applyVisibilityFromState();
}
function installRuntimeListener() {
    if (runtimeListenerInstalled)
        return;
    runtimeListenerInstalled = true;
    chrome.runtime.onMessage.addListener((message, sender) => {
        if (sender.id !== chrome.runtime.id)
            return false;
        if (message.type !== RuntimeMessageName.SHOW_TRACKED_HOVER_LAUNCHER)
            return false;
        handleReshowRequest();
        return false;
    });
}
function applyVisibilityFromState() {
    if (trackedEnabled && !hiddenUntilPopupOpen) {
        mountLauncher();
        return;
    }
    unmountLauncher();
}
async function startDrawMode() {
    try {
        const drawModeModule = await import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'));
        if (typeof drawModeModule.activateDrawMode === 'function') {
            drawModeModule.activateDrawMode('user');
        }
    }
    catch {
        // Best-effort action; runtime listener provides canonical error handling.
    }
}
function runScreenshotCapture() {
    chrome.runtime.sendMessage({ type: 'captureScreenshot' }, () => {
        void chrome.runtime.lastError;
    });
}
function toggleRecordingAction() {
    const wasActive = recordingActive;
    const message = wasActive ? { type: 'record_stop' } : { type: 'record_start', audio: '' };
    const button = recordButtonEl;
    if (button)
        button.disabled = true;
    chrome.runtime.sendMessage(message, (response) => {
        if (button)
            button.disabled = false;
        if (chrome.runtime.lastError)
            return;
        const responseStatus = response?.status;
        if (wasActive) {
            if (responseStatus !== 'saved') {
                syncRecordingStateFromStorage();
                return;
            }
            updateRecordButtonState(false);
            return;
        }
        if (responseStatus === 'recording') {
            updateRecordButtonState(true);
            return;
        }
        syncRecordingStateFromStorage();
    });
}
function createActionButton(label, title, onClick) {
    const button = document.createElement('button');
    button.textContent = label;
    button.title = title;
    button.type = 'button';
    Object.assign(button.style, {
        height: '34px',
        minWidth: '54px',
        borderRadius: '10px',
        border: '1px solid #d1d5db',
        background: '#f3f4f6',
        color: '#1f2937',
        fontSize: '12px',
        fontWeight: '600',
        cursor: 'pointer',
        padding: '0 10px',
        transition: 'transform 140ms cubic-bezier(0.2, 0.8, 0.2, 1), box-shadow 160ms ease, background-color 160ms ease, border-color 160ms ease, color 160ms ease'
    });
    button.addEventListener('mouseenter', () => {
        button.style.transform = 'translateY(-1px)';
        button.style.boxShadow = '0 4px 12px rgba(15, 23, 42, 0.12)';
    });
    button.addEventListener('mouseleave', () => {
        button.style.transform = 'translateY(0)';
        button.style.boxShadow = 'none';
    });
    button.addEventListener('click', (event) => {
        event.preventDefault();
        event.stopPropagation();
        onClick();
    });
    return button;
}
function createSettingsMenuLink(label, href) {
    const link = document.createElement('a');
    link.textContent = label;
    link.href = href;
    link.target = '_blank';
    link.rel = 'noopener noreferrer';
    Object.assign(link.style, {
        display: 'block',
        color: '#111827',
        textDecoration: 'none',
        fontSize: '12px',
        fontWeight: '600',
        padding: '8px 10px',
        borderRadius: '8px',
        background: '#f9fafb',
        transition: 'transform 120ms ease, background-color 140ms ease'
    });
    link.addEventListener('mouseenter', () => {
        link.style.transform = 'translateX(1px)';
        link.style.background = '#f3f4f6';
    });
    link.addEventListener('mouseleave', () => {
        link.style.transform = 'translateX(0)';
        link.style.background = '#f9fafb';
    });
    link.addEventListener('click', () => {
        panelPinned = false;
        setPanelOpen(false);
        setSettingsMenuOpen(false);
    });
    return link;
}
function createLauncherUi() {
    const root = document.createElement('div');
    root.id = ROOT_ID;
    Object.assign(root.style, {
        position: 'fixed',
        top: '18px',
        right: '18px',
        zIndex: '2147483643',
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'
    });
    const panel = document.createElement('div');
    panel.id = PANEL_ID;
    Object.assign(panel.style, {
        display: 'flex',
        alignItems: 'center',
        gap: '6px',
        padding: '7px',
        borderRadius: '18px',
        background: '#ffffff',
        border: '1px solid rgba(15, 23, 42, 0.12)',
        boxShadow: '0 8px 24px rgba(15, 23, 42, 0.2)',
        opacity: '0',
        transform: 'translateX(12px) scale(0.96)',
        transformOrigin: 'right center',
        transition: 'opacity 220ms cubic-bezier(0.16, 1, 0.3, 1), transform 220ms cubic-bezier(0.16, 1, 0.3, 1)',
        pointerEvents: 'none',
        backdropFilter: 'saturate(160%) blur(6px)',
        willChange: 'opacity, transform'
    });
    const drawButton = createActionButton('Draw', 'Start annotation draw mode', () => {
        panelPinned = false;
        setPanelOpen(false);
        void startDrawMode();
    });
    const recordButton = createActionButton('Rec', 'Start recording', () => {
        panelPinned = true;
        toggleRecordingAction();
    });
    recordButtonEl = recordButton;
    const screenshotButton = createActionButton('Shot', 'Capture screenshot', () => {
        panelPinned = false;
        setPanelOpen(false);
        runScreenshotCapture();
    });
    const settingsButton = createActionButton('⚙', 'Launcher settings', () => {
        panelPinned = true;
        setSettingsMenuOpen(!settingsMenuOpen);
    });
    settingsButton.style.minWidth = '38px';
    settingsButton.style.padding = '0';
    panel.appendChild(drawButton);
    panel.appendChild(recordButton);
    panel.appendChild(screenshotButton);
    panel.appendChild(settingsButton);
    const settingsMenu = document.createElement('div');
    settingsMenu.id = SETTINGS_MENU_ID;
    Object.assign(settingsMenu.style, {
        position: 'absolute',
        top: '52px',
        right: '0',
        minWidth: '220px',
        display: 'flex',
        flexDirection: 'column',
        gap: '6px',
        padding: '10px',
        borderRadius: '12px',
        background: '#ffffff',
        border: '1px solid rgba(15, 23, 42, 0.12)',
        boxShadow: '0 10px 30px rgba(15, 23, 42, 0.18)',
        opacity: '0',
        transform: 'translateY(-8px) scale(0.96)',
        transformOrigin: 'top right',
        transition: 'opacity 180ms cubic-bezier(0.2, 0.8, 0.2, 1), transform 180ms cubic-bezier(0.2, 0.8, 0.2, 1)',
        pointerEvents: 'none',
        willChange: 'opacity, transform'
    });
    const docsLink = createSettingsMenuLink('Docs', 'https://cookwithgasoline.com/docs');
    const repoLink = createSettingsMenuLink('GitHub Repository', 'https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp');
    const hideButton = createActionButton('Hide Gasoline Devtool', 'Hide launcher until popup is opened again', () => {
        hideLauncherUntilPopupReopen();
    });
    hideButton.style.width = '100%';
    hideButton.style.justifyContent = 'center';
    settingsMenu.appendChild(docsLink);
    settingsMenu.appendChild(repoLink);
    settingsMenu.appendChild(hideButton);
    const toggle = document.createElement('button');
    toggle.id = TOGGLE_ID;
    toggle.type = 'button';
    toggle.textContent = 'G';
    toggle.title = 'Gasoline quick actions';
    Object.assign(toggle.style, {
        width: '44px',
        height: '44px',
        borderRadius: '22px',
        border: '2px solid #2563eb',
        background: '#ffffff',
        color: '#1d4ed8',
        fontSize: '16px',
        fontWeight: '700',
        cursor: 'pointer',
        boxShadow: '0 8px 24px rgba(15, 23, 42, 0.25)',
        transition: 'transform 180ms cubic-bezier(0.2, 0.8, 0.2, 1), box-shadow 180ms ease'
    });
    toggle.addEventListener('mouseenter', () => {
        toggle.style.transform = 'translateY(-1px)';
        toggle.style.boxShadow = '0 10px 26px rgba(15, 23, 42, 0.28)';
    });
    toggle.addEventListener('mouseleave', () => {
        toggle.style.transform = 'translateY(0)';
        toggle.style.boxShadow = '0 8px 24px rgba(15, 23, 42, 0.25)';
    });
    toggle.addEventListener('click', (event) => {
        event.preventDefault();
        event.stopPropagation();
        panelPinned = !panelPinned;
        clearHideTimer();
        setPanelOpen(panelPinned);
        if (!panelPinned)
            setSettingsMenuOpen(false);
    });
    root.addEventListener('mouseenter', () => {
        clearHideTimer();
        setPanelOpen(true);
    });
    root.addEventListener('mouseleave', () => {
        if (panelPinned || settingsMenuOpen)
            return;
        clearHideTimer();
        hideTimer = setTimeout(() => {
            setPanelOpen(false);
            setSettingsMenuOpen(false);
        }, 120);
    });
    root.appendChild(panel);
    root.appendChild(toggle);
    root.appendChild(settingsMenu);
    panelEl = panel;
    settingsMenuEl = settingsMenu;
    syncRecordingStateFromStorage();
    return root;
}
function mountLauncher() {
    if (hiddenUntilPopupOpen)
        return;
    if (rootEl || document.getElementById(ROOT_ID))
        return;
    rootEl = createLauncherUi();
    const target = document.body || document.documentElement;
    if (!target || !rootEl)
        return;
    target.appendChild(rootEl);
    installRecordingStorageSync();
}
function unmountLauncher() {
    clearHideTimer();
    panelPinned = false;
    setSettingsMenuOpen(false);
    panelEl = null;
    settingsMenuEl = null;
    recordButtonEl = null;
    if (rootEl) {
        rootEl.remove();
        rootEl = null;
    }
    uninstallRecordingStorageSync();
}
export function setTrackedHoverLauncherEnabled(enabled) {
    trackedEnabled = enabled;
    installRuntimeListener();
    syncHiddenStateFromStorage(applyVisibilityFromState);
}
//# sourceMappingURL=tracked-hover-launcher.js.map
/**
 * Purpose: Popup "Update now" button — wires self-update endpoint + reload-extension prompt.
 * Why: Lets users one-click upgrade the daemon from inside the extension.
 * Docs: docs/features/feature/self-update/index.md
 */
import { DEFAULT_SERVER_URL, StorageKey } from '../lib/constants.js';
import { buildDaemonHeaders, postDaemonJSON } from '../lib/daemon-http.js';
import { getLocal } from '../lib/storage-utils.js';
// Poll /health every 2s for up to 30s after kicking off the install. The
// daemon SIGTERMs itself + the supervisor respawns, so a short window is
// enough to see the version change.
const VERSION_POLL_INTERVAL_MS = 2000;
const VERSION_POLL_TIMEOUT_MS = 30000;
async function getServerUrl() {
    const value = await getLocal(StorageKey.SERVER_URL);
    return value || DEFAULT_SERVER_URL;
}
async function fetchHealth(serverUrl) {
    const resp = await fetch(`${serverUrl}/health`, {
        headers: buildDaemonHeaders()
    });
    if (!resp.ok) {
        throw new Error(`health HTTP ${resp.status}`);
    }
    return (await resp.json());
}
async function fetchNonce(serverUrl) {
    const resp = await fetch(`${serverUrl}/upgrade/nonce`, {
        headers: buildDaemonHeaders()
    });
    if (!resp.ok) {
        throw new Error(`nonce HTTP ${resp.status}`);
    }
    const body = (await resp.json());
    if (!body.nonce) {
        throw new Error('nonce missing from response');
    }
    return body.nonce;
}
async function postInstall(serverUrl, nonce) {
    const resp = await postDaemonJSON(`${serverUrl}/upgrade/install`, { nonce });
    if (resp.status === 501) {
        throw new Error('Self-update is not supported on this platform — re-run the installer manually.');
    }
    if (resp.status === 429) {
        throw new Error('Update was requested recently. Wait a minute and try again.');
    }
    if (!resp.ok) {
        throw new Error(`install HTTP ${resp.status}`);
    }
}
// Poll /health until the daemon reports a version equal to `target`. Returns
// the observed version on success, or null on timeout.
async function waitForDaemonVersion(serverUrl, target) {
    const deadline = Date.now() + VERSION_POLL_TIMEOUT_MS;
    while (Date.now() < deadline) {
        await new Promise((resolve) => setTimeout(resolve, VERSION_POLL_INTERVAL_MS));
        try {
            const health = await fetchHealth(serverUrl);
            if (health.version && health.version === target) {
                return health.version;
            }
        }
        catch {
            // Daemon is restarting — expected during the upgrade window.
        }
    }
    return null;
}
function openExtensionsPage() {
    const id = chrome?.runtime?.id;
    const url = id ? `chrome://extensions/?id=${id}` : 'chrome://extensions';
    chrome.tabs.create({ url });
}
function showState(mode, errorMessage) {
    const idle = document.getElementById('update-action-idle');
    const running = document.getElementById('update-action-running');
    const reload = document.getElementById('update-action-reload');
    const errorEl = document.getElementById('update-action-error');
    if (idle)
        idle.style.display = mode === 'idle' ? '' : 'none';
    if (running)
        running.style.display = mode === 'running' ? '' : 'none';
    if (reload)
        reload.style.display = mode === 'reload' ? '' : 'none';
    if (errorEl) {
        errorEl.style.display = mode === 'error' ? '' : 'none';
        errorEl.textContent = mode === 'error' && errorMessage ? errorMessage : '';
    }
}
async function runUpgradeFlow(info) {
    showState('running');
    try {
        const nonce = await fetchNonce(info.serverUrl);
        await postInstall(info.serverUrl, nonce);
        const observed = await waitForDaemonVersion(info.serverUrl, info.availableVersion);
        if (observed) {
            showState('reload');
        }
        else {
            showState('error', 'Daemon did not restart in time. Check the terminal or rerun the installer manually.');
        }
    }
    catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        showState('error', msg);
    }
}
/**
 * Render the update-available banner based on latest health. No-op if no
 * upgrade is offered by the daemon.
 */
export async function renderUpdateAvailableBanner(health) {
    const container = document.getElementById('update-available');
    const detail = document.getElementById('update-available-detail');
    if (!container)
        return;
    const current = health.version ?? '';
    const next = health.available_version ?? '';
    const isNewer = next && current && next !== current;
    if (!isNewer) {
        container.style.display = 'none';
        return;
    }
    const serverUrl = await getServerUrl();
    if (detail) {
        detail.textContent = `v${current} installed; v${next} available.`;
    }
    container.style.display = '';
    showState('idle');
    const btn = document.getElementById('update-now-btn');
    if (btn && !btn.dataset.wired) {
        btn.dataset.wired = '1';
        btn.addEventListener('click', () => {
            void runUpgradeFlow({ currentVersion: current, availableVersion: next, serverUrl });
        });
    }
    const reloadBtn = document.getElementById('update-reload-ext-btn');
    if (reloadBtn && !reloadBtn.dataset.wired) {
        reloadBtn.dataset.wired = '1';
        reloadBtn.addEventListener('click', openExtensionsPage);
    }
}
//# sourceMappingURL=update-button.js.map
/**
 * Purpose: Terminal session lifecycle — config persistence, session start/validate/persist.
 * Why: Isolates all daemon HTTP calls and chrome.storage I/O from UI and orchestrator logic.
 * Docs: docs/features/feature/terminal/index.md
 */
import { DEFAULT_SERVER_URL, StorageKey } from '../../lib/constants.js';
import { getLocalValue, setSessionValue, getSessionValue, removeSessions, setLocal } from '../../lib/storage-utils.js';
import { state, getTerminalServerUrl } from './terminal-widget-types.js';
import { showSandboxError } from './terminal-widget-ui.js';
// =============================================================================
// CONFIG HELPERS — read/write chrome.storage.local
// =============================================================================
export function getServerUrl() {
    return new Promise((resolve) => {
        try {
            getLocalValue(StorageKey.SERVER_URL, (value) => {
                const url = value || DEFAULT_SERVER_URL;
                state.serverUrl = url;
                resolve(url);
            });
        }
        catch {
            resolve(DEFAULT_SERVER_URL); // Extension context invalidated
        }
    });
}
export function getTerminalConfig() {
    return new Promise((resolve) => {
        try {
            getLocalValue(StorageKey.TERMINAL_CONFIG, (value) => {
                const config = value || {};
                resolve(config);
            });
        }
        catch {
            resolve({}); // Extension context invalidated
        }
    });
}
export function saveTerminalConfig(config) {
    try {
        void setLocal(StorageKey.TERMINAL_CONFIG, config);
    }
    catch {
        // Extension context invalidated — config won't persist but session still works
    }
}
function getTerminalAICommand() {
    return new Promise((resolve) => {
        try {
            getLocalValue(StorageKey.TERMINAL_AI_COMMAND, (value) => {
                const cmd = value || 'claude';
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
            getLocalValue(StorageKey.TERMINAL_DEV_ROOT, (value) => {
                resolve(value || '');
            });
        }
        catch {
            resolve('');
        }
    });
}
// =============================================================================
// SESSION PERSISTENCE — survives page refresh via chrome.storage.session
// =============================================================================
export function persistSession(ss) {
    try {
        setSessionValue(StorageKey.TERMINAL_SESSION, ss);
    }
    catch { /* extension context invalidated */ }
}
export function clearPersistedSession() {
    try {
        void removeSessions([StorageKey.TERMINAL_SESSION, StorageKey.TERMINAL_UI_STATE]);
    }
    catch { /* extension context invalidated */ }
}
export function persistUIState(uiState) {
    try {
        setSessionValue(StorageKey.TERMINAL_UI_STATE, uiState);
    }
    catch { /* extension context invalidated */ }
}
export function loadPersistedSession() {
    return new Promise((resolve) => {
        try {
            getSessionValue(StorageKey.TERMINAL_SESSION, (sessionValue) => {
                getSessionValue(StorageKey.TERMINAL_UI_STATE, (uiValue) => {
                    const session = sessionValue;
                    const uiState = uiValue || 'closed';
                    resolve({ session: session || null, uiState });
                });
            });
        }
        catch {
            resolve({ session: null, uiState: 'closed' });
        }
    });
}
// =============================================================================
// SESSION LIFECYCLE — start, validate
// =============================================================================
/** Validate that a persisted token is still alive on the daemon. */
export async function validateSession(token) {
    try {
        const base = await getServerUrl();
        const termUrl = getTerminalServerUrl(base);
        const resp = await fetch(`${termUrl}/terminal/validate?token=${encodeURIComponent(token)}`, { signal: AbortSignal.timeout(2000) });
        if (!resp.ok)
            return false;
        const data = await resp.json();
        return data.valid === true;
    }
    catch {
        return false;
    }
}
export async function startSession(config) {
    const base = await getServerUrl();
    const termUrl = getTerminalServerUrl(base);
    const aiCommand = await getTerminalAICommand();
    const devRoot = await getTerminalDevRoot();
    try {
        // Build init_command: unset CLAUDECODE to avoid nesting detection, then launch the AI tool.
        const initCommand = aiCommand ? `unset CLAUDECODE 2>/dev/null; ${aiCommand}` : '';
        const resp = await fetch(`${termUrl}/terminal/start`, {
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
                const ss = { sessionId: body.session_id ?? 'default', token: body.token };
                persistSession(ss);
                return ss;
            }
            console.warn('[Gasoline] Terminal session rejected (HTTP ' + resp.status + '): ' +
                (body.error ?? 'unknown') + '. Check the daemon logs for details.');
            return null;
        }
        const data = await resp.json();
        const ss = { sessionId: data.session_id, token: data.token };
        persistSession(ss);
        return ss;
    }
    catch (err) {
        console.warn('[Gasoline] Terminal session start failed: ' +
            (err instanceof Error ? err.message : String(err)) +
            '. Is the Gasoline daemon running? Start it with: npx gasoline-agentic-browser');
        return null;
    }
}
//# sourceMappingURL=terminal-widget-session.js.map
/**
 * Purpose: Terminal session lifecycle — config persistence, session start/validate/persist.
 * Why: Isolates all daemon HTTP calls and chrome.storage I/O from UI and orchestrator logic.
 * Docs: docs/features/feature/terminal/index.md
 */

import { DEFAULT_SERVER_URL, StorageKey } from '../../lib/constants.js'
import { getLocal, setSession, getSession, removeSessions, setLocal } from '../../lib/storage-utils.js'
import {
  state,
  getTerminalServerUrl,
  type TerminalConfig,
  type TerminalSessionState,
  type TerminalUIState
} from './terminal-widget-types.js'
import { showSandboxError } from './terminal-widget-ui.js'

// =============================================================================
// CONFIG HELPERS — read/write chrome.storage.local
// =============================================================================

export async function getServerUrl(): Promise<string> {
  try {
    const value = await getLocal(StorageKey.SERVER_URL)
    const url = (value as string) || DEFAULT_SERVER_URL
    state.serverUrl = url
    return url
  } catch {
    return DEFAULT_SERVER_URL // Extension context invalidated
  }
}

export async function getTerminalConfig(): Promise<TerminalConfig> {
  try {
    const value = await getLocal(StorageKey.TERMINAL_CONFIG)
    const config = (value as TerminalConfig) || {}
    return config
  } catch {
    return {} // Extension context invalidated
  }
}

export function saveTerminalConfig(config: TerminalConfig): void {
  try {
    void setLocal(StorageKey.TERMINAL_CONFIG, config)
  } catch {
    // Extension context invalidated — config won't persist but session still works
  }
}

async function getTerminalAICommand(): Promise<string> {
  try {
    const value = await getLocal(StorageKey.TERMINAL_AI_COMMAND)
    const cmd = (value as string) || 'claude'
    return cmd
  } catch {
    return 'claude'
  }
}

async function getTerminalDevRoot(): Promise<string> {
  try {
    const value = await getLocal(StorageKey.TERMINAL_DEV_ROOT)
    return (value as string) || ''
  } catch {
    return ''
  }
}

// =============================================================================
// SESSION PERSISTENCE — survives page refresh via chrome.storage.session
// =============================================================================

export function persistSession(ss: TerminalSessionState): void {
  try {
    void setSession(StorageKey.TERMINAL_SESSION, ss)
  } catch { /* extension context invalidated */ }
}

export function clearPersistedSession(): void {
  try {
    void removeSessions([StorageKey.TERMINAL_SESSION, StorageKey.TERMINAL_UI_STATE])
  } catch { /* extension context invalidated */ }
}

export function persistUIState(uiState: TerminalUIState): void {
  try {
    void setSession(StorageKey.TERMINAL_UI_STATE, uiState)
  } catch { /* extension context invalidated */ }
}

export async function loadPersistedSession(): Promise<{ session: TerminalSessionState | null; uiState: TerminalUIState }> {
  try {
    const sessionValue = await getSession(StorageKey.TERMINAL_SESSION)
    const uiValue = await getSession(StorageKey.TERMINAL_UI_STATE)
    const session = sessionValue as TerminalSessionState | undefined
    const uiState = (uiValue as TerminalUIState) || 'closed'
    return { session: session || null, uiState }
  } catch {
    return { session: null, uiState: 'closed' }
  }
}

// =============================================================================
// SESSION LIFECYCLE — start, validate
// =============================================================================

/** Validate that a persisted token is still alive on the daemon. */
export async function validateSession(token: string): Promise<boolean> {
  try {
    const base = await getServerUrl()
    const termUrl = getTerminalServerUrl(base)
    const resp = await fetch(
      `${termUrl}/terminal/validate?token=${encodeURIComponent(token)}`,
      { signal: AbortSignal.timeout(2000) }
    )
    if (!resp.ok) return false
    const data = await resp.json() as { valid?: boolean }
    return data.valid === true
  } catch {
    return false
  }
}

export async function startSession(config: TerminalConfig): Promise<TerminalSessionState | null> {
  const base = await getServerUrl()
  const termUrl = getTerminalServerUrl(base)
  const aiCommand = await getTerminalAICommand()
  const devRoot = await getTerminalDevRoot()
  try {
    // Build init_command: unset CLAUDECODE to avoid nesting detection, then launch the AI tool.
    const initCommand = aiCommand ? `unset CLAUDECODE 2>/dev/null; ${aiCommand}` : ''
    const resp = await fetch(`${termUrl}/terminal/start`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        cmd: config.cmd || '',
        args: config.args || [],
        dir: config.dir || devRoot || '',
        init_command: initCommand
      })
    })
    if (!resp.ok) {
      const body = await resp.json() as {
        error?: string; message?: string; instruction?: string; command?: string
        session_id?: string; token?: string
      }
      // Sandbox restriction — show actionable instructions to the user.
      if (resp.status === 503 && body.error === 'sandbox_restricted') {
        showSandboxError(body.message ?? '', body.instruction ?? '', body.command ?? '')
        return null
      }
      // Session already exists — reconnect using the returned token.
      if (resp.status === 409 && body.token) {
        const ss = { sessionId: body.session_id ?? 'default', token: body.token }
        persistSession(ss)
        return ss
      }
      console.warn('[Gasoline] Terminal session rejected (HTTP ' + resp.status + '): ' +
        (body.error ?? 'unknown') + '. Check the daemon logs for details.')
      return null
    }
    const data = await resp.json() as { session_id: string; token: string; pid: number }
    const ss = { sessionId: data.session_id, token: data.token }
    persistSession(ss)
    return ss
  } catch (err) {
    console.warn('[Gasoline] Terminal session start failed: ' +
      (err instanceof Error ? err.message : String(err)) +
      '. Is the Gasoline daemon running? Start it with: npx gasoline-agentic-browser')
    return null
  }
}

/**
 * Purpose: Terminal session lifecycle — config persistence, session start/validate/persist.
 * Why: Isolates all daemon HTTP calls and chrome.storage I/O from UI and orchestrator logic.
 * Docs: docs/features/feature/terminal/index.md
 */

import { DEFAULT_SERVER_URL, StorageKey } from '../../lib/constants.js'
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

export function getServerUrl(): Promise<string> {
  return new Promise((resolve) => {
    try {
      chrome.storage.local.get([StorageKey.SERVER_URL], (result: Record<string, unknown>) => {
        if (chrome.runtime.lastError) {
          resolve(DEFAULT_SERVER_URL) // Storage read failed — fall back to default
          return
        }
        const url = (result[StorageKey.SERVER_URL] as string) || DEFAULT_SERVER_URL
        state.serverUrl = url
        resolve(url)
      })
    } catch {
      resolve(DEFAULT_SERVER_URL) // Extension context invalidated
    }
  })
}

export function getTerminalConfig(): Promise<TerminalConfig> {
  return new Promise((resolve) => {
    try {
      chrome.storage.local.get([StorageKey.TERMINAL_CONFIG], (result: Record<string, unknown>) => {
        if (chrome.runtime.lastError) {
          resolve({}) // Storage read failed — use defaults
          return
        }
        const config = (result[StorageKey.TERMINAL_CONFIG] as TerminalConfig) || {}
        resolve(config)
      })
    } catch {
      resolve({}) // Extension context invalidated
    }
  })
}

export function saveTerminalConfig(config: TerminalConfig): void {
  try {
    chrome.storage.local.set({ [StorageKey.TERMINAL_CONFIG]: config }, () => {
      void chrome.runtime.lastError // Best-effort persistence
    })
  } catch {
    // Extension context invalidated — config won't persist but session still works
  }
}

function getTerminalAICommand(): Promise<string> {
  return new Promise((resolve) => {
    try {
      chrome.storage.local.get([StorageKey.TERMINAL_AI_COMMAND], (result: Record<string, unknown>) => {
        if (chrome.runtime.lastError) {
          resolve('claude')
          return
        }
        const cmd = (result[StorageKey.TERMINAL_AI_COMMAND] as string) || 'claude'
        resolve(cmd)
      })
    } catch {
      resolve('claude')
    }
  })
}

function getTerminalDevRoot(): Promise<string> {
  return new Promise((resolve) => {
    try {
      chrome.storage.local.get([StorageKey.TERMINAL_DEV_ROOT], (result: Record<string, unknown>) => {
        if (chrome.runtime.lastError) {
          resolve('')
          return
        }
        resolve((result[StorageKey.TERMINAL_DEV_ROOT] as string) || '')
      })
    } catch {
      resolve('')
    }
  })
}

// =============================================================================
// SESSION PERSISTENCE — survives page refresh via chrome.storage.session
// =============================================================================

export function persistSession(ss: TerminalSessionState): void {
  try {
    chrome.storage.session.set({ [StorageKey.TERMINAL_SESSION]: ss }, () => {
      void chrome.runtime.lastError
    })
  } catch { /* extension context invalidated */ }
}

export function clearPersistedSession(): void {
  try {
    chrome.storage.session.remove([StorageKey.TERMINAL_SESSION, StorageKey.TERMINAL_UI_STATE], () => {
      void chrome.runtime.lastError
    })
  } catch { /* extension context invalidated */ }
}

export function persistUIState(uiState: TerminalUIState): void {
  try {
    chrome.storage.session.set({ [StorageKey.TERMINAL_UI_STATE]: uiState }, () => {
      void chrome.runtime.lastError
    })
  } catch { /* extension context invalidated */ }
}

export function loadPersistedSession(): Promise<{ session: TerminalSessionState | null; uiState: TerminalUIState }> {
  return new Promise((resolve) => {
    try {
      chrome.storage.session.get(
        [StorageKey.TERMINAL_SESSION, StorageKey.TERMINAL_UI_STATE],
        (result: Record<string, unknown>) => {
          if (chrome.runtime.lastError) {
            resolve({ session: null, uiState: 'closed' })
            return
          }
          const session = result[StorageKey.TERMINAL_SESSION] as TerminalSessionState | undefined
          const uiState = (result[StorageKey.TERMINAL_UI_STATE] as TerminalUIState) || 'closed'
          resolve({ session: session || null, uiState })
        }
      )
    } catch {
      resolve({ session: null, uiState: 'closed' })
    }
  })
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

/**
 * Purpose: In-browser terminal widget that embeds a PTY-backed terminal via iframe.
 * Why: Provides a Lovable-like experience — chat with any CLI (claude, codex, aider) from
 * a browser overlay while seeing code edits reflected via hot reload on the tracked page.
 * Docs: docs/features/feature/terminal/index.md
 */

import { DEFAULT_SERVER_URL, TERMINAL_PORT_OFFSET, StorageKey } from '../../lib/constants.js'
import { showActionToast } from './toast.js'

const WIDGET_ID = 'gasoline-terminal-widget'
const IFRAME_ID = 'gasoline-terminal-iframe'
const HEADER_ID = 'gasoline-terminal-header'
const DISCONNECT_TERMINAL_BUTTON_ID = 'gasoline-terminal-disconnect-button'
const REDRAW_TERMINAL_BUTTON_ID = 'gasoline-terminal-redraw-button'
const MINIMIZE_TERMINAL_BUTTON_ID = 'gasoline-terminal-minimize-button'
const CLOSE_TERMINAL_BUTTON_ID = 'gasoline-terminal-close-button'
const DEFAULT_WIDGET_WIDTH = '50vw'
const DEFAULT_WIDGET_HEIGHT = '40vh'
const MIN_WIDGET_WIDTH = '400px'
const MIN_WIDGET_HEIGHT = '250px'
const MAX_WIDGET_WIDTH = '100vw'
const MAX_WIDGET_HEIGHT = '80vh'
const MINIMIZED_WIDGET_HEIGHT = '32px'
const TERMINAL_WRITE_SUBMIT_DELAY_MS = 600
const TERMINAL_TYPING_IDLE_MS = 1500
const TERMINAL_GUARD_POLL_MS = 200
const TERMINAL_GUARD_TOAST_INTERVAL_MS = 3000

interface TerminalConfig {
  cmd?: string
  args?: string[]
  dir?: string
  serverUrl?: string
}

interface TerminalSessionState {
  token: string
  sessionId: string
}

let widgetEl: HTMLDivElement | null = null
let iframeEl: HTMLIFrameElement | null = null
let resizeHandleEl: HTMLDivElement | null = null
let sessionState: TerminalSessionState | null = null
let visible = false
let minimized = false
let savedHeight = ''
let serverUrl = DEFAULT_SERVER_URL
let terminalFocused = false
let lastTypingAt = 0
let queuedWrites: string[] = []
let queuedWriteFlushTimer: ReturnType<typeof setTimeout> | null = null
let queuedSubmitTimer: ReturnType<typeof setTimeout> | null = null
let queuedWriteInFlight = false
let lastGuardToastAt = 0
let terminalConnected = false

/** Compute the terminal server URL from a base daemon URL (port + TERMINAL_PORT_OFFSET). */
function getTerminalServerUrl(baseUrl: string): string {
  const url = new URL(baseUrl)
  url.port = String(parseInt(url.port || '7890', 10) + TERMINAL_PORT_OFFSET)
  return url.origin
}

function getServerUrl(): Promise<string> {
  return new Promise((resolve) => {
    try {
      chrome.storage.local.get([StorageKey.SERVER_URL], (result: Record<string, unknown>) => {
        if (chrome.runtime.lastError) {
          resolve(DEFAULT_SERVER_URL) // Storage read failed — fall back to default
          return
        }
        const url = (result[StorageKey.SERVER_URL] as string) || DEFAULT_SERVER_URL
        serverUrl = url
        resolve(url)
      })
    } catch {
      resolve(DEFAULT_SERVER_URL) // Extension context invalidated
    }
  })
}

function getTerminalConfig(): Promise<TerminalConfig> {
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

function saveTerminalConfig(config: TerminalConfig): void {
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

type TerminalUIState = 'open' | 'closed' | 'minimized'

function persistSession(state: TerminalSessionState): void {
  try {
    chrome.storage.session.set({ [StorageKey.TERMINAL_SESSION]: state }, () => {
      void chrome.runtime.lastError
    })
  } catch { /* extension context invalidated */ }
}

function clearPersistedSession(): void {
  try {
    chrome.storage.session.remove([StorageKey.TERMINAL_SESSION, StorageKey.TERMINAL_UI_STATE], () => {
      void chrome.runtime.lastError
    })
  } catch { /* extension context invalidated */ }
}

function persistUIState(uiState: TerminalUIState): void {
  try {
    chrome.storage.session.set({ [StorageKey.TERMINAL_UI_STATE]: uiState }, () => {
      void chrome.runtime.lastError
    })
  } catch { /* extension context invalidated */ }
}

function loadPersistedSession(): Promise<{ session: TerminalSessionState | null; uiState: TerminalUIState }> {
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

/** Validate that a persisted token is still alive on the daemon. */
async function validateSession(token: string): Promise<boolean> {
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

async function startSession(config: TerminalConfig): Promise<TerminalSessionState | null> {
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
        const state = { sessionId: body.session_id ?? 'default', token: body.token }
        persistSession(state)
        return state
      }
      console.warn('[Gasoline] Terminal session rejected (HTTP ' + resp.status + '): ' +
        (body.error ?? 'unknown') + '. Check the daemon logs for details.')
      return null
    }
    const data = await resp.json() as { session_id: string; token: string; pid: number }
    const state = { sessionId: data.session_id, token: data.token }
    persistSession(state)
    return state
  } catch (err) {
    console.warn('[Gasoline] Terminal session start failed: ' +
      (err instanceof Error ? err.message : String(err)) +
      '. Is the Gasoline daemon running? Start it with: npx gasoline-agentic-browser')
    return null
  }
}

function showSandboxError(message: string, instruction: string, command: string): void {
  // Remove any existing widget/error overlay
  const existing = document.getElementById(WIDGET_ID)
  if (existing) existing.remove()

  const overlay = document.createElement('div')
  overlay.id = WIDGET_ID
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
  })

  const title = document.createElement('div')
  title.textContent = 'Terminal Unavailable'
  Object.assign(title.style, {
    fontSize: '14px',
    fontWeight: '600',
    color: '#f7768e',
    marginBottom: '8px'
  })

  const msg = document.createElement('div')
  msg.textContent = message
  Object.assign(msg.style, {
    fontSize: '12px',
    color: '#787c99',
    marginBottom: '12px',
    lineHeight: '1.4'
  })

  const inst = document.createElement('div')
  inst.textContent = instruction
  Object.assign(inst.style, {
    fontSize: '12px',
    color: '#a9b1d6',
    marginBottom: '8px'
  })

  const cmdBox = document.createElement('div')
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
  })
  const cmdText = document.createElement('span')
  cmdText.textContent = command
  cmdText.style.flex = '1'
  const copyIcon = document.createElement('span')
  copyIcon.textContent = 'Copy'
  Object.assign(copyIcon.style, {
    fontSize: '11px',
    color: '#565f89',
    flexShrink: '0'
  })
  cmdBox.appendChild(cmdText)
  cmdBox.appendChild(copyIcon)
  cmdBox.addEventListener('click', () => {
    void navigator.clipboard.writeText(command).then(() => {
      copyIcon.textContent = 'Copied!'
      copyIcon.style.color = '#9ece6a'
      setTimeout(() => {
        copyIcon.textContent = 'Copy'
        copyIcon.style.color = '#565f89'
      }, 2000)
    }).catch(() => {
      copyIcon.textContent = 'Select & copy manually'
      copyIcon.style.color = '#f7768e'
    })
  })

  const closeBtn = document.createElement('button')
  closeBtn.textContent = 'Dismiss'
  closeBtn.type = 'button'
  Object.assign(closeBtn.style, {
    background: '#292e42',
    border: 'none',
    borderRadius: '6px',
    padding: '6px 16px',
    color: '#a9b1d6',
    fontSize: '12px',
    cursor: 'pointer',
    width: '100%'
  })
  closeBtn.addEventListener('click', () => {
    overlay.remove()
    widgetEl = null
    visible = false
  })

  overlay.appendChild(title)
  overlay.appendChild(msg)
  overlay.appendChild(inst)
  overlay.appendChild(cmdBox)
  overlay.appendChild(closeBtn)

  widgetEl = overlay
  visible = true
  const target = document.body || document.documentElement
  if (target) target.appendChild(overlay)
}

function createWidget(token: string): HTMLDivElement {
  terminalConnected = false
  const widget = document.createElement('div')
  widget.id = WIDGET_ID
  Object.assign(widget.style, {
    position: 'fixed',
    bottom: '0',
    right: '0',
    width: DEFAULT_WIDGET_WIDTH,
    height: DEFAULT_WIDGET_HEIGHT,
    minWidth: MIN_WIDGET_WIDTH,
    minHeight: MIN_WIDGET_HEIGHT,
    maxWidth: MAX_WIDGET_WIDTH,
    maxHeight: MAX_WIDGET_HEIGHT,
    zIndex: '2147483644',
    display: 'flex',
    flexDirection: 'column',
    borderRadius: '12px 0 0 0',
    overflow: 'hidden',
    boxShadow: '0 -4px 24px rgba(0, 0, 0, 0.3)',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    transition: 'opacity 200ms ease, transform 200ms ease',
    transformOrigin: 'bottom right'
  })

  // Resize handle (top-left corner)
  const resizeHandle = document.createElement('div')
  Object.assign(resizeHandle.style, {
    position: 'absolute',
    top: '0',
    left: '0',
    width: '12px',
    height: '12px',
    cursor: 'nw-resize',
    zIndex: '10'
  })
  setupResize(resizeHandle, widget)
  resizeHandleEl = resizeHandle
  widget.appendChild(resizeHandle)

  // Header bar
  const header = document.createElement('div')
  header.id = HEADER_ID
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
  })

  // Connection status dot
  const statusDot = document.createElement('span')
  statusDot.className = 'gasoline-terminal-status-dot'
  Object.assign(statusDot.style, {
    width: '8px',
    height: '8px',
    borderRadius: '50%',
    background: '#565f89',
    flexShrink: '0',
    transition: 'background 200ms ease'
  })

  const titleSpan = document.createElement('span')
  titleSpan.textContent = 'Gasoline Terminal'
  Object.assign(titleSpan.style, {
    color: '#787c99',
    fontSize: '12px',
    fontWeight: '600',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    userSelect: 'none'
  })

  // Minimize button
  const minimizeTerminalButton = document.createElement('button')
  minimizeTerminalButton.id = MINIMIZE_TERMINAL_BUTTON_ID
  minimizeTerminalButton.textContent = '\u2581' // ▁
  minimizeTerminalButton.title = 'Minimize terminal'
  minimizeTerminalButton.type = 'button'
  Object.assign(minimizeTerminalButton.style, {
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
  })
  minimizeTerminalButton.addEventListener('mouseenter', () => {
    minimizeTerminalButton.style.background = '#292e42'
    minimizeTerminalButton.style.color = '#a9b1d6'
  })
  minimizeTerminalButton.addEventListener('mouseleave', () => {
    minimizeTerminalButton.style.background = 'transparent'
    minimizeTerminalButton.style.color = '#565f89'
  })
  minimizeTerminalButton.addEventListener('click', (e: MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    toggleMinimize(widget, minimizeTerminalButton, header)
  })

  // Exit session button — kills the PTY. Placed left, next to title, glows red.
  const disconnectTerminalButton = document.createElement('button')
  disconnectTerminalButton.id = DISCONNECT_TERMINAL_BUTTON_ID
  disconnectTerminalButton.textContent = '\u23FB' // ⏻ power symbol
  disconnectTerminalButton.title = 'disconnect terminal & and end session'
  disconnectTerminalButton.type = 'button'
  Object.assign(disconnectTerminalButton.style, {
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
  })
  disconnectTerminalButton.addEventListener('mouseenter', () => {
    disconnectTerminalButton.style.background = '#3b1219'
    disconnectTerminalButton.style.opacity = '1'
    disconnectTerminalButton.style.boxShadow = '0 0 8px rgba(247, 118, 142, 0.4)'
  })
  disconnectTerminalButton.addEventListener('mouseleave', () => {
    disconnectTerminalButton.style.background = 'transparent'
    disconnectTerminalButton.style.opacity = '0.7'
    disconnectTerminalButton.style.boxShadow = 'none'
  })
  disconnectTerminalButton.addEventListener('click', (e: MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    void exitTerminalSession()
  })

  // Spacer pushes minimize/close to the right
  const spacer = document.createElement('div')
  spacer.style.flex = '1'

  const redrawTerminalButton = document.createElement('button')
  redrawTerminalButton.id = REDRAW_TERMINAL_BUTTON_ID
  redrawTerminalButton.textContent = '\u21BB' // ↻
  redrawTerminalButton.title = 'Redraw terminal graphics'
  redrawTerminalButton.type = 'button'
  Object.assign(redrawTerminalButton.style, {
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
  })
  redrawTerminalButton.addEventListener('mouseenter', () => {
    redrawTerminalButton.style.background = '#292e42'
    redrawTerminalButton.style.color = '#a9b1d6'
  })
  redrawTerminalButton.addEventListener('mouseleave', () => {
    redrawTerminalButton.style.background = 'transparent'
    redrawTerminalButton.style.color = '#565f89'
  })
  redrawTerminalButton.addEventListener('click', (e: MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    redrawTerminal(widget, header, minimizeTerminalButton)
  })

  const closeTerminalButton = document.createElement('button')
  closeTerminalButton.id = CLOSE_TERMINAL_BUTTON_ID
  closeTerminalButton.textContent = '\u2715'
  closeTerminalButton.title = 'Close terminal'
  closeTerminalButton.type = 'button'
  Object.assign(closeTerminalButton.style, {
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
  })
  closeTerminalButton.addEventListener('mouseenter', () => {
    closeTerminalButton.style.background = '#292e42'
    closeTerminalButton.style.color = '#a9b1d6'
  })
  closeTerminalButton.addEventListener('mouseleave', () => {
    closeTerminalButton.style.background = 'transparent'
    closeTerminalButton.style.color = '#565f89'
  })
  closeTerminalButton.addEventListener('click', (e: MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    hideTerminal()
  })

  // Title bar click restores when minimized
  header.addEventListener('click', () => {
    if (!minimized) return
    toggleMinimize(widget, minimizeTerminalButton, header)
  })

  header.appendChild(statusDot)
  header.appendChild(titleSpan)
  header.appendChild(disconnectTerminalButton)
  header.appendChild(spacer)
  header.appendChild(redrawTerminalButton)
  header.appendChild(minimizeTerminalButton)
  header.appendChild(closeTerminalButton)

  // Iframe
  const iframe = document.createElement('iframe')
  iframe.id = IFRAME_ID
  iframe.src = `${getTerminalServerUrl(serverUrl)}/terminal?token=${encodeURIComponent(token)}`
  Object.assign(iframe.style, {
    flex: '1',
    width: '100%',
    border: 'none',
    background: '#1a1b26'
  })
  iframe.setAttribute('allow', 'clipboard-write')

  widget.appendChild(header)
  widget.appendChild(iframe)

  iframeEl = iframe

  // Listen for messages from the terminal iframe
  window.addEventListener('message', handleIframeMessage)

  return widget
}

function updateStatusDot(state: 'connected' | 'disconnected' | 'exited'): void {
  const dot = widgetEl?.querySelector('.gasoline-terminal-status-dot') as HTMLElement | null
  if (!dot) return
  switch (state) {
    case 'connected':
      dot.style.background = '#9ece6a' // green
      break
    case 'disconnected':
      dot.style.background = '#e0af68' // orange
      break
    case 'exited':
      dot.style.background = '#f7768e' // red
      break
  }
}

function handleIframeMessage(event: MessageEvent): void {
  if (!event.data || event.data.source !== 'gasoline-terminal') return
  // Only accept messages from the terminal server's origin (localhost:port+1)
  try {
    const termOrigin = getTerminalServerUrl(serverUrl)
    if (event.origin !== termOrigin) return
  } catch {
    return // Malformed serverUrl — reject all messages
  }
  // Handle terminal connection lifecycle events
  switch (event.data.event as string) {
    case 'connected':
      updateStatusDot('connected')
      terminalConnected = true
      if (queuedWrites.length > 0 && !queuedWriteInFlight) {
        scheduleQueuedWriteFlush(0)
      }
      break
    case 'disconnected':
      updateStatusDot('disconnected')
      terminalConnected = false
      terminalFocused = false
      break
    case 'exited':
      updateStatusDot('exited')
      terminalConnected = false
      terminalFocused = false
      resetWriteGuardState()
      break
    case 'focus':
      terminalFocused = Boolean((event.data.data as { focused?: boolean } | undefined)?.focused)
      if (terminalFocused) {
        lastTypingAt = Date.now()
      } else if (queuedWrites.length > 0 && !queuedWriteInFlight) {
        scheduleQueuedWriteFlush(0)
      }
      break
    case 'typing': {
      const rawAt = (event.data.data as { at?: number } | undefined)?.at
      const parsedAt = typeof rawAt === 'number' && Number.isFinite(rawAt) ? rawAt : Date.now()
      terminalFocused = true
      lastTypingAt = parsedAt
      break
    }
  }
}

function setupResize(handle: HTMLElement, widget: HTMLElement): void {
  let startX = 0
  let startY = 0
  let startWidth = 0
  let startHeight = 0

  function onMouseDown(e: MouseEvent): void {
    e.preventDefault()
    startX = e.clientX
    startY = e.clientY
    startWidth = widget.offsetWidth
    startHeight = widget.offsetHeight
    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
    // Prevent iframe from stealing mouse events during resize
    if (iframeEl) iframeEl.style.pointerEvents = 'none'
  }

  function onMouseMove(e: MouseEvent): void {
    const newWidth = startWidth - (e.clientX - startX)
    const newHeight = startHeight - (e.clientY - startY)
    widget.style.width = Math.max(400, Math.min(window.innerWidth, newWidth)) + 'px'
    widget.style.height = Math.max(250, Math.min(window.innerHeight * 0.8, newHeight)) + 'px'
  }

  function onMouseUp(): void {
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
    if (iframeEl) iframeEl.style.pointerEvents = 'auto'
    // Notify iframe to refit terminal
    notifyIframe('resize')
  }

  handle.addEventListener('mousedown', onMouseDown)
}

function redrawTerminal(widget: HTMLElement, header: HTMLElement, minimizeButton: HTMLButtonElement): void {
  if (minimized) {
    toggleMinimize(widget, minimizeButton, header)
  }

  savedHeight = DEFAULT_WIDGET_HEIGHT
  widget.style.bottom = '0'
  widget.style.right = '0'
  widget.style.width = DEFAULT_WIDGET_WIDTH
  widget.style.height = DEFAULT_WIDGET_HEIGHT
  widget.style.minWidth = MIN_WIDGET_WIDTH
  widget.style.minHeight = MIN_WIDGET_HEIGHT
  widget.style.maxWidth = MAX_WIDGET_WIDTH
  widget.style.maxHeight = MAX_WIDGET_HEIGHT
  widget.style.opacity = '1'
  widget.style.transform = 'translateY(0) scale(1)'
  widget.style.pointerEvents = 'auto'

  if (iframeEl) {
    iframeEl.style.display = 'block'
    updateStatusDot('disconnected')
    iframeEl.src = iframeEl.src
  }
  if (resizeHandleEl) resizeHandleEl.style.display = 'block'

  minimizeButton.textContent = '\u2581' // ▁
  minimizeButton.title = 'Minimize terminal'
  header.style.cursor = 'default'
  header.style.borderBottom = '1px solid #292e42'

  visible = true
  requestAnimationFrame(() => {
    notifyIframe('resize')
    notifyIframe('focus')
  })
  persistUIState('open')
}

function toggleMinimize(widget: HTMLElement, btn: HTMLButtonElement, header: HTMLElement): void {
  if (minimized) {
    // Restore
    minimized = false
    widget.style.height = savedHeight || DEFAULT_WIDGET_HEIGHT
    widget.style.minHeight = MIN_WIDGET_HEIGHT
    if (iframeEl) iframeEl.style.display = 'block'
    if (resizeHandleEl) resizeHandleEl.style.display = 'block'
    btn.textContent = '\u2581' // ▁
    btn.title = 'Minimize terminal'
    header.style.cursor = 'default'
    header.style.borderBottom = '1px solid #292e42'
    notifyIframe('resize')
    persistUIState('open')
  } else {
    // Minimize
    minimized = true
    savedHeight = widget.style.height || DEFAULT_WIDGET_HEIGHT
    widget.style.height = MINIMIZED_WIDGET_HEIGHT
    widget.style.minHeight = MINIMIZED_WIDGET_HEIGHT
    if (iframeEl) iframeEl.style.display = 'none'
    if (resizeHandleEl) resizeHandleEl.style.display = 'none'
    btn.textContent = '\u25A1' // □
    btn.title = 'Restore terminal'
    header.style.cursor = 'pointer'
    header.style.borderBottom = 'none'
    persistUIState('minimized')
  }
}

function notifyIframe(command: string, data?: Record<string, unknown>): void {
  if (!iframeEl?.contentWindow) return
  let origin = '*'
  try { origin = getTerminalServerUrl(serverUrl) } catch { /* fall back to wildcard */ }
  iframeEl.contentWindow.postMessage({
    target: 'gasoline-terminal',
    command,
    ...data
  }, origin)
}

function resetWriteGuardState(): void {
  queuedWrites = []
  terminalFocused = false
  lastTypingAt = 0
  queuedWriteInFlight = false
  lastGuardToastAt = 0
  if (queuedWriteFlushTimer !== null) {
    clearTimeout(queuedWriteFlushTimer)
    queuedWriteFlushTimer = null
  }
  if (queuedSubmitTimer !== null) {
    clearTimeout(queuedSubmitTimer)
    queuedSubmitTimer = null
  }
}

function shouldDeferQueuedWrite(nowMs = Date.now()): boolean {
  if (!terminalFocused) return false
  return nowMs - lastTypingAt < TERMINAL_TYPING_IDLE_MS
}

function maybeShowQueuedWriteToast(nowMs = Date.now()): void {
  if (nowMs - lastGuardToastAt < TERMINAL_GUARD_TOAST_INTERVAL_MS) return
  lastGuardToastAt = nowMs
  showActionToast('waiting for user to stop typing', 'Queued terminal action', 'warning', 1800)
}

function scheduleQueuedWriteFlush(delayMs = 0): void {
  if (queuedWriteFlushTimer !== null) clearTimeout(queuedWriteFlushTimer)
  queuedWriteFlushTimer = setTimeout(() => {
    queuedWriteFlushTimer = null
    flushQueuedWrites()
  }, delayMs)
}

function scheduleQueuedSubmit(delayMs: number): void {
  if (queuedSubmitTimer !== null) clearTimeout(queuedSubmitTimer)
  queuedSubmitTimer = setTimeout(() => {
    queuedSubmitTimer = null
    if (!visible || !iframeEl) {
      resetWriteGuardState()
      return
    }
    if (!terminalConnected) {
      scheduleQueuedSubmit(TERMINAL_GUARD_POLL_MS)
      return
    }
    if (shouldDeferQueuedWrite()) {
      maybeShowQueuedWriteToast()
      scheduleQueuedSubmit(TERMINAL_GUARD_POLL_MS)
      return
    }
    notifyIframe('write', { text: '\r' })
    notifyIframe('focus')
    queuedWriteInFlight = false
    if (queuedWrites.length > 0) {
      scheduleQueuedWriteFlush(0)
    }
  }, delayMs)
}

function flushQueuedWrites(): void {
  if (!visible || !iframeEl) {
    resetWriteGuardState()
    return
  }
  if (!terminalConnected) {
    scheduleQueuedWriteFlush(TERMINAL_GUARD_POLL_MS)
    return
  }
  if (queuedWriteInFlight) return
  if (queuedWrites.length === 0) {
    lastGuardToastAt = 0
    return
  }
  if (shouldDeferQueuedWrite()) {
    maybeShowQueuedWriteToast()
    scheduleQueuedWriteFlush(TERMINAL_GUARD_POLL_MS)
    return
  }

  const nextWrite = queuedWrites.shift()
  if (!nextWrite) return

  lastGuardToastAt = 0
  queuedWriteInFlight = true
  notifyIframe('redraw')
  notifyIframe('write', { text: nextWrite })
  scheduleQueuedSubmit(TERMINAL_WRITE_SUBMIT_DELAY_MS)
}

export function hideTerminal(): void {
  if (!widgetEl) return
  visible = false
  widgetEl.style.opacity = '0'
  widgetEl.style.transform = 'translateY(20px) scale(0.98)'
  widgetEl.style.pointerEvents = 'none'
  resetWriteGuardState()
  persistUIState('closed')
  // Session stays alive — can reconnect via toggle or page reload
}

/** Kill the PTY session on the daemon and tear down the widget completely. */
export async function exitTerminalSession(): Promise<void> {
  // Stop the PTY on the daemon (with timeout so the UI never hangs).
  if (sessionState) {
    try {
      const termUrl = getTerminalServerUrl(serverUrl)
      await fetch(`${termUrl}/terminal/stop`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: sessionState.sessionId }),
        signal: AbortSignal.timeout(3000)
      })
    } catch { /* daemon unreachable or timeout — tear down locally */ }
  }
  clearPersistedSession()
  unmountTerminal()
}

export function showTerminal(): void {
  if (!widgetEl) return
  visible = true
  widgetEl.style.opacity = '1'
  widgetEl.style.transform = 'translateY(0) scale(1)'
  widgetEl.style.pointerEvents = 'auto'
  notifyIframe('focus')
  persistUIState(minimized ? 'minimized' : 'open')
}

export function isTerminalVisible(): boolean {
  return visible
}

export async function toggleTerminal(): Promise<void> {
  if (visible && widgetEl) {
    hideTerminal()
    return
  }

  // If widget exists but hidden, just show it
  if (widgetEl && sessionState) {
    showTerminal()
    return
  }

  // Try to reconnect to a persisted session first
  await getServerUrl()
  const persisted = await loadPersistedSession()
  if (persisted.session) {
    const alive = await validateSession(persisted.session.token)
    if (alive) {
      sessionState = persisted.session
      mountWidget(persisted.session.token, persisted.uiState === 'minimized')
      return
    }
    // Session died — clear stale state and start fresh
    clearPersistedSession()
  }

  // Start a new session
  const config = await getTerminalConfig()
  const state = await startSession(config)
  if (!state) return

  sessionState = state
  mountWidget(state.token, false)
}

/** Restore terminal on page load if it was previously open/minimized. */
export async function restoreTerminalIfNeeded(): Promise<void> {
  const persisted = await loadPersistedSession()
  if (!persisted.session || persisted.uiState === 'closed') return

  await getServerUrl()
  const alive = await validateSession(persisted.session.token)
  if (!alive) {
    // Session died (daemon restart, process exited) but UI was open — start fresh.
    clearPersistedSession()
    const config = await getTerminalConfig()
    const state = await startSession(config)
    if (!state) return
    sessionState = state
    mountWidget(state.token, persisted.uiState === 'minimized')
    return
  }

  sessionState = persisted.session
  mountWidget(persisted.session.token, persisted.uiState === 'minimized')
}

function mountWidget(token: string, startMinimized: boolean): void {
  if (widgetEl) {
    widgetEl.remove()
    widgetEl = null
  }
  widgetEl = createWidget(token)
  const target = document.body || document.documentElement
  if (!target) return
  target.appendChild(widgetEl)

  // Animate in
  widgetEl.style.opacity = '0'
  widgetEl.style.transform = 'translateY(20px) scale(0.98)'
  requestAnimationFrame(() => {
    showTerminal()
    // Apply minimized state after show animation
    if (startMinimized) {
      const header = widgetEl?.querySelector('#' + HEADER_ID)
      const minimizeTerminalButton = header?.querySelector('#' + MINIMIZE_TERMINAL_BUTTON_ID) as HTMLButtonElement | null
      if (widgetEl && header && minimizeTerminalButton) {
        toggleMinimize(widgetEl, minimizeTerminalButton, header as HTMLElement)
      }
    }
  })
}

function unmountTerminal(): void {
  window.removeEventListener('message', handleIframeMessage)
  resetWriteGuardState()
  terminalConnected = false
  if (widgetEl) {
    widgetEl.remove()
    widgetEl = null
  }
  iframeEl = null
  resizeHandleEl = null
  sessionState = null
  visible = false
  minimized = false
  savedHeight = ''
}

/** Write text to the terminal PTY stdin via the iframe postMessage bridge, then press Enter to submit. */
export function writeToTerminal(text: string): void {
  if (!visible || !iframeEl) return
  // Strip trailing whitespace/newlines — we'll send our own Enter to submit.
  const trimmed = text.replace(/[\r\n\s]+$/, '')
  if (!trimmed) return
  queuedWrites.push(trimmed)
  scheduleQueuedWriteFlush(0)
}

// Re-export for launcher integration
export { saveTerminalConfig }
export type { TerminalConfig }

/**
 * Purpose: Renders a tracked-tab hover launcher for fast annotate/record/screenshot actions.
 * Why: Reduces popup churn by exposing common capture actions directly on tracked pages.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */

import type { ShowTrackedHoverLauncherMessage } from '../../types/runtime-messages.js'
import { RuntimeMessageName, StorageKey } from '../../lib/constants.js'
import { toggleTerminal, unmountTerminal } from './terminal-widget.js'

const ROOT_ID = 'gasoline-tracked-hover-launcher'
const PANEL_ID = 'gasoline-tracked-hover-panel'
const TOGGLE_ID = 'gasoline-tracked-hover-toggle'
const SETTINGS_MENU_ID = 'gasoline-tracked-hover-settings-menu'
const STORAGE_AREA_LOCAL = 'local'

type RecordingStorageValue = { active?: boolean }

let rootEl: HTMLDivElement | null = null
let panelEl: HTMLDivElement | null = null
let settingsMenuEl: HTMLDivElement | null = null
let recordButtonEl: HTMLButtonElement | null = null
let recordingActive = false
let panelPinned = false
let settingsMenuOpen = false
let trackedEnabled = false
let hiddenUntilPopupOpen = false
let hideTimer: ReturnType<typeof setTimeout> | null = null
let recordingStorageListener:
  | ((changes: Record<string, chrome.storage.StorageChange>, areaName: string) => void)
  | null = null
let runtimeListenerInstalled = false

function clearHideTimer(): void {
  if (!hideTimer) return
  clearTimeout(hideTimer)
  hideTimer = null
}

function setPanelOpen(open: boolean): void {
  if (!panelEl) return
  panelEl.style.opacity = open ? '1' : '0'
  panelEl.style.transform = open ? 'translateX(0) scale(1)' : 'translateX(12px) scale(0.96)'
  panelEl.style.pointerEvents = open ? 'auto' : 'none'
}

function setSettingsMenuOpen(open: boolean): void {
  settingsMenuOpen = open
  if (!settingsMenuEl) return
  settingsMenuEl.style.opacity = open ? '1' : '0'
  settingsMenuEl.style.transform = open ? 'translateY(0) scale(1)' : 'translateY(-8px) scale(0.96)'
  settingsMenuEl.style.pointerEvents = open ? 'auto' : 'none'
}

function updateRecordButtonState(active: boolean): void {
  recordingActive = active
  if (!recordButtonEl) return
  recordButtonEl.textContent = active ? '\u23F9' : '\u25C9'
  recordButtonEl.title = active ? 'Stop recording' : 'Record actions — capture clicks and inputs for replay'
  recordButtonEl.style.background = active ? '#c0392b' : '#f3f4f6'
  recordButtonEl.style.color = active ? '#fff' : '#1f2937'
  recordButtonEl.style.borderColor = active ? '#a93226' : '#d1d5db'
}

function readRecordingActive(value: unknown): boolean {
  if (!value || typeof value !== 'object') return false
  return Boolean((value as RecordingStorageValue).active)
}

function syncRecordingStateFromStorage(): void {
  try {
    chrome.storage.local.get([StorageKey.RECORDING], (result: Record<string, unknown>) => {
      if (chrome.runtime.lastError) return // Storage read failed — keep current state
      updateRecordButtonState(readRecordingActive(result[StorageKey.RECORDING]))
    })
  } catch {
    // Extension context invalidated — content script outlived the extension lifecycle
  }
}

function syncHiddenStateFromStorage(onSynced: () => void): void {
  try {
    chrome.storage.local.get([StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN], (result: Record<string, unknown>) => {
      if (chrome.runtime.lastError) {
        onSynced() // Proceed with default state on storage failure
        return
      }
      hiddenUntilPopupOpen = Boolean(result[StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN])
      onSynced()
    })
  } catch {
    onSynced() // Extension context invalidated — proceed with defaults
  }
}

function persistHiddenState(hidden: boolean): void {
  try {
    if (hidden) {
      chrome.storage.local.set({ [StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN]: true }, () => {
        void chrome.runtime.lastError // Best-effort persistence — no user-visible impact on failure
      })
      return
    }
    chrome.storage.local.remove(StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN, () => {
      void chrome.runtime.lastError
    })
  } catch {
    // Extension context invalidated — hidden state won't persist but functionality is unaffected
  }
}

function installRecordingStorageSync(): void {
  if (recordingStorageListener) return
  recordingStorageListener = (changes, areaName) => {
    if (areaName !== STORAGE_AREA_LOCAL) return
    const recordingChange = changes[StorageKey.RECORDING]
    if (!recordingChange) return
    updateRecordButtonState(readRecordingActive(recordingChange.newValue))
  }
  chrome.storage.onChanged.addListener(recordingStorageListener)
}

function uninstallRecordingStorageSync(): void {
  if (!recordingStorageListener) return
  chrome.storage.onChanged.removeListener(recordingStorageListener)
  recordingStorageListener = null
}

function hideLauncherUntilPopupReopen(): void {
  hiddenUntilPopupOpen = true
  persistHiddenState(true)
  setSettingsMenuOpen(false)
  unmountLauncher()
}

function handleReshowRequest(): void {
  hiddenUntilPopupOpen = false
  persistHiddenState(false)
  applyVisibilityFromState()
}

function installRuntimeListener(): void {
  if (runtimeListenerInstalled) return
  runtimeListenerInstalled = true

  chrome.runtime.onMessage.addListener(
    (message: ShowTrackedHoverLauncherMessage, sender: chrome.runtime.MessageSender) => {
      if (sender.id !== chrome.runtime.id) return false
      if (message.type !== RuntimeMessageName.SHOW_TRACKED_HOVER_LAUNCHER) return false
      handleReshowRequest()
      return false
    }
  )
}

function applyVisibilityFromState(): void {
  if (trackedEnabled && !hiddenUntilPopupOpen) {
    mountLauncher()
    return
  }
  unmountLauncher()
}

async function startDrawMode(): Promise<void> {
  try {
    if (!chrome?.runtime?.getURL) {
      console.warn('[Gasoline] Draw mode unavailable: extension context invalidated. Refresh the page to restore.')
      return
    }
    const drawModeModule = await import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
    if (typeof drawModeModule.activateDrawMode === 'function') {
      drawModeModule.activateDrawMode('user')
    }
  } catch (err) {
    console.warn('[Gasoline] Draw mode failed to load: ' + (err instanceof Error ? err.message : String(err)) +
      '. The extension may need to be reloaded at chrome://extensions.')
  }
}

// Primed AudioContext — created during user gesture so it won't be blocked.
// Reused across captures; closed lazily by the browser when the page unloads.
let shutterAudioCtx: AudioContext | null = null

function playShutterSound(): void {
  try {
    if (!shutterAudioCtx || shutterAudioCtx.state === 'closed') {
      shutterAudioCtx = new AudioContext()
    }
    const ctx = shutterAudioCtx
    // Resume in case the context was suspended (autoplay policy)
    if (ctx.state === 'suspended') void ctx.resume()
    const duration = 0.08
    const buffer = ctx.createBuffer(1, Math.ceil(ctx.sampleRate * duration), ctx.sampleRate)
    const data = buffer.getChannelData(0)
    for (let i = 0; i < data.length; i++) {
      const t = i / data.length
      const envelope = t < 0.1 ? t * 10 : Math.exp(-12 * (t - 0.1))
      data[i] = (Math.random() * 2 - 1) * envelope * 0.3
    }
    const source = ctx.createBufferSource()
    source.buffer = buffer
    source.connect(ctx.destination)
    source.start()
  } catch {
    // Audio unavailable — silent fallback
  }
}

function showScreenshotFlash(success: boolean): void {
  const flash = document.createElement('div')
  Object.assign(flash.style, {
    position: 'fixed',
    inset: '0',
    zIndex: '2147483647',
    background: success ? 'rgba(250,204,21,0.3)' : 'rgba(239,68,68,0.25)',
    pointerEvents: 'none',
    opacity: '1'
  })
  document.documentElement.appendChild(flash)
  // Hold the flash visible for 120ms before fading out
  setTimeout(() => {
    flash.style.transition = 'opacity 300ms ease-out'
    flash.style.opacity = '0'
  }, 120)
  setTimeout(() => flash.remove(), 450)
}

function runScreenshotCapture(): void {
  // Prime the AudioContext during the user gesture (click) so Chrome allows playback.
  if (!shutterAudioCtx || shutterAudioCtx.state === 'closed') {
    try { shutterAudioCtx = new AudioContext() } catch { /* no audio */ }
  }

  try {
    chrome.runtime.sendMessage(
      { type: 'captureScreenshot' },
      (response: { success?: boolean; error?: string } | undefined) => {
        const err = chrome.runtime.lastError
        const success = !err && response !== undefined && response.success !== false
        showScreenshotFlash(success)
        if (success) playShutterSound()
      }
    )
  } catch {
    showScreenshotFlash(false)
  }
}

function toggleRecordingAction(): void {
  const wasActive = recordingActive
  const message = wasActive ? { type: 'screen_recording_stop' } : { type: 'screen_recording_start', audio: '' }
  const button = recordButtonEl
  if (button) button.disabled = true

  chrome.runtime.sendMessage(
    message,
    (
      response:
        | { status?: 'recording' | 'saved' | 'error'; error?: string }
        | { success?: boolean; error?: string }
        | undefined
    ) => {
      if (button) button.disabled = false
      if (chrome.runtime.lastError) return

      const responseStatus = (response as { status?: string } | undefined)?.status
      if (wasActive) {
        if (responseStatus !== 'saved') {
          syncRecordingStateFromStorage()
          return
        }
        updateRecordButtonState(false)
        return
      }

      if (responseStatus === 'recording') {
        updateRecordButtonState(true)
        return
      }
      syncRecordingStateFromStorage()
    }
  )
}

function createActionButton(label: string, title: string, onClick: () => void): HTMLButtonElement {
  const button = document.createElement('button')
  button.textContent = label
  button.title = title
  button.type = 'button'
  Object.assign(button.style, {
    height: '48px',
    minWidth: '68px',
    borderRadius: '12px',
    border: '1px solid #d1d5db',
    background: '#f3f4f6',
    color: '#1f2937',
    fontSize: '32px',
    lineHeight: '1',
    fontWeight: '600',
    cursor: 'pointer',
    padding: '0 14px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    transition:
      'transform 140ms cubic-bezier(0.2, 0.8, 0.2, 1), box-shadow 160ms ease, background-color 160ms ease, border-color 160ms ease, color 160ms ease'
  })
  button.addEventListener('mouseenter', () => {
    button.style.transform = 'translateY(-1px)'
    button.style.boxShadow = '0 4px 12px rgba(15, 23, 42, 0.12)'
    button.style.color = '#ea580c'
  })
  button.addEventListener('mouseleave', () => {
    button.style.transform = 'translateY(0)'
    button.style.boxShadow = 'none'
    button.style.color = '#1f2937'
  })
  button.addEventListener('click', (event: MouseEvent) => {
    event.preventDefault()
    event.stopPropagation()
    onClick()
  })
  return button
}

function createSettingsMenuLink(label: string, href: string): HTMLAnchorElement {
  const link = document.createElement('a')
  link.textContent = label
  link.href = href
  link.target = '_blank'
  link.rel = 'noopener noreferrer'
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
  })
  link.addEventListener('mouseenter', () => {
    link.style.transform = 'translateX(1px)'
    link.style.background = '#f3f4f6'
  })
  link.addEventListener('mouseleave', () => {
    link.style.transform = 'translateX(0)'
    link.style.background = '#f9fafb'
  })
  link.addEventListener('click', () => {
    panelPinned = false
    setPanelOpen(false)
    setSettingsMenuOpen(false)
  })
  return link
}

function injectPulseKeyframes(): void {
  if (document.getElementById('gasoline-pulse-keyframes')) return
  const style = document.createElement('style')
  style.id = 'gasoline-pulse-keyframes'
  style.textContent = `
    @keyframes gasoline-pulse {
      0% { box-shadow: 0 0 0 0 rgba(249, 115, 22, 0.45); }
      70% { box-shadow: 0 0 0 10px rgba(249, 115, 22, 0); }
      100% { box-shadow: 0 0 0 0 rgba(249, 115, 22, 0); }
    }
  `
  ;(document.head || document.documentElement).appendChild(style)
}

function createLauncherUi(): HTMLDivElement {
  injectPulseKeyframes()

  const root = document.createElement('div')
  root.id = ROOT_ID
  Object.assign(root.style, {
    position: 'fixed',
    top: '33vh',
    right: '18px',
    zIndex: '2147483643',
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'
  })

  const panel = document.createElement('div')
  panel.id = PANEL_ID
  Object.assign(panel.style, {
    display: 'flex',
    alignItems: 'center',
    gap: '3px',
    padding: '4px',
    borderRadius: '16px',
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
  })

  const drawButton = createActionButton('\u270E', 'Annotate the page — draw, highlight, and mark up elements', () => {
    panelPinned = false
    setPanelOpen(false)
    void startDrawMode()
  })
  drawButton.style.fontSize = '36px'

  const recordButton = createActionButton('\u25C9', 'Record actions — capture clicks and inputs for replay', () => {
    panelPinned = true
    toggleRecordingAction()
  })
  recordButtonEl = recordButton
  recordButton.style.fontSize = '34px'

  const screenshotButton = createActionButton('\u2316', 'Screenshot — capture the current page and send to AI', () => {
    panelPinned = false
    setPanelOpen(false)
    runScreenshotCapture()
  })
  screenshotButton.style.fontSize = '38px'
  screenshotButton.style.paddingBottom = '8px'

  const terminalButton = createActionButton('_\u276F', 'Terminal — open an interactive CLI session', () => {
    panelPinned = false
    setPanelOpen(false)
    void toggleTerminal()
  })
  terminalButton.style.fontSize = '30px'

  const settingsButton = createActionButton('\u2699', 'Settings — docs, GitHub, hide launcher', () => {
    panelPinned = true
    setSettingsMenuOpen(!settingsMenuOpen)
  })
  settingsButton.style.fontSize = '45px'
  settingsButton.style.paddingBottom = '4px'

  panel.appendChild(drawButton)
  panel.appendChild(recordButton)
  panel.appendChild(screenshotButton)
  panel.appendChild(terminalButton)
  panel.appendChild(settingsButton)

  const settingsMenu = document.createElement('div')
  settingsMenu.id = SETTINGS_MENU_ID
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
  })

  const docsLink = createSettingsMenuLink('Docs', 'https://cookwithgasoline.com/docs')
  const repoLink = createSettingsMenuLink(
    'GitHub Repository',
    'https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp'
  )

  const hideButton = createActionButton('Hide Gasoline Devtool', 'Hide launcher until popup is opened again', () => {
    hideLauncherUntilPopupReopen()
  })
  hideButton.style.width = '100%'
  hideButton.style.justifyContent = 'center'

  settingsMenu.appendChild(docsLink)
  settingsMenu.appendChild(repoLink)
  settingsMenu.appendChild(hideButton)

  const toggle = document.createElement('button')
  toggle.id = TOGGLE_ID
  toggle.type = 'button'
  toggle.title = 'Gasoline quick actions'

  const toggleIcon = document.createElement('img')
  toggleIcon.src = chrome.runtime.getURL('icons/icon.svg')
  toggleIcon.alt = 'Gasoline'
  Object.assign(toggleIcon.style, {
    width: '52px',
    height: '52px',
    borderRadius: '50%',
    pointerEvents: 'none'
  })
  toggle.appendChild(toggleIcon)

  Object.assign(toggle.style, {
    width: '52px',
    height: '52px',
    borderRadius: '50%',
    border: 'none',
    background: 'transparent',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    cursor: 'pointer',
    padding: '0',
    boxShadow: '0 8px 24px rgba(15, 23, 42, 0.25)',
    transition: 'transform 180ms cubic-bezier(0.2, 0.8, 0.2, 1), box-shadow 180ms ease',
    overflow: 'hidden',
    animation: 'gasoline-pulse 2.5s ease-in-out infinite'
  })
  toggle.addEventListener('mouseenter', () => {
    toggle.style.transform = 'translateY(-1px)'
    toggle.style.boxShadow = '0 10px 26px rgba(15, 23, 42, 0.28)'
  })
  toggle.addEventListener('mouseleave', () => {
    toggle.style.transform = 'translateY(0)'
    toggle.style.boxShadow = '0 8px 24px rgba(15, 23, 42, 0.25)'
  })

  toggle.addEventListener('click', (event: MouseEvent) => {
    event.preventDefault()
    event.stopPropagation()
    panelPinned = !panelPinned
    clearHideTimer()
    setPanelOpen(panelPinned)
    if (!panelPinned) setSettingsMenuOpen(false)
  })

  root.addEventListener('mouseenter', () => {
    clearHideTimer()
    setPanelOpen(true)
  })

  root.addEventListener('mouseleave', () => {
    if (panelPinned || settingsMenuOpen) return
    clearHideTimer()
    hideTimer = setTimeout(() => {
      setPanelOpen(false)
      setSettingsMenuOpen(false)
    }, 120)
  })

  root.appendChild(panel)
  root.appendChild(toggle)
  root.appendChild(settingsMenu)

  panelEl = panel
  settingsMenuEl = settingsMenu
  syncRecordingStateFromStorage()

  return root
}

function mountLauncher(): void {
  if (hiddenUntilPopupOpen) return
  if (rootEl || document.getElementById(ROOT_ID)) return
  rootEl = createLauncherUi()
  const target = document.body || document.documentElement
  if (!target || !rootEl) return
  target.appendChild(rootEl)
  installRecordingStorageSync()
}

function unmountLauncher(): void {
  clearHideTimer()
  panelPinned = false
  setSettingsMenuOpen(false)
  panelEl = null
  settingsMenuEl = null
  recordButtonEl = null
  if (rootEl) {
    rootEl.remove()
    rootEl = null
  }
  uninstallRecordingStorageSync()
  unmountTerminal()
}

export function setTrackedHoverLauncherEnabled(enabled: boolean): void {
  trackedEnabled = enabled
  installRuntimeListener()
  syncHiddenStateFromStorage(applyVisibilityFromState)
}

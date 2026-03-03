/**
 * Purpose: Renders a tracked-tab hover launcher for fast annotate/record/screenshot actions.
 * Why: Reduces popup churn by exposing common capture actions directly on tracked pages.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */

import { StorageKey } from '../../lib/constants.js'

const ROOT_ID = 'gasoline-tracked-hover-launcher'
const PANEL_ID = 'gasoline-tracked-hover-panel'
const TOGGLE_ID = 'gasoline-tracked-hover-toggle'

type RecordingStorageValue = { active?: boolean }

let rootEl: HTMLDivElement | null = null
let panelEl: HTMLDivElement | null = null
let recordButtonEl: HTMLButtonElement | null = null
let recordingActive = false
let panelPinned = false
let hideTimer: ReturnType<typeof setTimeout> | null = null
let recordingStorageListener:
  | ((changes: Record<string, chrome.storage.StorageChange>, areaName: string) => void)
  | null = null

function clearHideTimer(): void {
  if (!hideTimer) return
  clearTimeout(hideTimer)
  hideTimer = null
}

function setPanelOpen(open: boolean): void {
  if (!panelEl) return
  panelEl.style.opacity = open ? '1' : '0'
  panelEl.style.transform = open ? 'translateX(0)' : 'translateX(8px)'
  panelEl.style.pointerEvents = open ? 'auto' : 'none'
}

function updateRecordButtonState(active: boolean): void {
  recordingActive = active
  if (!recordButtonEl) return
  recordButtonEl.textContent = active ? 'Stop' : 'Rec'
  recordButtonEl.title = active ? 'Stop recording' : 'Start recording'
  recordButtonEl.style.background = active ? '#c0392b' : '#f3f4f6'
  recordButtonEl.style.color = active ? '#fff' : '#1f2937'
  recordButtonEl.style.borderColor = active ? '#a93226' : '#d1d5db'
}

function readRecordingActive(value: unknown): boolean {
  if (!value || typeof value !== 'object') return false
  return Boolean((value as RecordingStorageValue).active)
}

function syncRecordingStateFromStorage(): void {
  chrome.storage.local.get([StorageKey.RECORDING], (result: Record<string, unknown>) => {
    updateRecordButtonState(readRecordingActive(result[StorageKey.RECORDING]))
  })
}

function installRecordingStorageSync(): void {
  if (recordingStorageListener) return
  recordingStorageListener = (changes, areaName) => {
    if (areaName !== 'local') return
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

async function startDrawMode(): Promise<void> {
  try {
    const drawModeModule = await import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
    if (typeof drawModeModule.activateDrawMode === 'function') {
      drawModeModule.activateDrawMode('user')
    }
  } catch {
    // Best-effort action; runtime listener provides canonical error handling.
  }
}

function runScreenshotCapture(): void {
  chrome.runtime.sendMessage({ type: 'captureScreenshot' }, () => {
    void chrome.runtime.lastError
  })
}

function toggleRecordingAction(): void {
  const wasActive = recordingActive
  const message = wasActive ? { type: 'record_stop' } : { type: 'record_start', audio: '' }
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
    height: '34px',
    minWidth: '54px',
    borderRadius: '10px',
    border: '1px solid #d1d5db',
    background: '#f3f4f6',
    color: '#1f2937',
    fontSize: '12px',
    fontWeight: '600',
    cursor: 'pointer',
    padding: '0 10px'
  })
  button.addEventListener('click', (event: MouseEvent) => {
    event.preventDefault()
    event.stopPropagation()
    onClick()
  })
  return button
}

function createLauncherUi(): HTMLDivElement {
  const root = document.createElement('div')
  root.id = ROOT_ID
  Object.assign(root.style, {
    position: 'fixed',
    top: '18px',
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
    gap: '6px',
    padding: '7px',
    borderRadius: '18px',
    background: '#ffffff',
    border: '1px solid rgba(15, 23, 42, 0.12)',
    boxShadow: '0 8px 24px rgba(15, 23, 42, 0.2)',
    opacity: '0',
    transform: 'translateX(8px)',
    transition: 'opacity 0.16s ease, transform 0.16s ease',
    pointerEvents: 'none'
  })

  const drawButton = createActionButton('Draw', 'Start annotation draw mode', () => {
    panelPinned = false
    setPanelOpen(false)
    void startDrawMode()
  })

  const recordButton = createActionButton('Rec', 'Start recording', () => {
    panelPinned = true
    toggleRecordingAction()
  })
  recordButtonEl = recordButton

  const screenshotButton = createActionButton('Shot', 'Capture screenshot', () => {
    panelPinned = false
    setPanelOpen(false)
    runScreenshotCapture()
  })

  panel.appendChild(drawButton)
  panel.appendChild(recordButton)
  panel.appendChild(screenshotButton)

  const toggle = document.createElement('button')
  toggle.id = TOGGLE_ID
  toggle.type = 'button'
  toggle.textContent = 'G'
  toggle.title = 'Gasoline quick actions'
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
    boxShadow: '0 8px 24px rgba(15, 23, 42, 0.25)'
  })

  toggle.addEventListener('click', (event: MouseEvent) => {
    event.preventDefault()
    event.stopPropagation()
    panelPinned = !panelPinned
    clearHideTimer()
    setPanelOpen(panelPinned)
  })

  root.addEventListener('mouseenter', () => {
    clearHideTimer()
    setPanelOpen(true)
  })

  root.addEventListener('mouseleave', () => {
    if (panelPinned) return
    clearHideTimer()
    hideTimer = setTimeout(() => setPanelOpen(false), 120)
  })

  root.appendChild(panel)
  root.appendChild(toggle)

  panelEl = panel
  syncRecordingStateFromStorage()

  return root
}

function mountLauncher(): void {
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
  panelEl = null
  recordButtonEl = null
  if (rootEl) {
    rootEl.remove()
    rootEl = null
  }
  uninstallRecordingStorageSync()
}

export function setTrackedHoverLauncherEnabled(enabled: boolean): void {
  if (enabled) {
    mountLauncher()
    return
  }
  unmountLauncher()
}

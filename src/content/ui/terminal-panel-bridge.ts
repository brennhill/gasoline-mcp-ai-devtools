/**
 * Purpose: Bridges the page hover launcher to the terminal side panel.
 * Why: Keeps the page overlay focused on quick actions while terminal visibility
 * and writes are coordinated through session state and runtime messages.
 * Docs: docs/features/feature/terminal/index.md
 */

import { StorageKey } from '../../lib/constants.js'
import { getSession, onStorageChanged } from '../../lib/storage-utils.js'

type VisibilityListener = (visible: boolean) => void

let panelVisible = false
let bridgeInitialized = false
let storageListenerInstalled = false
const visibilityListeners = new Set<VisibilityListener>()

function notifyVisibilityListeners(visible: boolean): void {
  for (const listener of visibilityListeners) {
    listener(visible)
  }
}

function setPanelVisible(nextVisible: boolean): void {
  if (panelVisible === nextVisible) return
  panelVisible = nextVisible
  notifyVisibilityListeners(panelVisible)
}

async function syncPanelVisibilityFromStorage(): Promise<void> {
  try {
    const value = await getSession(StorageKey.TERMINAL_UI_STATE)
    const uiState = value as string | undefined
    setPanelVisible(uiState === 'open')
  } catch {
    // Extension context invalidated - keep the last known visibility.
  }
}

function installStorageListener(): void {
  if (storageListenerInstalled) return
  storageListenerInstalled = true
  onStorageChanged((changes, areaName) => {
    if (areaName !== 'session') return
    const change = changes[StorageKey.TERMINAL_UI_STATE]
    if (!change) return
    const nextValue = change.newValue as string | undefined
    setPanelVisible(nextValue === 'open')
  })
}

export async function initTerminalPanelBridge(): Promise<void> {
  if (bridgeInitialized) return
  bridgeInitialized = true
  installStorageListener()
  await syncPanelVisibilityFromStorage()
}

export function isTerminalVisible(): boolean {
  return panelVisible
}

export function onTerminalPanelVisibilityChanged(listener: VisibilityListener): () => void {
  visibilityListeners.add(listener)
  return () => {
    visibilityListeners.delete(listener)
  }
}

export async function openTerminalPanel(): Promise<boolean> {
  try {
    const result = (await chrome.runtime.sendMessage({ type: 'open_terminal_panel' })) as
      | { success?: boolean }
      | undefined
    return result?.success === true
  } catch {
    return false
  }
}

export function writeToTerminal(text: string): void {
  if (!panelVisible) return
  try {
    chrome.runtime.sendMessage({ type: 'terminal_panel_write', text })
  } catch {
    // Extension context invalidated - writes are dropped.
  }
}

export const _terminalPanelBridgeForTests = {
  reset(): void {
    panelVisible = false
    bridgeInitialized = false
    storageListenerInstalled = false
    visibilityListeners.clear()
  }
}

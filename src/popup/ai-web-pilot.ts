/**
 * Purpose: Owns ai-web-pilot.ts runtime behavior and integration logic.
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview AI Web Pilot Toggle Module
 * Manages the AI Web Pilot feature toggle
 */

import { StorageKey } from '../lib/constants'

/**
 * Initialize the AI Web Pilot toggle.
 * Read the current state from chrome.storage.local.
 */
export async function initAiWebPilotToggle(): Promise<void> {
  const toggle = document.getElementById('aiWebPilotEnabled') as HTMLInputElement | null
  if (!toggle) return

  return new Promise((resolve) => {
    // Read from chrome.storage.local (single source of truth)
    chrome.storage.local.get([StorageKey.AI_WEB_PILOT_ENABLED], (result: { aiWebPilotEnabled?: boolean }) => {
      toggle.checked = result.aiWebPilotEnabled !== false

      // Set up change handler
      toggle.addEventListener('change', () => {
        handleAiWebPilotToggle(toggle.checked)
      })

      resolve()
    })
  })
}

/**
 * Handle AI Web Pilot toggle change.
 *
 * CRITICAL: ONLY background.js updates the state via setAiWebPilotEnabled message.
 * Popup NEVER writes to chrome.storage directly.
 *
 * This ensures single source of truth. If popup wrote to storage directly:
 * 1. Popup updates storage
 * 2. Background cache doesn't update (no listener yet)
 * 3. Pilot command checks cache and gets wrong value
 * 4. User sees toggle "on" but commands fail saying "off"
 *
 * By routing through background, we guarantee:
 * 1. Popup sends message to background
 * 2. Background updates cache immediately
 * 3. Background writes to storage
 * 4. Pilot commands see correct cache state
 * 5. Everything is consistent
 */
export async function handleAiWebPilotToggle(enabled: boolean): Promise<void> {
  // ONLY communicate with background - do NOT write to storage directly
  chrome.runtime.sendMessage(
    { type: 'setAiWebPilotEnabled', enabled },
    (response: { success?: boolean } | undefined) => {
      if (!response || !response.success) {
        console.error('[Gasoline] Failed to set AI Web Pilot toggle in background')
        // Revert the UI if background didn't accept the change
        const toggle = document.getElementById('aiWebPilotEnabled') as HTMLInputElement | null
        if (toggle) {
          toggle.checked = !enabled
        }
      }
    }
  )
}

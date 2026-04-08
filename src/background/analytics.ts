// analytics.ts — Anonymous daily usage telemetry.
// Sends a single daily heartbeat with boolean feature-usage flags.
// No URLs, no user data, no PII. Fingerprint is a permanent random UUID (not derived from identity).

import { getLocal, setLocal } from '../lib/storage-utils.js'

// =============================================================================
// CONSTANTS
// =============================================================================

const ANALYTICS_ENDPOINT = 'https://t.gokaboom.dev/api/events'

const ANALYTICS_STORAGE = {
  FINGERPRINT: 'kaboom_analytics_fingerprint',
  FIRST_SEEN_DATE: 'kaboom_analytics_first_seen',
  LAST_PING_DATE: 'kaboom_analytics_last_ping',
  DAILY_FLAGS: 'kaboom_analytics_daily_flags'
} as const

export const ALARM_NAME_ANALYTICS = 'analyticsPing'

/** How often to attempt a ping (hours). Server deduplicates by fingerprint+date. */
const PING_INTERVAL_HOURS = 4

// =============================================================================
// FEATURE CATEGORIES — boolean "was this used today" flags
// =============================================================================

export interface DailyFlags {
  ai_connected: boolean
  screenshot: boolean
  js_exec: boolean
  annotations: boolean
  video: boolean
  dom_action: boolean
  a11y: boolean
  network_observe: boolean
}

const EMPTY_FLAGS: DailyFlags = {
  ai_connected: false,
  screenshot: false,
  js_exec: false,
  annotations: false,
  video: false,
  dom_action: false,
  a11y: false,
  network_observe: false
}

/** Map command types dispatched via registry → feature flag */
const COMMAND_TO_FLAG: Record<string, keyof DailyFlags> = {
  screenshot: 'screenshot',
  execute: 'js_exec',
  draw_mode: 'annotations',
  screen_recording_start: 'video',
  screen_recording_stop: 'video',
  dom_action: 'dom_action',
  cdp_action: 'dom_action',
  browser_action: 'dom_action',
  a11y: 'a11y',
  waterfall: 'network_observe',
  page_info: 'network_observe',
  page_inventory: 'network_observe'
}

// =============================================================================
// IN-MEMORY STATE (persisted to storage periodically)
// =============================================================================

let currentFlags: DailyFlags = { ...EMPTY_FLAGS }
let flagsDirty = false

// =============================================================================
// FINGERPRINT
// =============================================================================

function generateFingerprint(): string {
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  // Format as UUID v4
  bytes[6] = (bytes[6]! & 0x0f) | 0x40
  bytes[8] = (bytes[8]! & 0x3f) | 0x80
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
  return [hex.slice(0, 8), hex.slice(8, 12), hex.slice(12, 16), hex.slice(16, 20), hex.slice(20)].join('-')
}

async function getOrCreateFingerprint(): Promise<string> {
  const fingerprint = (await getLocal(ANALYTICS_STORAGE.FINGERPRINT)) as string | undefined
  if (fingerprint) return fingerprint

  const newFingerprint = generateFingerprint()
  await setLocal(ANALYTICS_STORAGE.FINGERPRINT, newFingerprint)
  return newFingerprint
}

// =============================================================================
// DAILY FLAGS
// =============================================================================

async function loadFlags(): Promise<void> {
  const stored = (await getLocal(ANALYTICS_STORAGE.DAILY_FLAGS)) as DailyFlags | undefined
  if (stored) {
    currentFlags = { ...EMPTY_FLAGS, ...stored }
  }
}

async function persistFlags(): Promise<void> {
  if (!flagsDirty) return
  await setLocal(ANALYTICS_STORAGE.DAILY_FLAGS, currentFlags)
  flagsDirty = false
}

function resetFlags(): void {
  currentFlags = { ...EMPTY_FLAGS }
  flagsDirty = true
}

// =============================================================================
// PUBLIC API
// =============================================================================

/**
 * Called from command dispatch to record that a feature was used today.
 * Cheap — sets a boolean in memory, no async work.
 */
export function trackCommandUsage(commandType: string): void {
  const flag = COMMAND_TO_FLAG[commandType]
  if (flag && !currentFlags[flag]) {
    currentFlags[flag] = true
    flagsDirty = true
  }
}

/**
 * Called when AI/MCP connection is established.
 */
export function trackAiConnected(): void {
  if (!currentFlags.ai_connected) {
    currentFlags.ai_connected = true
    flagsDirty = true
  }
}

/**
 * Initialize analytics on extension startup.
 * Loads persisted flags, sets up alarm, sends initial ping if needed.
 */
export async function initAnalytics(): Promise<void> {
  if (typeof chrome === 'undefined' || !chrome.alarms) return

  await loadFlags()

  // Ensure first_seen_date is set
  const firstSeen = await getLocal(ANALYTICS_STORAGE.FIRST_SEEN_DATE)
  if (!firstSeen) {
    await setLocal(ANALYTICS_STORAGE.FIRST_SEEN_DATE, todayDateString())
  }

  // Create periodic alarm (survives SW restarts)
  chrome.alarms.create(ALARM_NAME_ANALYTICS, {
    periodInMinutes: PING_INTERVAL_HOURS * 60,
    delayInMinutes: 1 // First ping 1 minute after startup
  })
}

/**
 * Handle the analytics alarm firing. Called from alarm listener.
 */
export async function handleAnalyticsAlarm(): Promise<void> {
  await persistFlags()
  await sendPing()
}

// =============================================================================
// PING
// =============================================================================

interface AnalyticsPing {
  fingerprint: string
  date: string
  first_seen: string
  version: string
  is_new_install: boolean
  flags: DailyFlags
}

function todayDateString(): string {
  return new Date().toISOString().slice(0, 10) // YYYY-MM-DD
}

function getExtensionVersion(): string {
  try {
    return chrome.runtime.getManifest().version
  } catch {
    return 'unknown'
  }
}

async function sendPing(): Promise<void> {
  try {
    const today = todayDateString()

    // Check if we already pinged today with no new data
    const lastPing = (await getLocal(ANALYTICS_STORAGE.LAST_PING_DATE)) as string | undefined
    if (lastPing === today && !flagsDirty) return

    const fingerprint = await getOrCreateFingerprint()
    const firstSeen = ((await getLocal(ANALYTICS_STORAGE.FIRST_SEEN_DATE)) as string) || today

    // If day rolled over, send yesterday's accumulated flags first, then reset
    if (lastPing && lastPing !== today) {
      const yesterdayPing: AnalyticsPing = {
        fingerprint,
        date: lastPing,
        first_seen: firstSeen,
        version: getExtensionVersion(),
        is_new_install: false,
        flags: { ...currentFlags }
      }
      try {
        const resp = await fetch(ANALYTICS_ENDPOINT, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(yesterdayPing)
        })
        if (!resp.ok) { /* best-effort flush of previous day */ }
      } catch { /* best-effort */ }
      resetFlags()
    }

    const ping: AnalyticsPing = {
      fingerprint,
      date: today,
      first_seen: firstSeen,
      version: getExtensionVersion(),
      is_new_install: firstSeen === today,
      flags: { ...currentFlags }
    }

    const response = await fetch(ANALYTICS_ENDPOINT, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(ping)
    })

    if (response.ok) {
      await setLocal(ANALYTICS_STORAGE.LAST_PING_DATE, today)
    }
  } catch {
    // Silently fail — analytics must never break the extension
  }
}

/**
 * Purpose: Anonymous telemetry beacons for error visibility. Disable with STRUM_TELEMETRY storage key.
 */

const TELEMETRY_ENDPOINT = 'https://t.getstrum.dev/v1/event'

// Opt-out flag cached from chrome.storage.local for synchronous checks.
let telemetryDisabled = false

// Check opt-out on load
try {
  chrome.storage.local.get('strum_telemetry_off', (result) => {
    telemetryDisabled = result.strum_telemetry_off === true
  })
  // Listen for runtime changes so opt-out takes effect immediately
  chrome.storage.onChanged.addListener((changes, area) => {
    if (area === 'local' && 'strum_telemetry_off' in changes) {
      telemetryDisabled = changes.strum_telemetry_off.newValue === true
    }
  })
} catch {
  /* not in extension context */
}

/**
 * Fire an anonymous telemetry beacon. Fire-and-forget, never throws.
 * Uses navigator.sendBeacon when available, falls back to fetch.
 */
export function beacon(event: string, props: Record<string, string> = {}): void {
  if (telemetryDisabled) return

  try {
    const manifest = typeof chrome !== 'undefined' && chrome.runtime ? chrome.runtime.getManifest() : null
    const version = manifest?.version || 'unknown'

    const payload = JSON.stringify({ event, v: version, props })

    if (typeof navigator !== 'undefined' && navigator.sendBeacon) {
      const blob = new Blob([payload], { type: 'application/json' })
      navigator.sendBeacon(TELEMETRY_ENDPOINT, blob)
      return
    }

    // Fallback to fetch (fire-and-forget)
    void fetch(TELEMETRY_ENDPOINT, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: payload,
      keepalive: true
    }).catch(() => {
      /* never throw */
    })
  } catch {
    /* never throw */
  }
}

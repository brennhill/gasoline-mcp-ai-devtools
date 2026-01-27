// @ts-check
/**
 * @fileoverview Playwright fixture for Gasoline CI browser observability.
 * Automatically injects gasoline-ci.js into every page, manages test boundaries,
 * and captures failure context for Playwright reports.
 */

const { test: base } = require('@playwright/test')
const { resolve } = require('path')

const CI_SCRIPT_PATH = resolve(require.resolve('@anthropic/gasoline-ci'))

/**
 * @param {import('@playwright/test').TestInfo} testInfo
 * @returns {string}
 */
function deriveTestId(testInfo) {
  return testInfo.titlePath.join(' > ')
}

/**
 * @param {object} snapshot
 * @returns {string}
 */
function formatFailureSummary(snapshot) {
  if (!snapshot) return '=== Gasoline Failure Context ===\nNo snapshot data available.'

  const stats = snapshot.stats || {}
  const logs = Array.isArray(snapshot.logs) ? snapshot.logs : []
  const networkBodies = Array.isArray(snapshot.network_bodies) ? snapshot.network_bodies : []

  const lines = [
    '=== Gasoline Failure Context ===',
    `Captured at: ${snapshot.timestamp || 'unknown'}`,
    '',
    '--- Stats ---',
    `Total logs: ${stats.total_logs || 0}`,
    `Errors: ${stats.error_count || 0}`,
    `Warnings: ${stats.warning_count || 0}`,
    `Network failures: ${stats.network_failures || 0}`,
    `WebSocket connections: ${stats.ws_connections || 0}`,
  ]

  if ((stats.error_count || 0) > 0) {
    lines.push('', '--- Errors ---')
    for (const log of logs.filter((l) => l.level === 'error')) {
      lines.push(`  [${log.source || 'unknown'}] ${log.message || ''}`)
      if (log.stack) lines.push(`    ${log.stack.split('\n')[0]}`)
    }
  }

  if ((stats.network_failures || 0) > 0) {
    lines.push('', '--- Network Failures ---')
    for (const body of networkBodies.filter((b) => b.status >= 400)) {
      lines.push(`  ${body.method} ${body.url} → ${body.status}`)
      if (body.responseBody) {
        lines.push(`    ${body.responseBody.slice(0, 200)}`)
      }
    }
  }

  return lines.join('\n')
}

const test = base.extend({
  gasolinePort: [7890, { option: true }],
  gasolineAutoStart: [true, { option: true }],
  gasolineAttachOnFailure: [true, { option: true }],

  gasoline: async ({ gasolinePort, gasolineAttachOnFailure, page }, use, testInfo) => {
    const baseUrl = `http://127.0.0.1:${gasolinePort}`

    // Inject capture script into every page
    await page.addInitScript({ path: CI_SCRIPT_PATH })

    // Mark test start
    const testId = deriveTestId(testInfo)
    await fetch(`${baseUrl}/test-boundary`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ test_id: testId, action: 'start' }),
    }).catch(() => {}) // Server might not be running yet

    const fixture = {
      getSnapshot: async (since) => {
        try {
          const params = new URLSearchParams()
          if (since) params.set('since', since)
          params.set('test_id', testId)
          const res = await fetch(`${baseUrl}/snapshot?${params}`)
          return res.json()
        } catch {
          return { timestamp: new Date().toISOString(), stats: {}, logs: [], network_bodies: [] }
        }
      },
      clear: async () => {
        try {
          await fetch(`${baseUrl}/clear`, { method: 'POST' })
        } catch {
          // Server unreachable — safe to ignore
        }
      },
      markTest: async (id, action) => {
        try {
          await fetch(`${baseUrl}/test-boundary`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ test_id: id, action }),
          })
        } catch {
          // Server unreachable — safe to ignore
        }
      },
    }

    await use(fixture)

    // After test: if failed and attachOnFailure, grab snapshot
    if (testInfo.status !== testInfo.expectedStatus && gasolineAttachOnFailure) {
      try {
        const snapshot = await fixture.getSnapshot()
        await testInfo.attach('gasoline-snapshot', {
          body: JSON.stringify(snapshot, null, 2),
          contentType: 'application/json',
        })
        const summary = formatFailureSummary(snapshot)
        await testInfo.attach('gasoline-summary', {
          body: summary,
          contentType: 'text/plain',
        })
      } catch {
        // Server unreachable — skip attachment
      }
    }

    // Mark test end
    await fixture.markTest(testId, 'end')

    // Clear between tests
    await fixture.clear()
  },
})

module.exports = { test }

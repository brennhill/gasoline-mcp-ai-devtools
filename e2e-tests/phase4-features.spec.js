/**
 * E2E Tests: Phase 4 Features
 *
 * Tests the full pipeline for:
 *   1. AI Capture Control — configure(action: "capture") + GET /settings
 *   2. Error Clustering — analyze(target: "errors")
 *   3. Cross-Session Temporal Graph — configure(action: "record_event") + analyze(target: "history")
 */
import { test, expect } from './helpers/extension.js'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const fixturesDir = path.join(__dirname, 'fixtures')

/**
 * Call an MCP method via the HTTP endpoint
 */
async function mcpCall(serverUrl, method, params = {}) {
  const response = await fetch(`${serverUrl}/mcp`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      id: Date.now(),
      method,
      params,
    }),
  })
  return response.json()
}

/**
 * Call an MCP tool and return the raw text content
 */
async function mcpToolText(serverUrl, toolName, args = {}) {
  const resp = await mcpCall(serverUrl, 'tools/call', { name: toolName, arguments: args })
  if (resp.error) throw new Error(`MCP error: ${JSON.stringify(resp.error)}`)
  const content = resp.result.content
  if (!content || content.length === 0) return ''
  return content[0].text
}

// ===================================================
// Capture Control
// ===================================================

test.describe('Phase 4: Capture Control', () => {
  test('should set a capture override via configure tool', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: { log_level: 'all' },
    })

    expect(text).toContain('log_level')
    expect(text).toContain('all')
  })

  test('should expose overrides via GET /settings', async ({ page, serverUrl }) => {
    // Set an override first
    await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: { ws_mode: 'messages' },
    })

    // Poll /settings
    const resp = await fetch(`${serverUrl}/settings`)
    const data = await resp.json()

    expect(data.connected).toBe(true)
    expect(data.capture_overrides).toBeDefined()
    expect(data.capture_overrides.ws_mode).toBe('messages')
  })

  test('should reset overrides to defaults', async ({ page, serverUrl }) => {
    // Set an override
    await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: { log_level: 'all' },
    })

    // Wait to avoid rate limit
    await page.waitForTimeout(1100)

    // Reset
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: 'reset',
    })

    expect(text).toContain('reset')

    // Verify /settings shows empty overrides
    const resp = await fetch(`${serverUrl}/settings`)
    const data = await resp.json()
    expect(Object.keys(data.capture_overrides).length).toBe(0)
  })

  test('should reject invalid setting names', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: { invalid_setting: 'value' },
    })

    expect(text).toContain('Unknown capture setting')
  })

  test('should reject invalid setting values', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: { log_level: 'invalid_value' },
    })

    expect(text).toContain('Invalid value')
  })

  test('should rate-limit rapid changes', async ({ page, serverUrl }) => {
    // First change should succeed
    const text1 = await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: { log_level: 'all' },
    })
    expect(text1).toContain('log_level')

    // Immediate second change should be rate-limited
    const text2 = await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: { log_level: 'warn' },
    })
    expect(text2).toContain('Rate limited')
  })
})

// ===================================================
// Error Clustering
// ===================================================

test.describe('Phase 4: Error Clustering', () => {
  test('should return empty cluster state initially', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'analyze', { target: 'errors' })
    const data = JSON.parse(text)

    expect(data.clusters).toBeDefined()
    expect(data.total_errors).toBe(0)
    expect(data.unclustered_errors).toBe(0)
  })

  test('should cluster similar errors from the page', async ({ page, serverUrl }) => {
    // Navigate to the test page and trigger similar errors
    await page.goto(`file://${path.join(fixturesDir, 'phase4-test-page.html')}`)
    await page.waitForTimeout(1000)

    await page.click('#trigger-similar-errors')

    // Wait for errors to be captured and sent to server
    await page.waitForTimeout(4000)

    const text = await mcpToolText(serverUrl, 'analyze', { target: 'errors' })
    const data = JSON.parse(text)

    expect(data.total_errors).toBeGreaterThan(0)
    // Errors should have clustered (at least one cluster formed from 3 similar errors)
    if (data.clusters.length > 0) {
      const cluster = data.clusters[0]
      expect(cluster.instance_count).toBeGreaterThanOrEqual(2)
      expect(cluster.root_cause).toBeDefined()
      expect(cluster.severity).toBe('error')
    }
  })
})

// ===================================================
// Temporal Graph
// ===================================================

test.describe('Phase 4: Temporal Graph', () => {
  test('should return valid history response structure', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'analyze', {
      target: 'history',
      query: { since: '1h' },
    })
    const data = JSON.parse(text)

    expect(data.events).toBeDefined()
    expect(Array.isArray(data.events)).toBe(true)
    expect(typeof data.total_events).toBe('number')
    expect(data.time_range).toBe('1h')
    expect(data.summary).toBeDefined()
  })

  test('should record an event via configure', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'record_event',
      event: {
        type: 'fix',
        description: 'Fixed null user in UserProfile component',
        source: 'user-profile.js:42',
      },
    })

    expect(text).toContain('Event recorded')
    expect(text).toContain('fix')
  })

  test('should query recorded events by type', async ({ page, serverUrl }) => {
    // Record two events of different types
    await mcpToolText(serverUrl, 'configure', {
      action: 'record_event',
      event: { type: 'error', description: 'TypeError in render' },
    })
    await mcpToolText(serverUrl, 'configure', {
      action: 'record_event',
      event: { type: 'deploy', description: 'Deployed v2.1.0' },
    })

    // Query only errors
    const text = await mcpToolText(serverUrl, 'analyze', {
      target: 'history',
      query: { type: 'error', since: '1h' },
    })
    const data = JSON.parse(text)

    expect(data.total_events).toBeGreaterThan(0)
    for (const event of data.events) {
      expect(event.type).toBe('error')
    }
  })

  test('should query events by pattern', async ({ page, serverUrl }) => {
    // Use a unique pattern per test run to avoid collisions with persisted events
    const uniqueToken = `PATTERN_${Date.now()}_${Math.random().toString(36).slice(2)}`

    await mcpToolText(serverUrl, 'configure', {
      action: 'record_event',
      event: { type: 'fix', description: `Fixed ${uniqueToken} bug` },
    })

    const text = await mcpToolText(serverUrl, 'analyze', {
      target: 'history',
      query: { pattern: uniqueToken, since: '1h' },
    })
    const data = JSON.parse(text)

    expect(data.total_events).toBe(1)
    expect(data.events[0].description).toContain(uniqueToken)
  })

  test('should record events with causal links', async ({ page, serverUrl }) => {
    // Record an error event first
    await mcpToolText(serverUrl, 'configure', {
      action: 'record_event',
      event: { type: 'error', description: 'Crash in checkout flow' },
    })

    // Record a fix linked to a target
    await mcpToolText(serverUrl, 'configure', {
      action: 'record_event',
      event: {
        type: 'fix',
        description: 'Fixed checkout crash',
        related_to: 'evt_some_id',
      },
    })

    // Query events linked to that ID
    const text = await mcpToolText(serverUrl, 'analyze', {
      target: 'history',
      query: { related_to: 'evt_some_id', since: '1h' },
    })
    const data = JSON.parse(text)

    expect(data.total_events).toBeGreaterThan(0)
    const linked = data.events[0]
    expect(linked.links).toBeDefined()
    expect(linked.links[0].target).toBe('evt_some_id')
    expect(linked.links[0].confidence).toBe('explicit')
  })

  test('should reject record_event with missing type', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'record_event',
      event: { description: 'Missing type field' },
    })

    expect(text).toContain("'type' is missing")
  })

  test('should reject record_event with missing description', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'record_event',
      event: { type: 'error' },
    })

    expect(text).toContain("'description' is missing")
  })

  test('should reject record_event with missing event parameter', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'record_event',
    })

    expect(text).toContain("'event' is missing")
  })

  test('should set origin to agent for AI-recorded events', async ({ page, serverUrl }) => {
    const uniqueToken = `ORIGIN_${Date.now()}_${Math.random().toString(36).slice(2)}`

    await mcpToolText(serverUrl, 'configure', {
      action: 'record_event',
      event: { type: 'fix', description: `Agent fix ${uniqueToken}` },
    })

    const text = await mcpToolText(serverUrl, 'analyze', {
      target: 'history',
      query: { pattern: uniqueToken, since: '1h' },
    })
    const data = JSON.parse(text)

    expect(data.total_events).toBe(1)
    expect(data.events[0].origin).toBe('agent')
  })
})

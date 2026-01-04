/**
 * E2E Test: WebSocket Capture Flow
 *
 * Tests WebSocket lifecycle and message capture:
 *   Page creates WebSocket → inject.js intercepts →
 *   content.js bridges → background.js → POST /websocket-events
 *
 * Note: These tests require a WebSocket echo server spawned per suite.
 */
import { test, expect } from './helpers/extension.js'
import { createServer } from 'http'
import { WebSocketServer } from 'ws'
import path from 'path'
import { fileURLToPath } from 'url'
import net from 'net'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const fixturesDir = path.join(__dirname, 'fixtures')

function getFreePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer()
    server.listen(0, '127.0.0.1', () => {
      const port = server.address().port
      server.close(() => resolve(port))
    })
    server.on('error', reject)
  })
}

/**
 * Create a simple WebSocket echo server for testing
 */
async function createWsEchoServer() {
  const port = await getFreePort()
  const httpServer = createServer()
  const wss = new WebSocketServer({ server: httpServer, path: '/ws' })

  wss.on('connection', (ws) => {
    ws.on('message', (data) => {
      // Echo back
      ws.send(data.toString())
    })
  })

  await new Promise((resolve) => httpServer.listen(port, '127.0.0.1', resolve))

  return {
    port,
    url: `ws://127.0.0.1:${port}/ws`,
    close: () => new Promise((resolve) => {
      wss.close()
      httpServer.close(resolve)
    }),
  }
}

/**
 * Enable WebSocket capture by posting settings directly to the page's inject.js.
 * This simulates what content.js does when it receives messages from background.
 * Must be called AFTER the target page is loaded.
 */
async function enableWebSocketCapture(page, mode = 'lifecycle') {
  // Post settings directly to the page (same as content.js would)
  await page.evaluate((m) => {
    window.postMessage({ type: 'GASOLINE_SETTING', setting: 'setWebSocketCaptureEnabled', enabled: true }, '*')
    window.postMessage({ type: 'GASOLINE_SETTING', setting: 'setWebSocketCaptureMode', mode: m }, '*')
  }, mode)
  // Wait for inject.js to process the messages
  await page.waitForTimeout(500)
}

test.describe('WebSocket Capture', () => {
  let wsServer

  test.beforeAll(async () => {
    wsServer = await createWsEchoServer()
  })

  test.afterAll(async () => {
    if (wsServer) await wsServer.close()
  })

  test('should capture WebSocket open event when enabled', async ({ page, serverUrl, extensionId, context }) => {
    // Navigate to test page first (inject.js loads)
    await page.goto(`file://${path.join(fixturesDir, 'websocket-page.html')}?wsPort=${wsServer.port}`)
    await page.waitForTimeout(1000)

    // Enable WebSocket capture (sends message through background → content → inject)
    await enableWebSocketCapture(page)

    // Connect WebSocket
    await page.click('#connect-ws')

    // Wait for connection and batch delivery
    await page.waitForSelector('#ws-state:has-text("Connected")', { timeout: 5000 })
    await page.waitForTimeout(3000)

    // Query server for WebSocket events
    const response = await fetch(`${serverUrl}/websocket-events`)
    const data = await response.json()

    expect(data.events).toBeDefined()
    const openEvent = data.events.find((e) => e.event === 'open')
    expect(openEvent).toBeDefined()
    expect(openEvent.url).toContain(`127.0.0.1:${wsServer.port}`)
  })

  test('should capture WebSocket close event', async ({ page, serverUrl, extensionId, context }) => {
    await page.goto(`file://${path.join(fixturesDir, 'websocket-page.html')}?wsPort=${wsServer.port}`)
    await page.waitForTimeout(1000)

    await enableWebSocketCapture(page)

    // Connect then disconnect
    await page.click('#connect-ws')
    await page.waitForSelector('#ws-state:has-text("Connected")', { timeout: 5000 })
    await page.click('#close-ws')
    await page.waitForSelector('#ws-state:has-text("Disconnected")', { timeout: 5000 })

    // Wait for batch delivery
    await page.waitForTimeout(3000)

    const response = await fetch(`${serverUrl}/websocket-events`)
    const data = await response.json()

    expect(data.events).toBeDefined()
    const closeEvent = data.events.find((e) => e.event === 'close')
    expect(closeEvent).toBeDefined()
  })

  test('should capture messages when mode is "messages"', async ({ page, serverUrl, extensionId, context }) => {
    await page.goto(`file://${path.join(fixturesDir, 'websocket-page.html')}?wsPort=${wsServer.port}`)
    await page.waitForTimeout(1000)

    // Enable with messages mode
    await enableWebSocketCapture(page, 'messages')

    // Connect and send message
    await page.click('#connect-ws')
    await page.waitForSelector('#ws-state:has-text("Connected")', { timeout: 5000 })
    await page.click('#send-msg')

    // Wait for echo and batch delivery
    await page.waitForTimeout(3000)

    const response = await fetch(`${serverUrl}/websocket-events`)
    const data = await response.json()

    expect(data.events).toBeDefined()
    // Should have outgoing message event
    const outgoing = data.events.find((e) => e.event === 'message' && e.direction === 'outgoing')
    expect(outgoing).toBeDefined()
  })

  test('should NOT capture messages when mode is "lifecycle"', async ({ page, serverUrl, extensionId, context }) => {
    await page.goto(`file://${path.join(fixturesDir, 'websocket-page.html')}?wsPort=${wsServer.port}`)
    await page.waitForTimeout(1000)

    // Enable with lifecycle mode (default)
    await enableWebSocketCapture(page, 'lifecycle')

    // Connect and send message
    await page.click('#connect-ws')
    await page.waitForSelector('#ws-state:has-text("Connected")', { timeout: 5000 })
    await page.click('#send-msg')

    // Wait for batch delivery
    await page.waitForTimeout(3000)

    const response = await fetch(`${serverUrl}/websocket-events`)
    const data = await response.json()

    // Should have open event but no message events
    const openEvents = (data.events || []).filter((e) => e.event === 'open')
    const messageEvents = (data.events || []).filter((e) => e.event === 'message')

    expect(openEvents.length).toBeGreaterThan(0)
    expect(messageEvents.length).toBe(0)
  })

  test('should not capture WebSocket events when disabled', async ({ page, serverUrl, extensionId, context }) => {
    await page.goto(`file://${path.join(fixturesDir, 'websocket-page.html')}?wsPort=${wsServer.port}`)
    await page.waitForTimeout(1000)

    // Don't enable WebSocket capture (it's disabled by default)

    await page.click('#connect-ws')
    await page.waitForSelector('#ws-state:has-text("Connected")', { timeout: 5000 })

    // Wait for potential batch delivery
    await page.waitForTimeout(3000)

    const response = await fetch(`${serverUrl}/websocket-events`)
    const data = await response.json()

    // Should have no WebSocket events
    const wsEvents = (data.events || []).filter((e) => ['open', 'close', 'message'].includes(e.event))
    expect(wsEvents.length).toBe(0)
  })
})

/**
 * E2E Test: WebSocket Capture Flow
 *
 * Tests WebSocket lifecycle and message capture:
 *   Page creates WebSocket → inject.js intercepts →
 *   content.js bridges → background.js → POST /websocket-events
 *
 * Note: These tests require a WebSocket server. We use a simple
 * echo server spawned as part of the test setup.
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

test.describe('WebSocket Capture', () => {
  let wsServer

  test.beforeAll(async () => {
    wsServer = await createWsEchoServer()
  })

  test.afterAll(async () => {
    if (wsServer) await wsServer.close()
  })

  test('should capture WebSocket open event when enabled', async ({ page, serverUrl, extensionId, context }) => {
    // Enable WebSocket capture via extension storage
    const setupPage = await context.newPage()
    await setupPage.goto(`chrome-extension://${extensionId}/options.html`)
    await setupPage.evaluate(() => {
      return new Promise((resolve) => {
        chrome.storage.local.set({ webSocketCaptureEnabled: true }, resolve)
      })
    })
    await setupPage.close()

    // Navigate to test page with WS port
    await page.goto(`file://${path.join(fixturesDir, 'websocket-page.html')}?wsPort=${wsServer.port}`)
    await page.waitForTimeout(1000)

    // Connect WebSocket
    await page.click('#connect-ws')

    // Wait for connection and batch delivery
    await page.waitForSelector('#ws-state:has-text("Connected")', { timeout: 5000 })
    await page.waitForTimeout(3000)

    // Query server for WebSocket events
    const response = await fetch(`${serverUrl}/websocket-events`)
    const data = await response.json()

    expect(data.events).toBeDefined()
    const openEvent = data.events.find((e) => e.type === 'open')
    expect(openEvent).toBeDefined()
    expect(openEvent.url).toContain(`127.0.0.1:${wsServer.port}`)
  })

  test('should capture WebSocket close event', async ({ page, serverUrl, extensionId, context }) => {
    // Enable WebSocket capture
    const setupPage = await context.newPage()
    await setupPage.goto(`chrome-extension://${extensionId}/options.html`)
    await setupPage.evaluate(() => {
      return new Promise((resolve) => {
        chrome.storage.local.set({ webSocketCaptureEnabled: true }, resolve)
      })
    })
    await setupPage.close()

    await page.goto(`file://${path.join(fixturesDir, 'websocket-page.html')}?wsPort=${wsServer.port}`)
    await page.waitForTimeout(1000)

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
    const closeEvent = data.events.find((e) => e.type === 'close')
    expect(closeEvent).toBeDefined()
  })

  test('should capture messages when mode is "messages"', async ({ page, serverUrl, extensionId, context }) => {
    // Enable WebSocket capture in messages mode
    const setupPage = await context.newPage()
    await setupPage.goto(`chrome-extension://${extensionId}/options.html`)
    await setupPage.evaluate(() => {
      return new Promise((resolve) => {
        chrome.storage.local.set({
          webSocketCaptureEnabled: true,
          webSocketCaptureMode: 'messages',
        }, resolve)
      })
    })
    await setupPage.close()

    await page.goto(`file://${path.join(fixturesDir, 'websocket-page.html')}?wsPort=${wsServer.port}`)
    await page.waitForTimeout(1000)

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
    const outgoing = data.events.find((e) => e.type === 'message' && e.direction === 'outgoing')
    expect(outgoing).toBeDefined()
  })

  test('should NOT capture messages when mode is "lifecycle"', async ({ page, serverUrl, extensionId, context }) => {
    // Enable WebSocket capture in lifecycle mode (default)
    const setupPage = await context.newPage()
    await setupPage.goto(`chrome-extension://${extensionId}/options.html`)
    await setupPage.evaluate(() => {
      return new Promise((resolve) => {
        chrome.storage.local.set({
          webSocketCaptureEnabled: true,
          webSocketCaptureMode: 'lifecycle',
        }, resolve)
      })
    })
    await setupPage.close()

    await page.goto(`file://${path.join(fixturesDir, 'websocket-page.html')}?wsPort=${wsServer.port}`)
    await page.waitForTimeout(1000)

    // Connect and send message
    await page.click('#connect-ws')
    await page.waitForSelector('#ws-state:has-text("Connected")', { timeout: 5000 })
    await page.click('#send-msg')

    // Wait for batch delivery
    await page.waitForTimeout(3000)

    const response = await fetch(`${serverUrl}/websocket-events`)
    const data = await response.json()

    // Should have open event but no message events
    const openEvents = data.events?.filter((e) => e.type === 'open') || []
    const messageEvents = data.events?.filter((e) => e.type === 'message') || []

    expect(openEvents.length).toBeGreaterThan(0)
    expect(messageEvents.length).toBe(0)
  })

  test('should not capture WebSocket events when disabled', async ({ page, serverUrl, extensionId, context }) => {
    // Ensure WebSocket capture is disabled
    const setupPage = await context.newPage()
    await setupPage.goto(`chrome-extension://${extensionId}/options.html`)
    await setupPage.evaluate(() => {
      return new Promise((resolve) => {
        chrome.storage.local.set({ webSocketCaptureEnabled: false }, resolve)
      })
    })
    await setupPage.close()

    await page.goto(`file://${path.join(fixturesDir, 'websocket-page.html')}?wsPort=${wsServer.port}`)
    await page.waitForTimeout(1000)

    await page.click('#connect-ws')
    await page.waitForSelector('#ws-state:has-text("Connected")', { timeout: 5000 })

    // Wait for potential batch delivery
    await page.waitForTimeout(3000)

    const response = await fetch(`${serverUrl}/websocket-events`)
    const data = await response.json()

    // Should have no WebSocket events
    const wsEvents = data.events?.filter((e) => ['open', 'close', 'message'].includes(e.type)) || []
    expect(wsEvents.length).toBe(0)
  })
})

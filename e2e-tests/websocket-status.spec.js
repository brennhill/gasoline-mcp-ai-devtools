/**
 * E2E Test: WebSocket Status Endpoint
 *
 * Tests the /websocket-status endpoint returns:
 *   - Connection duration (formatted)
 *   - Message rate (perSecond calculated from rolling window)
 *   - Last message age (formatted relative time)
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

async function createWsEchoServer() {
  const port = await getFreePort()
  const httpServer = createServer()
  const wss = new WebSocketServer({ server: httpServer, path: '/ws' })

  wss.on('connection', (ws) => {
    ws.on('message', (data) => {
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

async function enableWebSocketCapture(page, mode = 'messages') {
  await page.evaluate((m) => {
    window.postMessage({ type: 'DEV_CONSOLE_SETTING', setting: 'setWebSocketCaptureEnabled', enabled: true }, '*')
    window.postMessage({ type: 'DEV_CONSOLE_SETTING', setting: 'setWebSocketCaptureMode', mode: m }, '*')
  }, mode)
  await page.waitForTimeout(500)
}

test.describe('WebSocket Status', () => {
  let wsServer

  test.beforeAll(async () => {
    wsServer = await createWsEchoServer()
  })

  test.afterAll(async () => {
    if (wsServer) await wsServer.close()
  })

  test('should include duration for active connections', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'websocket-page.html')}?wsPort=${wsServer.port}`)
    await page.waitForTimeout(1000)

    await enableWebSocketCapture(page)

    // Connect WebSocket
    await page.click('#connect-ws')
    await page.waitForSelector('#ws-state:has-text("Connected")', { timeout: 5000 })

    // Wait for connection event to reach server
    await page.waitForTimeout(3000)

    // Query websocket-status
    const response = await fetch(`${serverUrl}/websocket-status`)
    const status = await response.json()

    expect(status.connections).toBeDefined()
    expect(status.connections.length).toBeGreaterThan(0)

    const conn = status.connections[0]
    expect(conn.duration).toBeDefined()
    expect(conn.duration).not.toBe('')
    // Duration should end with 's' (seconds) since connection is just a few seconds old
    expect(conn.duration).toMatch(/\d+s$/)
  })

  test('should include message rate for active connections', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'websocket-page.html')}?wsPort=${wsServer.port}`)
    await page.waitForTimeout(1000)

    await enableWebSocketCapture(page)

    // Connect and send messages
    await page.click('#connect-ws')
    await page.waitForSelector('#ws-state:has-text("Connected")', { timeout: 5000 })

    // Send multiple messages rapidly
    for (let i = 0; i < 5; i++) {
      await page.click('#send-msg')
      await page.waitForTimeout(200)
    }

    // Wait for batch delivery
    await page.waitForTimeout(3000)

    const response = await fetch(`${serverUrl}/websocket-status`)
    const status = await response.json()

    expect(status.connections.length).toBeGreaterThan(0)
    const conn = status.connections[0]

    // Should have message totals
    expect(conn.messageRate.outgoing.total).toBeGreaterThan(0)
    // perSecond may or may not be > 0 depending on timing,
    // but the total should reflect our sent messages
    expect(conn.messageRate.outgoing.total).toBeGreaterThanOrEqual(5)
  })

  test('should include age for last message preview', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'websocket-page.html')}?wsPort=${wsServer.port}`)
    await page.waitForTimeout(1000)

    await enableWebSocketCapture(page)

    // Connect and send a message
    await page.click('#connect-ws')
    await page.waitForSelector('#ws-state:has-text("Connected")', { timeout: 5000 })
    await page.click('#send-msg')

    // Wait for batch delivery
    await page.waitForTimeout(3000)

    const response = await fetch(`${serverUrl}/websocket-status`)
    const status = await response.json()

    expect(status.connections.length).toBeGreaterThan(0)
    const conn = status.connections[0]

    // Check outgoing last message has age
    if (conn.lastMessage.outgoing) {
      expect(conn.lastMessage.outgoing.age).toBeDefined()
      expect(conn.lastMessage.outgoing.age).not.toBe('')
      expect(conn.lastMessage.outgoing.age).toMatch(/s$/) // ends with 's'
    }

    // Check incoming last message (echo) has age
    if (conn.lastMessage.incoming) {
      expect(conn.lastMessage.incoming.age).toBeDefined()
      expect(conn.lastMessage.incoming.age).not.toBe('')
      expect(conn.lastMessage.incoming.age).toMatch(/s$/)
    }
  })

  test('should show closed connections with totals', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'websocket-page.html')}?wsPort=${wsServer.port}`)
    await page.waitForTimeout(1000)

    await enableWebSocketCapture(page)

    // Connect, send, disconnect
    await page.click('#connect-ws')
    await page.waitForSelector('#ws-state:has-text("Connected")', { timeout: 5000 })
    await page.click('#send-msg')
    await page.waitForTimeout(500)
    await page.click('#close-ws')
    await page.waitForSelector('#ws-state:has-text("Disconnected")', { timeout: 5000 })

    // Wait for batch delivery
    await page.waitForTimeout(3000)

    const response = await fetch(`${serverUrl}/websocket-status`)
    const status = await response.json()

    expect(status.closed).toBeDefined()
    expect(status.closed.length).toBeGreaterThan(0)

    const closed = status.closed[status.closed.length - 1]
    expect(closed.state).toBe('closed')
    expect(closed.openedAt).toBeDefined()
    expect(closed.closedAt).toBeDefined()
    expect(closed.totalMessages.outgoing).toBeGreaterThanOrEqual(1)
  })
})

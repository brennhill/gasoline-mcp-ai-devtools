/**
 * Shared WebSocket test helpers for E2E tests.
 *
 * Provides getFreePort, createWsEchoServer, and enableWebSocketCapture
 * used across websocket-capture.spec.js and websocket-status.spec.js.
 */
import { createServer } from 'http'
import { WebSocketServer } from 'ws'
import net from 'net'

/**
 * Find a free port on localhost
 * @returns {Promise<number>} Available port number
 */
export function getFreePort() {
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
 * @returns {Promise<{port: number, url: string, close: () => Promise<void>}>}
 */
export async function createWsEchoServer() {
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
 * @param {Object} page - Playwright page object
 * @param {string} mode - Capture mode: 'low', 'medium', 'high', or 'all' (required)
 */
export async function enableWebSocketCapture(page, mode) {
  if (!mode) throw new Error('enableWebSocketCapture requires a mode parameter ("low", "medium", "high", or "all")')
  // Post settings directly to the page (same as content.js would)
  await page.evaluate((m) => {
    window.postMessage({ type: 'GASOLINE_SETTING', setting: 'setWebSocketCaptureEnabled', enabled: true }, window.location.origin)
    window.postMessage({ type: 'GASOLINE_SETTING', setting: 'setWebSocketCaptureMode', mode: m }, window.location.origin)
  }, mode)
  // Wait for inject.js to process the messages
  await page.waitForTimeout(500)
}

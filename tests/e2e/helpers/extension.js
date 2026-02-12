/**
 * Custom Playwright fixture for Chrome extension E2E testing.
 *
 * Provides:
 * - A running Go server on a random port
 * - A Chromium browser with the extension loaded
 * - Helper methods for interacting with the MCP server
 */
import { test as base, chromium } from '@playwright/test'
import { spawn } from 'child_process'
import path from 'path'
import { fileURLToPath } from 'url'
import net from 'net'
import fs from 'fs'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const projectRoot = path.join(__dirname, '..', '..')
const extensionPath = path.join(projectRoot, 'extension')
const binaryPath = path.join(projectRoot, 'dist', 'gasoline')

/**
 * Find a free port on localhost
 */
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
 * Wait for the server to be ready by polling its endpoint
 */
async function waitForServer(port, timeoutMs = 10000) {
  const start = Date.now()
  while (Date.now() - start < timeoutMs) {
    try {
      const response = await fetch(`http://127.0.0.1:${port}/health`)
      if (response.ok) return true
    } catch {
      // Server not ready yet
    }
    await new Promise((r) => setTimeout(r, 100))
  }
  throw new Error(`Server failed to start within ${timeoutMs}ms`)
}

/**
 * Start the Go server as a subprocess
 */
async function startServer(port, logFile) {
  if (!fs.existsSync(binaryPath)) {
    throw new Error(`Server binary not found at ${binaryPath}. Run 'make dev' first.`)
  }

  const serverProcess = spawn(binaryPath, ['--server', '--port', String(port), '--log-file', logFile], {
    env: process.env,
    stdio: ['pipe', 'pipe', 'pipe']
  })

  // Collect stderr for debugging
  let stderr = ''
  serverProcess.stderr.on('data', (data) => {
    stderr += data.toString()
  })

  serverProcess.on('error', (err) => {
    throw new Error(`Failed to start server: ${err.message}\nStderr: ${stderr}`)
  })

  await waitForServer(port)
  return serverProcess
}

/**
 * Extended Playwright test fixture with extension + server
 */
export const test = base.extend({
  /**
   * The server port for this test
   */
  serverPort: async ({}, use) => {
    const port = await getFreePort()
    await use(port)
  },

  /**
   * The log file path for this test
   */
  logFile: async ({}, use) => {
    const tmpDir = path.join(projectRoot, 'e2e-tests', '.tmp')
    if (!fs.existsSync(tmpDir)) {
      fs.mkdirSync(tmpDir, { recursive: true })
    }
    const logFile = path.join(tmpDir, `test-${Date.now()}-${Math.random().toString(36).slice(2)}.log`)
    await use(logFile)
    // Cleanup
    if (fs.existsSync(logFile)) {
      fs.unlinkSync(logFile)
    }
  },

  /**
   * Running server process
   */
  server: async ({ serverPort, logFile }, use) => {
    const proc = await startServer(serverPort, logFile)
    await use(proc)
    proc.kill('SIGTERM')
    // Wait for process to exit
    await new Promise((resolve) => {
      proc.on('exit', resolve)
      setTimeout(resolve, 2000) // Force timeout
    })
  },

  /**
   * Browser context with extension loaded, configured to talk to the test server
   */
  context: async ({ serverPort, server }, use) => {
    const userDataDir = path.join(projectRoot, 'e2e-tests', '.tmp', `chrome-profile-${Date.now()}`)
    const isReview = !!process.env.REVIEW

    const args = [
      `--disable-extensions-except=${extensionPath}`,
      `--load-extension=${extensionPath}`,
      '--no-first-run',
      '--disable-default-apps',
      '--disable-popup-blocking',
      '--disable-translate',
      '--no-default-browser-check'
    ]
    if (!isReview) {
      args.unshift('--headless=new')
    }

    const context = await chromium.launchPersistentContext(userDataDir, {
      headless: false,
      args
    })

    // Set the server URL and notify background service worker
    const extensionId = await getExtensionId(context)
    const setupPage = await context.newPage()
    await setupPage.goto(`chrome-extension://${extensionId}/options.html`)

    // Set server URL in storage AND send message to background to apply it immediately
    await setupPage.evaluate((port) => {
      return new Promise((resolve) => {
        const url = `http://127.0.0.1:${port}`
        chrome.storage.local.set({ serverUrl: url }, () => {
          chrome.runtime.sendMessage({ type: 'setServerUrl', url }, () => {
            resolve()
          })
        })
      })
    }, serverPort)

    // Wait for the background to connect to the new server
    await setupPage.waitForTimeout(1000)
    await setupPage.close()

    await use(context)

    await context.close()
    // Clean up user data dir
    fs.rmSync(userDataDir, { recursive: true, force: true })
  },

  /**
   * The extension ID (derived from the loaded extension)
   */
  extensionId: async ({ context }, use) => {
    const id = await getExtensionId(context)
    await use(id)
  },

  /**
   * A page from the context (convenience).
   * In review mode (REVIEW=1), pauses after each test for human inspection.
   */
  page: async ({ context }, use) => {
    const page = await context.newPage()
    await use(page)
    if (process.env.REVIEW) {
      await page.pause()
    }
    await page.close()
  },

  /**
   * Server URL for this test instance
   */
  serverUrl: async ({ serverPort }, use) => {
    await use(`http://127.0.0.1:${serverPort}`)
  }
})

/**
 * Get the extension ID from a browser context with the extension loaded
 */
async function getExtensionId(context) {
  // Navigate to chrome://extensions to find the ID
  let [background] = context.serviceWorkers()
  if (!background) {
    background = await context.waitForEvent('serviceworker')
  }
  const extensionId = background.url().split('/')[2]
  return extensionId
}

export { expect } from '@playwright/test'

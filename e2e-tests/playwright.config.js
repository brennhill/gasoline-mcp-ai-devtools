import { defineConfig } from '@playwright/test'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  testDir: '.',
  testMatch: '*.spec.js',
  timeout: 30000,
  retries: 1,
  workers: 1, // Extensions require sequential execution (shared server)
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    // Chrome extension testing requires a persistent context,
    // configured per-test via the custom fixture in helpers/extension.js
    headless: false, // Extensions don't work in headless mode
    viewport: { width: 1280, height: 720 },
  },
  // Global setup: build the server binary before tests
  globalSetup: path.join(__dirname, 'helpers', 'global-setup.js'),
  globalTeardown: path.join(__dirname, 'helpers', 'global-teardown.js'),
})

/**
 * Global setup: Build the Go server binary before running E2E tests
 */
import { execSync } from 'child_process'
import path from 'path'
import { fileURLToPath } from 'url'
import fs from 'fs'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const projectRoot = path.join(__dirname, '..', '..')

export default async function globalSetup() {
  const binaryPath = path.join(projectRoot, 'dist', 'gasoline')

  // Build the server binary
  console.log('[e2e] Building Go server binary...')
  execSync('make dev', {
    cwd: projectRoot,
    stdio: 'pipe'
  })

  if (!fs.existsSync(binaryPath)) {
    throw new Error(`Server binary not found at ${binaryPath}. Build failed.`)
  }

  console.log('[e2e] Server binary ready.')
}

#!/usr/bin/env node

/**
 * Postinstall script to download the correct binary for the platform
 */

const https = require('https')
const fs = require('fs')
const path = require('path')
const { execSync } = require('child_process')

const VERSION = '5.6.0'
const GITHUB_REPO = 'brennhill/gasoline-mcp-ai-devtools'
const BINARY_NAME = 'gasoline'

// Map Node.js platform/arch to binary names
function getBinaryName() {
  const platform = process.platform
  const arch = process.arch

  const platformMap = {
    darwin: 'darwin',
    linux: 'linux',
    win32: 'win32',
  }

  const archMap = {
    x64: 'x64',
    arm64: 'arm64',
  }

  const p = platformMap[platform]
  const a = archMap[arch]

  if (!p || !a) {
    console.error(`Unsupported platform: ${platform}-${arch}`)
    process.exit(1)
  }

  const ext = platform === 'win32' ? '.exe' : ''
  return `${BINARY_NAME}-${p}-${a}${ext}`
}

// Download file from URL
function download(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest)

    const request = (url) => {
      https
        .get(url, (response) => {
          // Handle redirects
          if (response.statusCode === 301 || response.statusCode === 302) {
            request(response.headers.location)
            return
          }

          if (response.statusCode !== 200) {
            reject(new Error(`Failed to download: ${response.statusCode}`))
            return
          }

          response.pipe(file)
          file.on('finish', () => {
            file.close()
            resolve()
          })
        })
        .on('error', (err) => {
          fs.unlink(dest, () => {})
          reject(err)
        })
    }

    request(url)
  })
}

async function main() {
  const binDir = path.join(__dirname, '..', 'bin')
  const binaryName = getBinaryName()
  const binaryPath = path.join(binDir, binaryName)
  const shimPath = path.join(binDir, BINARY_NAME)

  // Ensure bin directory exists
  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true })
  }

  // Download URL
  const downloadUrl = `https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/${binaryName}`

  console.log(`Downloading gasoline binary for ${process.platform}-${process.arch}...`)
  console.log(`URL: ${downloadUrl}`)

  try {
    await download(downloadUrl, binaryPath)
  } catch (err) {
    // If download fails, check if we're in development (binary might be local)
    console.warn(`Download failed: ${err.message}`)
    console.warn('Checking for local binary...')

    // Look for local build
    const localBinary = path.join(__dirname, '..', '..', 'dist', binaryName)
    if (fs.existsSync(localBinary)) {
      console.log('Using local binary from dist/')
      fs.copyFileSync(localBinary, binaryPath)
    } else {
      console.error('')
      console.error('╔════════════════════════════════════════════════════════════════╗')
      console.error('║  GASOLINE BINARY NOT AVAILABLE                                 ║')
      console.error('╚════════════════════════════════════════════════════════════════╝')
      console.error('')
      console.error('No pre-built binary found for your platform.')
      console.error('')
      console.error('OPTION 1: Build from source (requires Go 1.21+)')
      console.error('  git clone https://github.com/brennhill/gasoline-mcp-ai-devtools.git')
      console.error('  cd gasoline')
      console.error('  go build -o /usr/local/bin/gasoline ./cmd/dev-console')
      console.error('')
      console.error('OPTION 2: Run directly with Go')
      console.error('  go run ./cmd/dev-console')
      console.error('')
      console.error('OPTION 3: Use MCP config with Go (Claude Code / Cursor)')
      console.error('  Add to .mcp.json:')
      console.error('  {"mcpServers":{"gasoline":{"command":"go","args":["run","./cmd/dev-console"]}}}')
      console.error('')
      console.error('For help: https://github.com/brennhill/gasoline-mcp-ai-devtools#quick-start')
      console.error('')
      process.exit(1)
    }
  }

  // Make binary executable (Unix only)
  if (process.platform !== 'win32') {
    fs.chmodSync(binaryPath, 0o755)
  }

  // Create shim script that runs the binary
  const shimContent =
    process.platform === 'win32'
      ? `@echo off\n"%~dp0${binaryName}" %*`
      : `#!/bin/sh\nexec "$(dirname "$0")/${binaryName}" "$@"`

  const shimExt = process.platform === 'win32' ? '.cmd' : ''
  fs.writeFileSync(shimPath + shimExt, shimContent)

  if (process.platform !== 'win32') {
    fs.chmodSync(shimPath, 0o755)
  }

  console.log('gasoline installed successfully!')
}

main().catch((err) => {
  console.error('Installation failed:', err.message)
  process.exit(1)
})

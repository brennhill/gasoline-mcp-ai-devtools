#!/usr/bin/env node

/**
 * Postinstall script to download the correct binary for the platform
 * Also handles cleanup of old gasoline processes for clean upgrades
 */

const https = require('https')
const http = require('http')
const fs = require('fs')
const path = require('path')
const { execSync, spawnSync, spawn } = require('child_process')

const VERSION = '6.0.0'
const GITHUB_REPO = 'brennhill/gasoline-mcp-ai-devtools'
const BINARY_NAME = 'gasoline'

/**
 * Kill all running gasoline processes to ensure clean upgrade
 */
function cleanupOldProcesses() {
  if (process.platform === 'win32') {
    // Windows: Find and kill gasoline processes
    try {
      const result = spawnSync('tasklist', ['/FI', 'IMAGENAME eq gasoline*', '/FO', 'CSV'], {
        encoding: 'utf8',
        windowsHide: true
      })
      if (result.stdout) {
        const lines = result.stdout.split('\n').slice(1) // Skip header
        for (const line of lines) {
          const match = line.match(/"gasoline[^"]*","(\d+)"/)
          if (match) {
            const pid = match[1]
            spawnSync('taskkill', ['/F', '/PID', pid], { windowsHide: true })
            console.log(`Killed old gasoline process (PID: ${pid})`)
          }
        }
      }
    } catch (e) {
      // Ignore errors - process might not exist
    }
  } else {
    // Unix: Find and kill gasoline processes by name
    try {
      // Method 1: pkill by name (most reliable)
      const pkillResult = spawnSync('pkill', ['-f', 'gasoline'], {
        encoding: 'utf8'
      })

      // Method 2: Also check for processes on common ports (7890, 17890)
      const ports = ['7890', '17890']
      for (const port of ports) {
        const lsofResult = spawnSync('lsof', ['-ti', `:${port}`], {
          encoding: 'utf8'
        })
        if (lsofResult.stdout && lsofResult.stdout.trim()) {
          const pids = lsofResult.stdout.trim().split('\n')
          for (const pid of pids) {
            if (pid) {
              spawnSync('kill', ['-9', pid])
              console.log(`Killed process on port ${port} (PID: ${pid})`)
            }
          }
        }
      }
    } catch (e) {
      // Ignore errors - processes might not exist
    }
  }
}

/**
 * Verify the installed version matches expected
 */
function verifyVersion(binaryPath) {
  try {
    const result = spawnSync(binaryPath, ['--version'], {
      encoding: 'utf8',
      timeout: 5000
    })
    if (result.stdout) {
      const version = result.stdout.trim()
      if (version.includes(VERSION)) {
        console.log(`✓ Verified gasoline version: ${version}`)
        return true
      } else {
        console.warn(`Warning: Expected version ${VERSION}, got: ${version}`)
        return false
      }
    }
  } catch (e) {
    console.warn(`Could not verify version: ${e.message}`)
  }
  return false
}

/**
 * Start the gasoline server in the background
 * Returns true if server started successfully
 */
function autoStartServer(binaryPath, port = 7890) {
  return new Promise((resolve) => {
    console.log(`Starting gasoline server on port ${port}...`)

    // Check if port is already in use
    const testServer = http.createServer()
    testServer.once('error', (err) => {
      if (err.code === 'EADDRINUSE') {
        console.log(`Port ${port} already in use - server may already be running`)
        resolve(true) // Consider this success - something is already there
      } else {
        console.warn(`Port check failed: ${err.message}`)
        resolve(false)
      }
    })

    testServer.once('listening', () => {
      testServer.close(() => {
        // Port is free, start the server
        try {
          // Spawn detached process that survives npm exit
          const child = spawn(binaryPath, ['--port', String(port)], {
            detached: true,
            stdio: ['pipe', 'ignore', 'ignore'], // pipe stdin to keep it open
            windowsHide: true
          })

          // Unref so npm can exit
          child.unref()

          // Wait for server to be ready
          let attempts = 0
          const maxAttempts = 30 // 3 seconds
          const checkHealth = () => {
            attempts++
            const req = http.request({
              hostname: '127.0.0.1',
              port: port,
              path: '/health',
              method: 'GET',
              timeout: 200
            }, (res) => {
              if (res.statusCode === 200) {
                console.log(`✓ Server started on http://127.0.0.1:${port}`)
                resolve(true)
              } else if (attempts < maxAttempts) {
                setTimeout(checkHealth, 100)
              } else {
                console.warn('Server started but health check failed')
                resolve(false)
              }
            })

            req.on('error', () => {
              if (attempts < maxAttempts) {
                setTimeout(checkHealth, 100)
              } else {
                console.warn('Server failed to respond within 3 seconds')
                resolve(false)
              }
            })

            req.end()
          }

          // Start health checking after a brief delay
          setTimeout(checkHealth, 100)

        } catch (e) {
          console.warn(`Failed to start server: ${e.message}`)
          resolve(false)
        }
      })
    })

    testServer.listen(port, '127.0.0.1')
  })
}

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

  // Clean up any old gasoline processes before installing new version
  console.log('Cleaning up old gasoline processes...')
  cleanupOldProcesses()

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

  // Verify the installed version
  verifyVersion(binaryPath)

  // Auto-start the server so extension reconnects immediately
  await autoStartServer(binaryPath)

  console.log('gasoline installed successfully!')
}

main().catch((err) => {
  console.error('Installation failed:', err.message)
  process.exit(1)
})

// @ts-nocheck
/**
 * @fileoverview test-server.mjs — HTTP test server for network body E2E tests.
 * Provides endpoints to test large bodies, binary content, header sanitization,
 * streaming responses, and various HTTP methods. Uses Node.js http module only.
 */

import http from 'node:http'

const PORT = process.env.TEST_SERVER_PORT || 19891

/**
 * Generate a large JSON response (~1MB)
 * @returns {string} Large JSON string
 */
function generateLargeJson() {
  const items = []
  const itemCount = 5000
  for (let i = 0; i < itemCount; i++) {
    items.push({
      id: i,
      name: `Item ${i}`,
      description: 'x'.repeat(180), // ~200 bytes per item
    })
  }
  return JSON.stringify({ data: items })
}

/**
 * Generate a large text response (~1MB)
 * @returns {string} Large text string
 */
function generateLargeText() {
  const line = 'Lorem ipsum dolor sit amet, consectetur adipiscing elit. '.repeat(10)
  const lines = []
  for (let i = 0; i < 5000; i++) {
    lines.push(`Line ${i}: ${line}`)
  }
  return lines.join('\n')
}

/**
 * Generate binary data (PNG-like header + random bytes)
 * @param {number} size - Size in bytes
 * @returns {Buffer} Binary data
 */
function generateBinaryData(size = 4096) {
  const buffer = Buffer.alloc(size)
  // PNG magic bytes
  buffer[0] = 0x89
  buffer[1] = 0x50
  buffer[2] = 0x4e
  buffer[3] = 0x47
  buffer[4] = 0x0d
  buffer[5] = 0x0a
  buffer[6] = 0x1a
  buffer[7] = 0x0a
  // Fill rest with pattern
  for (let i = 8; i < size; i++) {
    buffer[i] = i % 256
  }
  return buffer
}

// Pre-generate large responses (avoid regenerating on each request)
const largeJson = generateLargeJson()
const largeText = generateLargeText()
const binaryData = generateBinaryData(4096)

/**
 * Request handler
 * @param {http.IncomingMessage} req
 * @param {http.ServerResponse} res
 */
function handleRequest(req, res) {
  const url = new URL(req.url, `http://localhost:${PORT}`)
  const pathname = url.pathname

  // CORS headers for browser tests
  res.setHeader('Access-Control-Allow-Origin', '*')
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS')
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization, X-API-Key, X-Custom-Header')

  // Handle preflight
  if (req.method === 'OPTIONS') {
    res.writeHead(204)
    res.end()
    return
  }

  // Route handlers
  switch (pathname) {
    case '/health':
      res.writeHead(200, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({ status: 'ok' }))
      break

    case '/large-json':
      // ~1MB JSON response
      res.writeHead(200, {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(largeJson),
      })
      res.end(largeJson)
      break

    case '/large-text':
      // ~1MB text response
      res.writeHead(200, {
        'Content-Type': 'text/plain',
        'Content-Length': Buffer.byteLength(largeText),
      })
      res.end(largeText)
      break

    case '/echo':
      // Echo request body back
      {
        let body = ''
        req.on('data', (chunk) => {
          body += chunk.toString()
        })
        req.on('end', () => {
          res.writeHead(200, { 'Content-Type': req.headers['content-type'] || 'text/plain' })
          res.end(body)
        })
      }
      break

    case '/binary':
      // Binary data (PNG-like)
      res.writeHead(200, {
        'Content-Type': 'image/png',
        'Content-Length': binaryData.length,
      })
      res.end(binaryData)
      break

    case '/streaming': {
      // Chunked transfer encoding
      res.writeHead(200, {
        'Content-Type': 'text/plain',
        'Transfer-Encoding': 'chunked',
      })
      let chunkCount = 0
      const interval = setInterval(() => {
        if (chunkCount >= 5) {
          clearInterval(interval)
          res.end()
          return
        }
        res.write(`Chunk ${chunkCount}\n`)
        chunkCount++
      }, 10)
      break
    }

    case '/auth-header':
      // Return all headers (to verify sanitization on client side)
      res.writeHead(200, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({ receivedHeaders: req.headers }))
      break

    case '/slow':
      // 3 second delay
      setTimeout(() => {
        res.writeHead(200, { 'Content-Type': 'application/json' })
        res.end(JSON.stringify({ delayed: true, delayMs: 3000 }))
      }, 3000)
      break

    case '/error-400':
      res.writeHead(400, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({ error: 'Bad Request', message: 'Invalid parameters' }))
      break

    case '/error-404':
      res.writeHead(404, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({ error: 'Not Found', message: 'Resource not found' }))
      break

    case '/error-500':
      res.writeHead(500, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({ error: 'Internal Server Error', message: 'Server error occurred' }))
      break

    case '/json':
      // Simple JSON endpoint
      res.writeHead(200, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({ id: 1, name: 'Test', timestamp: Date.now() }))
      break

    case '/html':
      // HTML response
      res.writeHead(200, { 'Content-Type': 'text/html' })
      res.end('<html><body><h1>Test Page</h1></body></html>')
      break

    default:
      res.writeHead(404, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({ error: 'Not Found' }))
  }
}

const server = http.createServer(handleRequest)

server.listen(PORT, () => {
  console.log(`Test server running on http://localhost:${PORT}`)
  console.log('Available endpoints:')
  console.log('  GET  /health       - Health check')
  console.log('  GET  /large-json   - 1MB JSON response')
  console.log('  GET  /large-text   - 1MB text response')
  console.log('  POST /echo         - Echo request body')
  console.log('  GET  /binary       - Binary data (PNG)')
  console.log('  GET  /streaming    - Chunked response')
  console.log('  GET  /auth-header  - Returns received headers')
  console.log('  GET  /slow         - 3 second delay')
  console.log('  GET  /error-400    - 400 Bad Request')
  console.log('  GET  /error-404    - 404 Not Found')
  console.log('  GET  /error-500    - 500 Internal Server Error')
  console.log('  GET  /json         - Simple JSON')
  console.log('  GET  /html         - HTML response')
})

// Graceful shutdown
process.on('SIGTERM', () => {
  console.log('Shutting down test server...')
  server.close(() => {
    process.exit(0)
  })
})

process.on('SIGINT', () => {
  console.log('Shutting down test server...')
  server.close(() => {
    process.exit(0)
  })
})

export { server, PORT }

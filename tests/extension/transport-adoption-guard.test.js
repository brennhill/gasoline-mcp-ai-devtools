// @ts-nocheck
/**
 * @fileoverview transport-adoption-guard.test.js — Prevent direct server fetch
 * calls from non-provider background modules.
 */

import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

function readBackgroundFile(relativePath) {
  const full = path.join(__dirname, '../../extension/background', relativePath)
  return readFileSync(full, 'utf8')
}

describe('Transport provider adoption guards', () => {
  test('communication.js does not call fetch to server endpoints directly', () => {
    const src = readBackgroundFile('communication.js')
    assert.ok(
      !src.includes('fetch(`${serverUrl}/'),
      'communication.js must route server calls through ExtensionTransportProvider'
    )
  })

  test('server.js does not call fetch to server endpoints directly', () => {
    const src = readBackgroundFile('server.js')
    assert.ok(
      !src.includes('fetch(`${serverUrl}') && !src.includes('fetch(`${serverUrl}/'),
      'server.js must delegate to ExtensionTransportProvider methods'
    )
  })

  test('sync-client.js does not call fetch directly for /sync', () => {
    const src = readBackgroundFile('sync-client.js')
    assert.ok(
      !src.includes('fetch(`${this.serverUrl}/sync`'),
      'sync-client.js must route /sync through ExtensionTransportProvider'
    )
  })

  test('draw-mode-handler.js does not call fetch to server endpoints directly', () => {
    const src = readBackgroundFile('draw-mode-handler.js')
    assert.ok(
      !src.includes('fetch(`${serverUrl}/'),
      'draw-mode-handler.js must route server calls through ExtensionTransportProvider'
    )
  })

  test('message-handlers.js does not call fetch to server endpoints directly', () => {
    const src = readBackgroundFile('message-handlers.js')
    assert.ok(
      !src.includes('fetch(`${serverUrl}/'),
      'message-handlers.js must route server calls through ExtensionTransportProvider'
    )
  })

  test('upload-handler.js does not call fetch to server endpoints directly', () => {
    const src = readBackgroundFile('upload-handler.js')
    const hasDirectFetch =
      src.includes('fetch(`${serverUrl}/') ||
      src.includes('fetch(`${getServerUrl()}/')
    assert.ok(
      !hasDirectFetch,
      'upload-handler.js must route server calls through ExtensionTransportProvider'
    )
  })
})

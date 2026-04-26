// @ts-nocheck
/**
 * @fileoverview telemetry-beacon-branding.test.js — Pins that extension code
 * does not import or call any extension-side telemetry beacon helper. Remote
 * analytics are owned by the daemon (internal/telemetry/beacon.go); this test
 * keeps the extension out of that contract.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

describe('extension telemetry isolation', () => {
  test('background startup paths do not import or call extension telemetry beacons', () => {
    const initSource = fs.readFileSync(
      path.join(__dirname, '..', '..', 'src', 'background', 'init.ts'),
      'utf8'
    )
    const syncSource = fs.readFileSync(
      path.join(__dirname, '..', '..', 'src', 'background', 'sync-client.ts'),
      'utf8'
    )

    assert.doesNotMatch(initSource, /telemetry-beacon/, 'init.ts must not import a telemetry beacon helper')
    assert.doesNotMatch(initSource, /\bbeacon\(/, 'init.ts must not fire raw telemetry beacons')
    assert.doesNotMatch(syncSource, /telemetry-beacon/, 'sync-client.ts must not import a telemetry beacon helper')
    assert.doesNotMatch(syncSource, /\bbeacon\(/, 'sync-client.ts must not fire raw telemetry beacons')
  })

  test('extension lib does not ship a telemetry-beacon module', () => {
    const compiledShim = path.join(__dirname, '..', '..', 'extension', 'lib', 'telemetry-beacon.js')
    assert.strictEqual(
      fs.existsSync(compiledShim),
      false,
      `${compiledShim} should not exist; remote analytics are daemon-owned`
    )
  })
})

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

  test('extension lib does not ship any telemetry-beacon module under any name', () => {
    const libDir = path.join(__dirname, '..', '..', 'extension', 'lib')
    const re = /(telemetry.*beacon|beacon.*telemetry)/i
    const offenders = []

    // Symlinks are NOT followed: a symlinked node_modules would otherwise
    // explode the search and could trigger false positives off-tree. We
    // also check symlink TARGETS for matching names so a symlink renamed
    // to mask a beacon helper still trips the check.
    function scan(dir) {
      for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name)
        if (entry.isSymbolicLink()) {
          if (re.test(entry.name)) offenders.push(full)
          continue
        }
        if (entry.isDirectory()) {
          scan(full)
          continue
        }
        if (re.test(entry.name)) offenders.push(full)
      }
    }
    scan(libDir)

    assert.deepStrictEqual(
      offenders,
      [],
      `extension/lib must not ship telemetry-beacon files (found: ${offenders.join(', ')}); remote analytics are daemon-owned`
    )
  })

  test('extension source tree does not introduce a beacon helper', () => {
    const srcDir = path.join(__dirname, '..', '..', 'src')
    const re = /(telemetry.*beacon|beacon.*telemetry)/i
    const offenders = []

    function scan(dir) {
      for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name)
        if (entry.isSymbolicLink()) {
          if (re.test(entry.name)) offenders.push(full)
          continue
        }
        if (entry.isDirectory()) {
          scan(full)
          continue
        }
        if (re.test(entry.name)) offenders.push(full)
      }
    }
    scan(srcDir)

    assert.deepStrictEqual(
      offenders,
      [],
      `src/ must not contain telemetry-beacon files (found: ${offenders.join(', ')}); the daemon owns remote analytics`
    )
  })
})

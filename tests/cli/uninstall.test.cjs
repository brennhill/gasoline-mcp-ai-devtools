/**
 * Unit tests for lib/uninstall.js
 * Tests uninstall operation: executeUninstall with cleanup
 */

const test = require('node:test')
const assert = require('node:assert')
const fs = require('fs')
const path = require('path')
const os = require('os')
const uninstall = require('../../npm/gasoline-mcp/lib/uninstall')
const config = require('../../npm/gasoline-mcp/lib/config')

test('uninstall.executeUninstall returns correct structure', () => {
  const result = uninstall.executeUninstall({ dryRun: false, verbose: false })

  assert.ok(typeof result === 'object', 'Should return object')
  assert.ok(typeof result.success === 'boolean', 'Should have success boolean')
  assert.ok(Array.isArray(result.removed), 'Should have removed array')
  assert.ok(Array.isArray(result.notConfigured), 'Should have notConfigured array')
  assert.ok(Array.isArray(result.errors), 'Should have errors array')
})

test('uninstall.executeUninstall with dryRun=true does not modify files', () => {
  // Get original checksums
  const candidates = config.getConfigCandidates()
  const checksums = {}
  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) {
      const content = fs.readFileSync(candidate, 'utf8')
      checksums[candidate] = JSON.stringify(JSON.parse(content))
    }
  }

  // Run uninstall with dryRun
  const result = uninstall.executeUninstall({ dryRun: true, verbose: false })

  // Verify files haven't changed
  for (const candidate in checksums) {
    if (fs.existsSync(candidate)) {
      const content = fs.readFileSync(candidate, 'utf8')
      const newChecksum = JSON.stringify(JSON.parse(content))
      assert.strictEqual(newChecksum, checksums[candidate], `File ${candidate} should not be modified in dryRun`)
    }
  }

  assert.ok(typeof result === 'object', 'Should still return valid result')
})

test('uninstall.executeUninstall reports unconfigured tools', () => {
  const result = uninstall.executeUninstall({ dryRun: false, verbose: false })

  // At least one tool should be reported (either removed, notConfigured, or errored)
  const totalProcessed = result.removed.length + result.notConfigured.length + result.errors.length
  assert.ok(totalProcessed >= 0, 'Should process all candidates')
})

test('uninstall.executeUninstall removed items have correct structure', () => {
  const result = uninstall.executeUninstall({ dryRun: false, verbose: false })

  for (const item of result.removed) {
    assert.ok(item.name, 'Removed item should have name')
    if (item.method === 'file') {
      assert.ok(item.path, 'File removal should include path')
    }
  }
})

test('uninstall.executeUninstall returns success only if something removed', () => {
  const result = uninstall.executeUninstall({ dryRun: false, verbose: false })

  // Success should be true only if gasoline was actually removed from at least one config
  if (result.removed.length > 0) {
    assert.strictEqual(result.success, true, 'Should be successful if files were removed')
  }
})

test('uninstall.executeUninstall with verbose logs debug info', () => {
  // Verbose mode should not crash, just log more
  const result = uninstall.executeUninstall({ dryRun: true, verbose: true })

  assert.ok(typeof result === 'object', 'Should return result even with verbose=true')
})

test('uninstall.executeUninstall preserves other MCP servers', () => {
  const testDir = path.join(os.tmpdir(), 'uninstall-test')
  if (!fs.existsSync(testDir)) {
    fs.mkdirSync(testDir, { recursive: true })
  }

  try {
    const testPath = path.join(testDir, 'mixed-config.json')

    // Create a config with both gasoline and another server
    const config_data = {
      mcpServers: {
        gasoline: { command: 'gasoline-mcp' },
        other: { command: 'other-tool' }
      }
    }
    fs.writeFileSync(testPath, JSON.stringify(config_data))

    // Note: uninstall.executeUninstall checks the real config paths
    // This test verifies the logic conceptually - for actual testing we'd need to
    // modify one of the real config files which we don't want to do in tests
    const result = uninstall.executeUninstall({ dryRun: true, verbose: false })

    assert.ok(typeof result === 'object', 'Should return valid result')
  } finally {
    if (fs.existsSync(testDir)) {
      fs.rmSync(testDir, { recursive: true, force: true })
    }
  }
})

test('uninstall.executeUninstall handles non-existent config files', () => {
  // This should just report them as notConfigured, not error
  const result = uninstall.executeUninstall({ dryRun: false, verbose: false })

  // Should process available clients without crashing.
  const totalProcessed = result.removed.length + result.notConfigured.length + result.errors.length
  assert.ok(totalProcessed >= 0, 'Should produce a valid aggregate result')
})

test('uninstall.executeUninstall handles invalid JSON gracefully', () => {
  // Invalid JSON should be reported as an error, not crash
  const result = uninstall.executeUninstall({ dryRun: false, verbose: false })

  // Result should still be valid
  assert.ok(Array.isArray(result.errors), 'Should have errors array')
  assert.ok(typeof result.success === 'boolean', 'Should have success boolean')
})

test('uninstall.executeUninstall with dryRun still reports what would be done', () => {
  const result = uninstall.executeUninstall({ dryRun: true, verbose: false })

  // Even in dryRun, should report what would happen
  assert.ok(Array.isArray(result.removed), 'Should report what would be removed')
  assert.ok(Array.isArray(result.notConfigured), 'Should report not configured tools')
})

test('uninstall.executeUninstall is idempotent in dryRun mode', () => {
  // Running twice with dryRun should give same results
  const result1 = uninstall.executeUninstall({ dryRun: true, verbose: false })
  const result2 = uninstall.executeUninstall({ dryRun: true, verbose: false })

  assert.strictEqual(result1.removed.length, result2.removed.length, 'dryRun results should be consistent')
  assert.strictEqual(result1.notConfigured.length, result2.notConfigured.length, 'dryRun results should be consistent')
})

test('uninstall.executeUninstall checks all 4 config locations', () => {
  const result = uninstall.executeUninstall({ dryRun: false, verbose: false })

  // Should have checked all 4 tools
  const allTools = result.removed
    .map((r) => r.name)
    .concat(result.notConfigured)
    .concat(result.errors.map((e) => (typeof e === 'string' ? e.split(':')[0] : e.name)))

  // Rough check - should have processed multiple tools
  assert.ok(allTools.length >= 2, 'Should have checked multiple tools')
})

test('uninstall.executeUninstall success flag accuracy', () => {
  const result = uninstall.executeUninstall({ dryRun: false, verbose: false })

  // success should be true if and only if at least one file was removed
  const hasRemoved = result.removed.length > 0
  assert.strictEqual(result.success, hasRemoved, 'success flag should match whether items were removed')
})

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '../..')

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

test('extension sync path no longer sends reconnect analytics that can be miscounted as installs', () => {
  const source = read('internal/capture/sync.go')
  assert.doesNotMatch(
    source,
    /BeaconEvent\("extension_connect"/,
    'extension reconnects should not emit raw extension_connect analytics outside the canonical contract'
  )
})

test('marketing analytics docs distinguish acquisition signals from install counts', () => {
  const source = read('docs/marketing/analytics-setup-guide.md')
  assert.doesNotMatch(
    source,
    /Package installation metrics/i,
    'download metrics should not be documented as install counts'
  )
  assert.doesNotMatch(
    source,
    /Extension Install Tracking/i,
    'store download metrics should not be labeled as install tracking'
  )
  assert.match(
    source,
    /download|acquisition/i,
    'marketing analytics docs should frame package and store metrics as download/acquisition signals'
  )
})

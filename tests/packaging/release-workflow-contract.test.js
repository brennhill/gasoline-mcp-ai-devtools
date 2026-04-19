/**
 * @fileoverview Regression guard for release workflow asset contract and SARIF distribution boundaries.
 */

import { test } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const REPO_ROOT = path.resolve(__dirname, '../..')

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

function extractReleaseAssetList(workflow) {
  const match = workflow.match(/files:\s*\|\n((?:\s{12}\S.*\n)+)\s+fail_on_unmatched_files:/)
  assert.ok(match, 'release workflow should define a files block before fail_on_unmatched_files')
  return match[1]
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
}

test('release workflow fails closed and only uploads public release assets', () => {
  const workflow = read('.github/workflows/release.yml')

  assert.match(
    workflow,
    /fail_on_unmatched_files:\s*true/,
    'release workflow should fail if any expected release artifact is missing'
  )
  assert.doesNotMatch(
    workflow,
    /dist\/\*\.sarif/,
    'SARIF reports should not be published as public GitHub Release assets'
  )

  const expectedAssets = [
    'dist/kaboom-agentic-browser-darwin-arm64',
    'dist/kaboom-agentic-browser-darwin-x64',
    'dist/kaboom-agentic-browser-linux-arm64',
    'dist/kaboom-agentic-browser-linux-x64',
    'dist/kaboom-agentic-browser-win32-x64.exe',
    'dist/kaboom-hooks-darwin-arm64',
    'dist/kaboom-hooks-darwin-x64',
    'dist/kaboom-hooks-linux-arm64',
    'dist/kaboom-hooks-linux-x64',
    'dist/kaboom-hooks-win32-x64.exe',
    'dist/kaboom-extension-v*.zip',
    'dist/checksums.txt'
  ]

  const actualAssets = extractReleaseAssetList(workflow)
  assert.deepStrictEqual(
    actualAssets,
    expectedAssets,
    'release workflow should publish exactly the documented public asset set'
  )
})

test('release docs define the public asset contract and keep SARIF on the code-scanning path', () => {
  const releaseDoc = read('docs/core/release.md')

  assert.match(releaseDoc, /GitHub Release asset contract/i, 'release doc should define the public asset set')
  assert.match(releaseDoc, /checksums\.txt/, 'release doc should describe checksum publication')
  assert.match(
    releaseDoc,
    /SARIF reports are not GitHub Release assets/i,
    'release doc should explicitly keep SARIF out of the public release bundle'
  )
  assert.match(
    releaseDoc,
    /GitHub Code Scanning/i,
    'release doc should point SARIF uploads at GitHub Code Scanning'
  )
})

test('release docs and canonical flow map cross-reference the release asset contract', () => {
  const releaseDoc = read('docs/core/release.md')
  const flowMap = read('docs/architecture/flow-maps/release-asset-contract-and-sarif-distribution.md')
  const flowMapIndex = read('docs/architecture/flow-maps/README.md')

  assert.match(
    releaseDoc,
    /release-asset-contract-and-sarif-distribution\.md/,
    'release doc should link to the canonical release asset flow map'
  )
  assert.match(flowMap, /docs\/core\/release\.md/, 'flow map should link back to the release doc')
  assert.match(
    flowMap,
    /\.github\/workflows\/release\.yml/,
    'flow map should anchor the release workflow as a code path'
  )
  assert.match(
    flowMap,
    /tests\/packaging\/release-workflow-contract\.test\.js/,
    'flow map should anchor the regression test path'
  )
  assert.match(
    flowMapIndex,
    /Release Asset Contract and SARIF Distribution/,
    'flow map index should list the canonical release asset flow map'
  )
})

// @ts-nocheck
/**
 * @fileoverview gokaboom-domain-contract.test.js — Guards site metadata/domain contracts.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const TEST_DIR = path.dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = path.resolve(TEST_DIR, '../..')

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

describe('gokaboom domain contracts', () => {
  test('site metadata generators emit gokaboom.dev and Kaboom branding', () => {
    const astroConfig = read('gokaboom.dev/astro.config.mjs')
    const indexMarkdown = read('gokaboom.dev/src/pages/index.md.ts')
    const slugMarkdown = read('gokaboom.dev/src/pages/[...slug].md.ts')
    const nestedMarkdown = read('gokaboom.dev/src/pages/markdown/[...slug].md.ts')

    assert.match(astroConfig, /site:\s*'https:\/\/gokaboom\.dev'/)
    assert.match(astroConfig, /title:\s*'Kaboom'/)
    assert.match(astroConfig, /alt:\s*'Kaboom'/)
    assert.doesNotMatch(astroConfig, /cookwithgasoline\.com|STRUM Agentic Devtools/)

    assert.match(indexMarkdown, /canonical: https:\/\/gokaboom\.dev\//)
    assert.match(indexMarkdown, /'Kaboom MCP'/)
    assert.doesNotMatch(indexMarkdown, /cookwithgasoline\.com|STRUM MCP/)

    assert.match(slugMarkdown, /canonical: https:\/\/gokaboom\.dev\$\{canonicalPath\}/)
    assert.match(slugMarkdown, /'Kaboom MCP'/)
    assert.doesNotMatch(slugMarkdown, /cookwithgasoline\.com|STRUM MCP/)

    assert.match(nestedMarkdown, /canonical: https:\/\/gokaboom\.dev/)
    assert.match(nestedMarkdown, /'Kaboom MCP'/)
    assert.doesNotMatch(nestedMarkdown, /cookwithgasoline\.com|STRUM MCP/)
  })

  test('content contract tooling uses gokaboom naming', () => {
    const newScriptPath = path.join(REPO_ROOT, 'scripts/docs/check-gokaboom-content-contract.mjs')
    const oldScriptPath = path.join(REPO_ROOT, 'scripts/docs/check-cookwithgasoline-content-contract.mjs')

    assert.ok(fs.existsSync(newScriptPath))
    assert.ok(!fs.existsSync(oldScriptPath))

    const packageJson = read('package.json')
    const scriptSource = read('scripts/docs/check-gokaboom-content-contract.mjs')

    assert.match(packageJson, /check-gokaboom-content-contract\.mjs/)
    assert.doesNotMatch(packageJson, /check-cookwithgasoline-content-contract\.mjs/)
    assert.match(scriptSource, /gokaboom\.dev/)
    assert.doesNotMatch(scriptSource, /cookwithgasoline\.com/)
  })
})

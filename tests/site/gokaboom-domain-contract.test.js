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
  test('site metadata generators emit gokaboom.dev and KaBOOM branding', () => {
    const astroConfig = read('gokaboom.dev/astro.config.mjs')
    const indexMarkdown = read('gokaboom.dev/src/pages/index.md.ts')
    const slugMarkdown = read('gokaboom.dev/src/pages/[...slug].md.ts')
    const nestedMarkdown = read('gokaboom.dev/src/pages/markdown/[...slug].md.ts')

    assert.match(astroConfig, /site:\s*'https:\/\/gokaboom\.dev'/)
    assert.match(astroConfig, /title:\s*'KaBOOM'/)
    assert.match(astroConfig, /alt:\s*'KaBOOM'/)
    assert.doesNotMatch(astroConfig, /cookwithgasoline\.com|STRUM Agentic Devtools/)

    assert.match(indexMarkdown, /canonical: https:\/\/gokaboom\.dev\//)
    assert.match(indexMarkdown, /'KaBOOM MCP'/)
    assert.doesNotMatch(indexMarkdown, /cookwithgasoline\.com|STRUM MCP/)

    assert.match(slugMarkdown, /canonical: https:\/\/gokaboom\.dev\$\{canonicalPath\}/)
    assert.match(slugMarkdown, /'KaBOOM MCP'/)
    assert.doesNotMatch(slugMarkdown, /cookwithgasoline\.com|STRUM MCP/)

    assert.match(nestedMarkdown, /canonical: https:\/\/gokaboom\.dev/)
    assert.match(nestedMarkdown, /'KaBOOM MCP'/)
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

  test('public seo metadata uses KaBOOM and gokaboom.dev', () => {
    const seoMeta = read('gokaboom.dev/public/seo-meta.html')
    const robots = read('gokaboom.dev/public/robots.txt')

    assert.match(seoMeta, /KaBOOM/)
    assert.match(seoMeta, /https:\/\/gokaboom\.dev\//)
    assert.doesNotMatch(seoMeta, /STRUM|Strum|Gasoline|cookwithgasoline|getstrum/)

    assert.match(robots, /Kaboom/)
    assert.doesNotMatch(robots, /STRUM|Strum|Gasoline|cookwithgasoline|getstrum/)
  })

  test('site install docs batch 1 uses Kaboom naming and gokaboom.dev contracts', () => {
    const files = [
      'gokaboom.dev/src/content/docs/agent-install-guide.md',
      'gokaboom.dev/src/content/docs/downloads.md',
      'gokaboom.dev/src/content/docs/getting-started.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /Kaboom|kaboom-agentic-browser|gokaboom\.dev/)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-agentic-browser|~\/\.gasoline|GasolineAgenticDevtoolExtension/
      )
    }
  })

  test('site docs batch 2 removes legacy brand copy', () => {
    const files = [
      'gokaboom.dev/src/content/docs/alternatives.md',
      'gokaboom.dev/src/content/docs/architecture.md',
      'gokaboom.dev/src/content/docs/discover-workflows.mdx',
      'gokaboom.dev/src/content/docs/execute-scripts.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /Kaboom|kaboom-agentic-browser|gokaboom\.dev/)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-agentic-browser|How\.gasoline|\.gasoline/
      )
    }
  })

  test('site docs batch 3 removes legacy brand copy', () => {
    const files = [
      'gokaboom.dev/src/content/docs/features.md',
      'gokaboom.dev/src/content/docs/privacy.md',
      'gokaboom.dev/src/content/docs/security.md',
      'gokaboom.dev/src/content/docs/troubleshooting.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /Kaboom|kaboom-agentic-browser|gokaboom\.dev|kaboom-mcp/)
      assert.doesNotMatch(
        source,
        /STRUM|Strum|Gasoline|cookwithgasoline|getstrum|gasoline-agentic-browser|gasoline-agentic-devtools|\.gasoline|~\/\.gasoline|~\/\.strum|STRUM_TELEMETRY|usestrum/
      )
    }
  })
})

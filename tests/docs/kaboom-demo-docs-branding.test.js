// @ts-nocheck
/**
 * @fileoverview kaboom-demo-docs-branding.test.js — Guards demo docs branding.
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

describe('kaboom demo docs branding', () => {
  test('demo docs use Kaboom naming and gokaboom paths', () => {
    const files = [
      'docs/demos/demo-strategy.md',
      'docs/demos/demo-scenarios.md',
      'docs/demos/feature-power-ranking.md',
      'docs/demos/full-stack-lab-llm-script.md',
      'docs/demos/shopbroken-spec.md',
      'docs/demos/recording-web-demos/README.md',
      'docs/demos/replay-from-natural-language/README.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /Kaboom|KaBOOM|gokaboom|kaboom/i)
      assert.doesNotMatch(
        source,
        /Gasoline|STRUM|getstrum|cookwithgasoline|gasoline-mcp|gasoline-agentic-browser|\.gasoline|\bgasoline\b/
      )
    }
  })
})

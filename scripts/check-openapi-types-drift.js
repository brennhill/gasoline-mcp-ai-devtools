#!/usr/bin/env node
// check-openapi-types-drift.js — Regenerates src/generated/openapi-types.ts
// into a temp file and diffs against the committed file. Fails if they differ,
// which means someone edited openapi.json without running
// `npm run generate:openapi-types`.
//
// Usage: node scripts/check-openapi-types-drift.js
// Exit codes: 0 = up-to-date, 1 = drift.

import { execSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const REPO_ROOT = path.resolve(__dirname, '..')

const SPEC = path.join(REPO_ROOT, 'cmd/browser-agent/openapi.json')
const CURRENT = path.join(REPO_ROOT, 'src/generated/openapi-types.ts')

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'openapi-types-drift-'))
const fresh = path.join(tmp, 'openapi-types.ts')

try {
  execSync(`node_modules/.bin/openapi-typescript ${SPEC} -o ${fresh}`, {
    cwd: REPO_ROOT,
    stdio: ['ignore', 'pipe', 'pipe']
  })

  if (!fs.existsSync(CURRENT)) {
    console.error(`FAIL: ${path.relative(REPO_ROOT, CURRENT)} is missing`)
    console.error('Run: npm run generate:openapi-types')
    process.exit(1)
  }

  const a = fs.readFileSync(CURRENT, 'utf8')
  const b = fs.readFileSync(fresh, 'utf8')

  if (a !== b) {
    console.error(`FAIL: ${path.relative(REPO_ROOT, CURRENT)} is out of date with openapi.json`)
    console.error('Run: npm run generate:openapi-types')
    console.error('(then commit the regenerated file)')
    process.exit(1)
  }

  console.log('OK: openapi-types.ts is in sync with openapi.json')
} finally {
  fs.rmSync(tmp, { recursive: true, force: true })
}

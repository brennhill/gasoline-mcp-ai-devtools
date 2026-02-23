#!/usr/bin/env node
// Purpose: Enforce required source headers across core Go/TypeScript runtime files.
// Why: Prevents drift where code ownership/intent links disappear from active development paths.
// Docs: docs/DEVELOPMENT.md

import fs from 'node:fs'
import path from 'node:path'

const repoRoot = process.cwd()
const roots = ['src', 'cmd', 'internal']

function walk(dir, out) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      walk(full, out)
      continue
    }
    out.push(full)
  }
}

function isTarget(file) {
  const rel = path.relative(repoRoot, file)
  if (rel.includes('/testdata/')) return false
  if (rel.endsWith('_test.go')) return false
  if (rel.endsWith('.test.ts')) return false
  if (rel.endsWith('.d.ts')) return false
  if (rel.endsWith('/doc.go')) return false
  return rel.endsWith('.ts') || rel.endsWith('.go')
}

function hasHeader(content) {
  const head = content.split('\n').slice(0, 40).join('\n')
  const hasGenericPurpose = /Purpose:\s*Owns .* runtime behavior and integration logic\./.test(head)
  const hasGenericWhy = /Why:\s*Documents why this unit exists so maintenance decisions remain grounded in intent\./.test(head)
  return (
    /Purpose:\s*\S/.test(head) &&
    /Why:\s*\S/.test(head) &&
    /Docs:\s*docs\/features\/feature\/[a-z0-9-]+\/index\.md/.test(head) &&
    !hasGenericPurpose &&
    !hasGenericWhy
  )
}

function main() {
  const files = []
  for (const r of roots) {
    const dir = path.join(repoRoot, r)
    if (fs.existsSync(dir)) walk(dir, files)
  }
  const targets = files.filter(isTarget).sort((a, b) => a.localeCompare(b))

  const missing = []
  for (const file of targets) {
    const content = fs.readFileSync(file, 'utf8')
    if (!hasHeader(content)) missing.push(path.relative(repoRoot, file))
  }

  if (missing.length > 0) {
    console.error(`source header check failed: ${missing.length} file(s) missing Purpose/Why/Docs header`)
    for (const m of missing.slice(0, 200)) console.error(`- ${m}`)
    if (missing.length > 200) console.error(`... and ${missing.length - 200} more`)
    process.exit(1)
  }

  console.log(`source header check passed for ${targets.length} files`)
}

main()

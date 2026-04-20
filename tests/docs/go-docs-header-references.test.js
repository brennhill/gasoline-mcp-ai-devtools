// @ts-nocheck

// Purpose: Regression guard for "// Docs:" cross-references in Go source files.
// Every path that appears after "// Docs:" (comma-separated, whitespace-tolerant)
// must resolve to a real file in the repo. Catches drift like
// "docs/features/feature/<name>/index.md" where <name> was renamed or never created.

import { describe, test } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const TEST_DIR = path.dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = path.resolve(TEST_DIR, '../..')

// Directories to scan for Go files. Skips vendored, build output, worktree copies,
// and npm prebuilts to keep the test fast and stable.
const SCAN_DIRS = ['cmd', 'internal', 'tests']
const SKIP_SEGMENTS = new Set([
  'node_modules',
  'dist',
  '.git',
  '.worktrees',
  'getstrum.dev',
  'gokaboom.dev',
  'claude_skill'
])

function walkGoFiles(dir, out) {
  let entries
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true })
  } catch {
    return
  }
  for (const entry of entries) {
    if (SKIP_SEGMENTS.has(entry.name)) continue
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      walkGoFiles(full, out)
    } else if (entry.isFile() && entry.name.endsWith('.go')) {
      out.push(full)
    }
  }
}

function collectDocsHeaders(files) {
  // Map relative file → array of doc paths (already trimmed).
  const result = new Map()
  for (const file of files) {
    const text = fs.readFileSync(file, 'utf8')
    const refs = []
    // Match every "// Docs: <path>" line. We only read the first line after the
    // marker; multi-path forms use comma separation on the same line.
    const pattern = /\/\/\s*Docs:\s*(.+)$/gm
    let m
    while ((m = pattern.exec(text)) !== null) {
      const raw = m[1].trim()
      for (const part of raw.split(/[,;]/)) {
        const p = part.trim()
        if (p.length === 0) continue
        // Skip URLs — only repo-relative paths are asserted.
        if (/^https?:\/\//.test(p)) continue
        refs.push(p)
      }
    }
    if (refs.length > 0) {
      result.set(path.relative(REPO_ROOT, file), refs)
    }
  }
  return result
}

describe('go docs header references', () => {
  test('every "// Docs:" path in Go sources resolves to a real repo file', () => {
    const files = []
    for (const dir of SCAN_DIRS) {
      walkGoFiles(path.join(REPO_ROOT, dir), files)
    }
    const headers = collectDocsHeaders(files)
    const missing = []
    for (const [source, refs] of headers.entries()) {
      for (const ref of refs) {
        const absolute = path.join(REPO_ROOT, ref)
        if (!fs.existsSync(absolute)) {
          missing.push(`${source} → ${ref}`)
        }
      }
    }
    assert.deepStrictEqual(
      missing,
      [],
      `Go "// Docs:" headers reference paths that do not exist:\n  ${missing.join('\n  ')}`
    )
  })
})

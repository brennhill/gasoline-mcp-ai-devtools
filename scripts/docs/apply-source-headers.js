#!/usr/bin/env node
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

function inferDocs(rel) {
  const docs = new Set()
  const add = (slug) => docs.add(`docs/features/feature/${slug}/index.md`)

  if (rel.startsWith('cmd/dev-console/tools_analyze')) add('analyze-tool')
  if (rel.startsWith('cmd/dev-console/tools_interact')) add('interact-explore')
  if (rel.startsWith('cmd/dev-console/tools_observe')) add('observe')
  if (rel.startsWith('cmd/dev-console/tools_generate') || rel.includes('/testgen')) add('test-generation')
  if (rel.includes('reproduction')) add('reproduction-scripts')
  if (rel.startsWith('internal/export/')) {
    add('har-export')
    add('sarif-export')
  }
  if (rel.startsWith('internal/redaction/')) add('redaction-patterns')
  if (rel.startsWith('internal/performance/')) add('performance-audit')
  if (rel.startsWith('internal/capture/')) add('backend-log-streaming')
  if (rel.startsWith('internal/observe/')) add('observe')
  if (rel.startsWith('internal/session/')) {
    add('observe')
    add('pagination')
  }
  if (rel.startsWith('src/lib/dom-queries')) add('query-dom')
  if (rel.startsWith('src/lib/link-health')) add('link-health')
  if (rel.startsWith('src/lib/perf') || rel.startsWith('src/lib/performance')) add('performance-audit')
  if (rel.startsWith('src/lib/network') || rel.startsWith('src/lib/websocket')) add('backend-log-streaming')
  if (rel.startsWith('src/background/')) {
    add('analyze-tool')
    add('interact-explore')
    add('observe')
  }
  if (rel.startsWith('src/content/')) {
    add('interact-explore')
    add('query-dom')
  }
  if (rel.startsWith('src/inject/')) {
    add('interact-explore')
    add('query-dom')
  }
  if (rel === 'src/background.ts' || rel === 'src/content.ts' || rel === 'src/inject.ts') {
    add('interact-explore')
    add('analyze-tool')
  }

  if (docs.size === 0) {
    add('observe')
  }

  return Array.from(docs)
}

function inferPurpose(rel) {
  const base = path.basename(rel)
  if (rel.startsWith('cmd/dev-console/tools_analyze')) return 'Implements analyze tool handlers and response shaping.'
  if (rel.startsWith('cmd/dev-console/tools_interact')) return 'Implements interact tool handlers and browser action orchestration.'
  if (rel.startsWith('cmd/dev-console/tools_observe')) return 'Implements observe tool queries against captured runtime buffers.'
  if (rel.startsWith('cmd/dev-console/tools_generate')) return 'Implements generate tool formats and output assembly.'
  if (rel.startsWith('internal/export/')) return 'Implements export serializers and format-specific output builders.'
  if (rel.startsWith('internal/redaction/')) return 'Implements redaction rules for sensitive data in captured telemetry.'
  if (rel.startsWith('internal/performance/')) return 'Implements performance metric diffing and threshold evaluation.'
  if (rel.startsWith('internal/session/')) return 'Implements session lifecycle, snapshots, and diff state management.'
  if (rel.startsWith('src/background/')) return 'Handles extension background coordination and message routing.'
  if (rel.startsWith('src/content/')) return 'Handles content-script message relay between background and inject contexts.'
  if (rel.startsWith('src/inject/')) return 'Executes in-page actions and query handlers within the page context.'
  if (rel.startsWith('src/lib/')) return 'Provides shared runtime utilities used by extension and server workflows.'
  return `Owns ${base} runtime behavior and integration logic.`
}

function hasHeader(content) {
  const head = content.split('\n').slice(0, 40).join('\n')
  return /Purpose:\s*\S/.test(head) && /Docs:\s*docs\/features\/feature\/[a-z0-9-]+\/index\.md/.test(head)
}

function tsHeader(rel) {
  const docs = inferDocs(rel).map((d) => ` * Docs: ${d}`).join('\n')
  return `/**\n * Purpose: ${inferPurpose(rel)}\n${docs}\n */\n`
}

function goHeader(rel) {
  const docs = inferDocs(rel).map((d) => `// Docs: ${d}`).join('\n')
  return `// Purpose: ${inferPurpose(rel)}\n${docs}\n`
}

function insertHeader(rel, content) {
  const isGo = rel.endsWith('.go')
  const header = isGo ? goHeader(rel) : tsHeader(rel)
  if (content.startsWith('#!')) {
    const nl = content.indexOf('\n')
    if (nl !== -1) {
      return `${content.slice(0, nl + 1)}${header}${content.slice(nl + 1)}`
    }
  }
  return `${header}\n${content}`
}

function main() {
  const files = []
  for (const r of roots) {
    const dir = path.join(repoRoot, r)
    if (fs.existsSync(dir)) walk(dir, files)
  }
  const targets = files.filter(isTarget).sort((a, b) => a.localeCompare(b))
  let updated = 0

  for (const file of targets) {
    const rel = path.relative(repoRoot, file)
    const content = fs.readFileSync(file, 'utf8')
    if (hasHeader(content)) continue
    fs.writeFileSync(file, insertHeader(rel, content), 'utf8')
    updated += 1
  }

  console.log(`updated ${updated} file(s) with source headers`)
}

main()

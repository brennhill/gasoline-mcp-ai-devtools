#!/usr/bin/env node
import fs from 'node:fs'
import path from 'node:path'

const repoRoot = process.cwd()
const featuresRoot = path.join(repoRoot, 'docs', 'features')
const requiredFiles = ['index.md', 'product-spec.md', 'tech-spec.md', 'qa-plan.md']
const requiredFrontmatterKeys = ['doc_type', 'feature_id', 'last_reviewed']

const featureDirPredicates = [
  (rel) => rel.startsWith('feature/'),
  (rel) => rel.startsWith('bug/'),
  (rel) =>
    [
      'draw-mode',
      'file-upload',
      'cli-interface',
      'lifecycle-monitoring',
      'npm-preinstall-fix',
      'mcp-persistent-server',
      'icon-regression'
    ].includes(rel)
]

function isDir(p) {
  try {
    return fs.statSync(p).isDirectory()
  } catch {
    return false
  }
}

function parseFrontmatter(content) {
  if (!content.startsWith('---\n')) return {}
  const end = content.indexOf('\n---\n', 4)
  if (end === -1) return {}
  const lines = content.slice(4, end).split('\n')
  const out = {}
  for (const line of lines) {
    const idx = line.indexOf(':')
    if (idx <= 0) continue
    const key = line.slice(0, idx).trim()
    const value = line.slice(idx + 1).trim()
    out[key] = value
  }
  return out
}

function discoverFeatureDirs() {
  const dirs = []
  const stack = [featuresRoot]
  while (stack.length > 0) {
    const current = stack.pop()
    if (!current || !isDir(current)) continue
    for (const entry of fs.readdirSync(current)) {
      const full = path.join(current, entry)
      if (!isDir(full)) continue
      stack.push(full)
      const rel = path.relative(featuresRoot, full)
      if (featureDirPredicates.some((fn) => fn(rel))) {
        dirs.push(full)
      }
    }
  }
  return dirs.sort((a, b) => a.localeCompare(b))
}

function main() {
  const strictFrontmatter = process.env.DOCS_STRICT_FRONTMATTER === '1'
  const dirs = discoverFeatureDirs()
  const issues = []

  for (const dir of dirs) {
    const rel = path.relative(repoRoot, dir)
    for (const fileName of requiredFiles) {
      const filePath = path.join(dir, fileName)
      if (!fs.existsSync(filePath)) {
        issues.push(`${rel}: missing ${fileName}`)
        continue
      }
      const content = fs.readFileSync(filePath, 'utf8')
      const fm = parseFrontmatter(content)
      // Progressive rollout:
      // - Always require frontmatter metadata on index.md
      // - Require it on all files only when DOCS_STRICT_FRONTMATTER=1
      const shouldCheckFrontmatter = fileName === 'index.md' || strictFrontmatter
      if (shouldCheckFrontmatter) {
        for (const key of requiredFrontmatterKeys) {
          if (!fm[key]) {
            issues.push(`${rel}/${fileName}: missing frontmatter key '${key}'`)
          }
        }
      }
    }
  }

  if (issues.length > 0) {
    console.error(`feature bundle check failed: ${issues.length} issue(s)`)
    for (const issue of issues.slice(0, 200)) {
      console.error(`- ${issue}`)
    }
    if (issues.length > 200) {
      console.error(`... and ${issues.length - 200} more`)
    }
    process.exit(1)
  }

  console.log(`feature bundle check passed for ${dirs.length} feature directories`)
}

main()

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '..', '..')
const NEW_FLOW_MAP = 'docs/architecture/flow-maps/gokaboom-content-publishing-and-agent-markdown.md'
const OLD_FLOW_MAP = 'docs/architecture/flow-maps/cookwithgasoline-content-publishing-and-agent-markdown.md'
const DOCS_TO_SCAN = [
  'docs/architecture/flow-maps/README.md',
  NEW_FLOW_MAP,
  'docs/architecture/flow-maps/terminal-side-panel-host.md',
  'docs/architecture/flow-maps/tracked-tab-hover-quick-actions.md',
  'docs/features/feature/terminal/index.md',
  'docs/features/feature/terminal/flow-map.md',
  'docs/features/feature/terminal/product-spec.md',
  'docs/features/feature/terminal/tech-spec.md',
  'docs/features/feature/tab-tracking-ux/index.md',
  'docs/features/feature/tab-tracking-ux/flow-map.md'
]

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

test('kaboom docs flow maps point at gokaboom and kaboom naming', () => {
  assert.ok(fs.existsSync(path.join(REPO_ROOT, NEW_FLOW_MAP)))
  assert.ok(!fs.existsSync(path.join(REPO_ROOT, OLD_FLOW_MAP)))

  const readme = read('docs/architecture/flow-maps/README.md')
  assert.match(readme, /Gokaboom Content Publishing and Agent Markdown/)
  assert.doesNotMatch(readme, /Cookwithgasoline Content Publishing and Agent Markdown/)

  for (const relativePath of DOCS_TO_SCAN) {
    const source = read(relativePath)
    assert.doesNotMatch(
      source,
      /STRUM|Gasoline|cookwithgasoline|Cookwithgasoline|getstrum/
    )
  }

  const terminalIndex = read('docs/features/feature/terminal/index.md')
  assert.match(terminalIndex, /Kaboom work context/)

  const hoverFlowMap = read('docs/architecture/flow-maps/tracked-tab-hover-quick-actions.md')
  assert.match(hoverFlowMap, /Kaboom terminal side panel/)
  assert.match(hoverFlowMap, /Hide Kaboom Devtool/)
})

const test = require('node:test')
const assert = require('node:assert')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '../..')

test('clean-old-daemons script uses Kaboom copy while targeting legacy processes', () => {
  const source = fs.readFileSync(path.join(REPO_ROOT, 'scripts/clean-old-daemons.sh'), 'utf8')

  assert.match(source, /Kaboom Daemon Cleanup/)
  assert.match(source, /Safe to install or upgrade Kaboom now:/)
  assert.match(source, /npm install -g kaboom-agentic-browser@latest/)

  assert.match(source, /gasoline.*--daemon|gasoline\.exe|lsof -c gasoline/)
  assert.match(source, /strum.*--daemon|strum\.exe|lsof -c strum/)
  assert.match(source, /for legacy_name in gasoline strum; do/)
  assert.match(source, /\.\\?\$\{legacy_name\}-\$port\.pid/)

  assert.doesNotMatch(source, /STRUM Daemon Cleanup/)
  assert.doesNotMatch(source, /Safe to install or upgrade STRUM now:/)
  assert.doesNotMatch(source, /npm install -g gasoline-mcp@latest/)
})

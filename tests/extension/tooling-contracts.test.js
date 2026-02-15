// @ts-nocheck
import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'

import eslintConfig from '../../eslint.config.js'

function findConfigBlock(glob) {
  return eslintConfig.find((entry) => Array.isArray(entry.files) && entry.files.includes(glob))
}

describe('Tooling contracts', () => {
  test('eslint scripts block should load security plugin', () => {
    const scriptsBlock = findConfigBlock('scripts/**/*.js')
    assert.ok(scriptsBlock, 'expected scripts block in eslint.config.js')
    assert.ok(
      scriptsBlock.plugins && scriptsBlock.plugins.security,
      'scripts/**/*.js block must load eslint-plugin-security'
    )
  })

  test('eslint extension test block should define chrome global', () => {
    const testsBlock = findConfigBlock('tests/extension/**/*.js')
    assert.ok(testsBlock, 'expected tests/extension block in eslint.config.js')
    assert.strictEqual(
      testsBlock.languageOptions?.globals?.chrome,
      'readonly',
      'tests/extension block must define chrome as readonly'
    )
  })

  test('validate-versions should use VERSION file as source of truth (not brittle Makefile parsing)', () => {
    const script = readFileSync('scripts/validate-versions.sh', 'utf8')
    assert.match(
      script,
      /VERSION=\$\(tr -d '\[:space:\]' < VERSION\)/,
      'validate-versions should read semver from VERSION file'
    )
    assert.doesNotMatch(
      script,
      /grep "\^VERSION :=" Makefile \| awk '\{print \$3\}'/,
      'validate-versions must not parse VERSION from Makefile token position'
    )
  })

  test('validate-architecture should enforce /sync handler instead of removed legacy handlers', () => {
    const script = readFileSync('scripts/validate-architecture.sh', 'utf8')
    assert.match(script, /HandleSync/, 'validate-architecture should require HandleSync')
    assert.doesNotMatch(
      script,
      /HandlePendingQueries|HandleDOMResult|HandleExecuteResult|HandlePilotStatus/,
      'validate-architecture should not require removed legacy handlers'
    )
  })

  test('validate-architecture stub check should not depend on fixed grep context windows', () => {
    const script = readFileSync('scripts/validate-architecture.sh', 'utf8')
    assert.doesNotMatch(
      script,
      /grep\s+-r?A\s+20/,
      'stub detection must not use grep -A 20 windows (brittle false negatives)'
    )
  })

  test('validate-architecture should not hardcode AsyncCommandTimeout to 30s', () => {
    const script = readFileSync('scripts/validate-architecture.sh', 'utf8')
    assert.doesNotMatch(
      script,
      /AsyncCommandTimeout\.\*30\.\*time\.Second/,
      'AsyncCommandTimeout check should not be hardcoded to exactly 30s'
    )
    assert.match(
      script,
      /AsyncCommandTimeout too low/,
      'AsyncCommandTimeout check should enforce a minimum threshold'
    )
  })

  test('validate-versions should use file-specific checks for dynamic and placeholder version files', () => {
    const script = readFileSync('scripts/validate-versions.sh', 'utf8')
    assert.match(
      script,
      /server\/scripts\/install\.js uses package\.json version source/,
      'validate-versions should special-case install.js dynamic version sourcing'
    )
    assert.match(
      script,
      /mcp-initialize\.golden\.json uses VERSION placeholder/,
      'validate-versions should special-case VERSION placeholders in golden files'
    )
    assert.match(
      script,
      /export_sarif\.go uses build-time injected version fallback/,
      'validate-versions should special-case build-time injected version vars'
    )
  })
})

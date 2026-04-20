/**
 * @fileoverview Regression guard: install.sh must force STRICT_CHECKSUM=1 when
 * KABOOM_SELF_UPDATE=1 is set, so daemon-triggered self-updates always verify
 * release artifact integrity even if checksums.txt fetch is flaky or the caller
 * env forgot KABOOM_INSTALL_STRICT=1.
 *
 * The runner at internal/upgrade/runner_unix.go sets KABOOM_SELF_UPDATE=1 when
 * spawning the install script. install.sh must recognize that flag and opt into
 * strict checksum verification unconditionally on that code path.
 */

import { test } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const REPO_ROOT = path.resolve(__dirname, '../..')
const INSTALL_SH_PATH = path.join(REPO_ROOT, 'scripts/install.sh')

function readInstallScript() {
  return fs.readFileSync(INSTALL_SH_PATH, 'utf8')
}

test('install.sh overrides STRICT_CHECKSUM=1 when KABOOM_SELF_UPDATE=1 and keeps legacy env default otherwise', () => {
  const script = readInstallScript()

  // 1. Capture both branches of the override logic in a single regex. The
  //    self-update branch must set STRICT_CHECKSUM=1 unconditionally; the else
  //    branch must preserve the KABOOM_INSTALL_STRICT-driven default.
  const overrideBlock =
    /if\s+\[\s+"\$\{KABOOM_SELF_UPDATE:-\}"\s+=\s+"1"\s+\]\s*;\s*then\s+STRICT_CHECKSUM=1\s+else\s+STRICT_CHECKSUM="\$\{KABOOM_INSTALL_STRICT:-0\}"\s+fi/
  assert.match(
    script,
    overrideBlock,
    'install.sh must contain the KABOOM_SELF_UPDATE override block that forces STRICT_CHECKSUM=1 in the then branch and preserves the KABOOM_INSTALL_STRICT default in the else branch'
  )

  // 2. The assigned STRICT_CHECKSUM=1 literal must appear inside the then
  //    branch, not elsewhere as a standalone assignment. We extract the
  //    block and confirm the unconditional assignment lives within it.
  const blockMatch = script.match(overrideBlock)
  assert.ok(blockMatch, 'expected override block match to be captured')
  const block = blockMatch[0]
  assert.match(
    block,
    /then\s+STRICT_CHECKSUM=1/,
    'STRICT_CHECKSUM=1 must be inside the KABOOM_SELF_UPDATE then branch'
  )

  // 3. The legacy pattern must survive, but only inside the else branch. This
  //    is the regression guard: if a future edit strips the override (e.g. by
  //    reverting to the single assignment), this test fails.
  const legacyPattern = 'STRICT_CHECKSUM="${KABOOM_INSTALL_STRICT:-0}"'
  assert.ok(
    script.includes(legacyPattern),
    `install.sh must preserve the legacy pattern ${legacyPattern} in the else branch`
  )
  assert.ok(
    block.includes(legacyPattern),
    `the legacy pattern ${legacyPattern} must live inside the else branch of the KABOOM_SELF_UPDATE override block`
  )

  // 4. Guard against the legacy pattern reappearing at top level as a bare
  //    assignment (which would mean the override was removed or shadowed).
  //    Count occurrences; exactly one occurrence (inside the else branch) is
  //    expected.
  const occurrences = script.split(legacyPattern).length - 1
  assert.strictEqual(
    occurrences,
    1,
    `legacy STRICT_CHECKSUM assignment should appear exactly once (inside the else branch); found ${occurrences}`
  )
})

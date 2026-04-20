// @ts-nocheck

// Purpose: Regression guard for the KABOOM_EXTENSION_ID propagation in
// scripts/install.sh. The daemon's extensionOnly middleware can scope Origin
// matching to a specific extension ID when the env var is set, which closes
// the cross-extension nonce-harvest hole. This test pins the plumbing across
// launchd (macOS), systemd (Linux), and XDG autostart (non-systemd Linux) so
// no future edit silently drops one supervisor path.

import { describe, test } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const TEST_DIR = path.dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = path.resolve(TEST_DIR, '../..')
const INSTALL_SH = fs.readFileSync(path.join(REPO_ROOT, 'scripts/install.sh'), 'utf8')

describe('install.sh KABOOM_EXTENSION_ID scoping', () => {
  test('exports KABOOM_EXTENSION_ID from caller env with empty default', () => {
    assert.match(
      INSTALL_SH,
      /KABOOM_EXTENSION_ID="\$\{KABOOM_EXTENSION_ID:-\}"/,
      'install.sh must accept KABOOM_EXTENSION_ID from caller env with empty default'
    )
  })

  test('renders EnvironmentVariables dict in launchd plist when set', () => {
    // Plist block is emitted into $plist_env_block via a conditional guarded
    // by [ -n "$KABOOM_EXTENSION_ID" ].
    assert.match(
      INSTALL_SH,
      /plist_env_block="\s*<key>EnvironmentVariables<\/key>[\s\S]+?<key>KABOOM_EXTENSION_ID<\/key>[\s\S]+?<string>\$KABOOM_EXTENSION_ID<\/string>/,
      'launchd plist must embed KABOOM_EXTENSION_ID when set'
    )
  })

  test('renders Environment= line in systemd unit when set', () => {
    assert.match(
      INSTALL_SH,
      /systemd_env_line="Environment=KABOOM_EXTENSION_ID=\$KABOOM_EXTENSION_ID"/,
      'systemd unit must embed KABOOM_EXTENSION_ID when set'
    )
    assert.match(
      INSTALL_SH,
      /ExecStart=\$CANONICAL_KABOOM_BIN --daemon --port 7890\n\$systemd_env_line/,
      'Environment= line must appear in the unit right after ExecStart'
    )
  })

  test('renders env prefix in XDG autostart Exec line when set', () => {
    assert.match(
      INSTALL_SH,
      /desktop_exec_prefix="env KABOOM_EXTENSION_ID=\$KABOOM_EXTENSION_ID "/,
      'XDG autostart must prefix Exec= with env KABOOM_EXTENSION_ID when set'
    )
    assert.match(
      INSTALL_SH,
      /Exec=\$\{desktop_exec_prefix\}\$CANONICAL_KABOOM_BIN/,
      'desktop_exec_prefix must actually be interpolated into the Exec= line'
    )
  })

  test('propagates KABOOM_EXTENSION_ID to nohup fallback when set', () => {
    assert.match(
      INSTALL_SH,
      /KABOOM_EXTENSION_ID="\$KABOOM_EXTENSION_ID" nohup "\$CANONICAL_KABOOM_BIN"/,
      'non-systemd nohup fallback must inherit KABOOM_EXTENSION_ID'
    )
  })

  test('all three env-block variables are guarded by a single conditional', () => {
    // All three persistence slots share one guard block so the render logic
    // can only be DRY-broken by removing or splitting the conditional, which
    // this match catches.
    assert.match(
      INSTALL_SH,
      /if \[ -n "\$KABOOM_EXTENSION_ID" \]; then\n\s*plist_env_block="[\s\S]+?systemd_env_line="[\s\S]+?desktop_exec_prefix="[\s\S]+?fi/,
      'plist/systemd/desktop env rendering must stay grouped under one conditional'
    )
  })
})

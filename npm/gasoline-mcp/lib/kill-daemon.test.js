const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

function writeExecutable(filePath, body) {
  fs.writeFileSync(filePath, body, { mode: 0o755 });
}

function runKillDaemon({ homeDir, binDir, env = {} }) {
  const scriptPath = path.join(__dirname, 'kill-daemon.js');
  const run = spawnSync(process.execPath, [scriptPath], {
    env: {
      ...process.env,
      ...env,
      HOME: homeDir,
      PATH: `${binDir}${path.delimiter}${process.env.PATH || ''}`,
    },
    encoding: 'utf8',
  });
  assert.equal(run.status, 0, `kill-daemon.js exited with ${run.status}: ${run.stderr}`);
}

test('cleanup targets legacy and current daemon names', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-kill-test-'));
  const binDir = path.join(tmp, 'bin');
  fs.mkdirSync(binDir, { recursive: true });

  const pkillLog = path.join(tmp, 'pkill.log');
  writeExecutable(
    path.join(binDir, 'pkill'),
    `#!/bin/sh
echo "$@" >> "${pkillLog}"
exit 0
`
  );

  runKillDaemon({ homeDir: tmp, binDir });

  const log = fs.existsSync(pkillLog) ? fs.readFileSync(pkillLog, 'utf8') : '';
  assert.match(log, /gasoline-mcp/, 'expected cleanup to target gasoline-mcp');
  assert.match(log, /dev-console/, 'expected cleanup to target legacy dev-console');
});

test('cleanup removes modern and legacy pid files', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-kill-pids-'));
  const binDir = path.join(tmp, 'bin');
  fs.mkdirSync(binDir, { recursive: true });

  writeExecutable(
    path.join(binDir, 'pkill'),
    `#!/bin/sh
exit 0
`
  );

  const modernPid = path.join(tmp, '.gasoline', 'run', 'gasoline-7890.pid');
  const legacyPid = path.join(tmp, '.gasoline-7890.pid');
  fs.mkdirSync(path.dirname(modernPid), { recursive: true });
  fs.writeFileSync(modernPid, '123');
  fs.writeFileSync(legacyPid, '456');

  runKillDaemon({ homeDir: tmp, binDir });

  assert.equal(fs.existsSync(modernPid), false, `expected pid file removed: ${modernPid}`);
  assert.equal(fs.existsSync(legacyPid), false, `expected pid file removed: ${legacyPid}`);
});

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { KNOWN_PORTS } = require('./kill-daemon');

function writeExecutable(filePath, body) {
  fs.writeFileSync(filePath, body, { mode: 0o755 });
}

function runKillDaemon({ homeDir, binDir, env = {}, logPath }) {
  const scriptPath = path.join(__dirname, 'kill-daemon.js');
  const run = spawnSync(process.execPath, [scriptPath], {
    env: {
      ...process.env,
      ...env,
      GASOLINE_KILL_DAEMON_DRY_RUN: '1',
      GASOLINE_KILL_DAEMON_LOG: logPath || '',
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

  const logPath = path.join(tmp, 'kill-daemon.log');
  runKillDaemon({ homeDir: tmp, binDir, logPath });

  const log = fs.existsSync(logPath) ? fs.readFileSync(logPath, 'utf8') : '';
  if (process.platform === 'win32') {
    assert.match(log, /gasoline\*\.exe/, 'expected cleanup to target gasoline*.exe');
    assert.match(log, /dev-console\*\.exe/, 'expected cleanup to target legacy dev-console*.exe');
    assert.match(log, /\[execFile\] gasoline-mcp --force/, 'expected cleanup to invoke gasoline-mcp --force');
    return;
  }

  assert.match(log, /\[pattern\] gasoline-mcp/, 'expected cleanup to target gasoline-mcp');
  assert.match(log, /\[pattern\] dev-console/, 'expected cleanup to target legacy dev-console');
});

test('cleanup removes modern and legacy pid files', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-kill-pids-'));
  const binDir = path.join(tmp, 'bin');
  fs.mkdirSync(binDir, { recursive: true });

  const modernPid = path.join(tmp, '.gasoline', 'run', 'gasoline-7890.pid');
  const legacyPid = path.join(tmp, '.gasoline-7890.pid');
  fs.mkdirSync(path.dirname(modernPid), { recursive: true });
  fs.writeFileSync(modernPid, '123');
  fs.writeFileSync(legacyPid, '456');

  runKillDaemon({ homeDir: tmp, binDir, logPath: path.join(tmp, 'kill-daemon.log') });

  assert.equal(fs.existsSync(modernPid), false, `expected pid file removed: ${modernPid}`);
  assert.equal(fs.existsSync(legacyPid), false, `expected pid file removed: ${legacyPid}`);
});

test('cleanup removes pid files across known ports and XDG state root', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-kill-known-ports-'));
  const binDir = path.join(tmp, 'bin');
  const xdgStateHome = path.join(tmp, 'xdg-state');
  fs.mkdirSync(binDir, { recursive: true });

  const trackedPaths = [];
  for (const port of KNOWN_PORTS) {
    const modernPid = path.join(tmp, '.gasoline', 'run', `gasoline-${port}.pid`);
    const xdgPid = path.join(xdgStateHome, 'gasoline', 'run', `gasoline-${port}.pid`);
    const legacyPid = path.join(tmp, `.gasoline-${port}.pid`);

    fs.mkdirSync(path.dirname(modernPid), { recursive: true });
    fs.mkdirSync(path.dirname(xdgPid), { recursive: true });
    fs.writeFileSync(modernPid, String(port));
    fs.writeFileSync(xdgPid, String(port));
    fs.writeFileSync(legacyPid, String(port));

    trackedPaths.push(modernPid, xdgPid, legacyPid);
  }

  runKillDaemon({
    homeDir: tmp,
    binDir,
    env: { XDG_STATE_HOME: xdgStateHome },
    logPath: path.join(tmp, 'kill-daemon.log'),
  });

  for (const pidPath of trackedPaths) {
    assert.equal(fs.existsSync(pidPath), false, `expected pid file removed: ${pidPath}`);
  }
});

test('cleanup attempts to terminate pids discovered from pid files', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-kill-pid-kill-'));
  const binDir = path.join(tmp, 'bin');
  fs.mkdirSync(binDir, { recursive: true });

  const modernPid = path.join(tmp, '.gasoline', 'run', 'gasoline-22222.pid');
  fs.mkdirSync(path.dirname(modernPid), { recursive: true });
  fs.writeFileSync(modernPid, '22222');

  const logPath = path.join(tmp, 'kill-daemon.log');
  runKillDaemon({ homeDir: tmp, binDir, logPath });

  const log = fs.existsSync(logPath) ? fs.readFileSync(logPath, 'utf8') : '';
  assert.match(log, /\[pid\] 22222/, 'expected cleanup to attempt pid-file based process termination');
});

test('npm lifecycle hooks invoke daemon cleanup script', () => {
  const pkgPath = path.join(__dirname, '..', 'package.json');
  const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));

  assert.equal(pkg?.scripts?.preinstall, 'node lib/kill-daemon.js');
  assert.equal(pkg?.scripts?.preuninstall, 'node lib/kill-daemon.js');
});

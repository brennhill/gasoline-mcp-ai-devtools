// kill-daemon.js — Best-effort daemon cleanup for install/uninstall.
// Goal: old binaries must not survive an upgrade in memory.
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execSync, execFileSync } = require('child_process');

const KNOWN_PORTS = [
  17890,
  ...Array.from({ length: 21 }, (_, i) => 7890 + i),
];

function safeExec(command) {
  try {
    execSync(command, { stdio: 'ignore', shell: true, timeout: 5000 });
  } catch (_) {
    // Best effort only.
  }
}

function safeExecFile(file, args) {
  try {
    execFileSync(file, args, { stdio: 'ignore', timeout: 5000 });
  } catch (_) {
    // Best effort only.
  }
}

function runForceCleanupCommands() {
  // Try installed CLIs first. --force uses the binary's own stop logic.
  for (const binary of ['gasoline-mcp', 'gasoline', 'dev-console']) {
    safeExecFile(binary, ['--force']);
  }
}

function killByProcessName() {
  if (process.platform === 'win32') {
    for (const image of ['gasoline.exe', 'gasoline-mcp.exe', 'dev-console.exe']) {
      safeExec(`taskkill /F /IM ${image} 2>nul`);
    }
    return;
  }

  for (const pattern of ['gasoline-mcp', 'dev-console', 'gasoline']) {
    safeExec(`pkill -9 -f "${pattern}" 2>/dev/null`);
  }
}

function killByKnownPorts() {
  if (process.platform === 'win32') {
    return;
  }
  for (const port of KNOWN_PORTS) {
    safeExec(`lsof -ti :${port} 2>/dev/null | xargs kill -9 2>/dev/null`);
  }
}

function cleanupPIDFiles() {
  const home = os.homedir();
  const modernRoot = path.join(home, '.gasoline', 'run');
  const roots = [modernRoot];
  if (process.env.XDG_STATE_HOME) {
    roots.push(path.join(process.env.XDG_STATE_HOME, 'gasoline', 'run'));
  }

  for (const port of KNOWN_PORTS) {
    for (const root of roots) {
      try {
        fs.rmSync(path.join(root, `gasoline-${port}.pid`), { force: true });
      } catch (_) {
        // Best effort only.
      }
    }
    try {
      fs.rmSync(path.join(home, `.gasoline-${port}.pid`), { force: true });
    } catch (_) {
      // Best effort only.
    }
  }
}

function cleanupOldDaemons() {
  runForceCleanupCommands();
  killByProcessName();
  killByKnownPorts();
  cleanupPIDFiles();
}

if (require.main === module) {
  cleanupOldDaemons();
}

module.exports = {
  cleanupOldDaemons,
  KNOWN_PORTS,
};

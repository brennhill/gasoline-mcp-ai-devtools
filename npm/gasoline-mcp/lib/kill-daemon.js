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

const LOG_PATH = process.env.GASOLINE_KILL_DAEMON_LOG;
const DRY_RUN = process.env.GASOLINE_KILL_DAEMON_DRY_RUN === '1';

function logLine(message) {
  if (!LOG_PATH) return;
  try {
    fs.appendFileSync(LOG_PATH, `${message}\n`);
  } catch (_) {
    // Best effort only.
  }
}

function safeExec(command) {
  logLine(`[exec] ${command}`);
  if (DRY_RUN) return;
  try {
    execSync(command, { stdio: 'ignore', shell: true, timeout: 5000 });
  } catch (_) {
    // Best effort only.
  }
}

function safeExecFile(file, args) {
  logLine(`[execFile] ${file} ${args.join(' ')}`.trim());
  if (DRY_RUN) return;
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

  // Avoid killing this cleanup process (repo path includes "gasoline").
  const selfPid = process.pid;
  const parentPid = process.ppid;
  const isNodeCmd = (cmd) => /\bnode(\s|$)/.test(cmd) || /\bnpm(\s|$)/.test(cmd);

  for (const pattern of ['gasoline-mcp', 'dev-console', 'gasoline']) {
    logLine(`[pattern] ${pattern}`);
    if (DRY_RUN) continue;
    let output = '';
    try {
      output = execSync(`pgrep -af "${pattern}" 2>/dev/null`, { encoding: 'utf8' }).trim();
    } catch (_) {
      output = '';
    }
    if (!output) continue;
    for (const line of output.split('\n')) {
      const trimmed = line.trim();
      if (!trimmed) continue;
      const [pidPart, ...cmdParts] = trimmed.split(/\s+/);
      const pid = Number(pidPart);
      const cmd = cmdParts.join(' ');
      if (!Number.isFinite(pid) || pid <= 1) continue;
      if (pid === selfPid || pid === parentPid) continue;
      if (isNodeCmd(cmd)) continue;
      try {
        process.kill(pid, 'SIGKILL');
      } catch (_) {
        // Best effort only.
      }
    }
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
  const home = process.env.HOME || process.env.USERPROFILE || os.homedir();
  const modernRoot = path.join(home, '.gasoline', 'run');
  const roots = [modernRoot];
  if (process.env.XDG_STATE_HOME) {
    roots.push(path.join(process.env.XDG_STATE_HOME, 'gasoline', 'run'));
  }

  for (const root of roots) {
    try {
      for (const entry of fs.readdirSync(root)) {
        if (entry.startsWith('gasoline-') && entry.endsWith('.pid')) {
          try {
            fs.rmSync(path.join(root, entry), { force: true });
          } catch (_) {
            // Best effort only.
          }
        }
      }
    } catch (_) {
      // Best effort only.
    }
  }

  try {
    for (const entry of fs.readdirSync(home)) {
      if (entry.startsWith('.gasoline-') && entry.endsWith('.pid')) {
        try {
          fs.rmSync(path.join(home, entry), { force: true });
        } catch (_) {
          // Best effort only.
        }
      }
    }
  } catch (_) {
    // Best effort only.
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

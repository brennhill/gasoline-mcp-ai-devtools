/**
 * Doctor diagnostics for Gasoline MCP CLI
 * Checks config validity, binary availability, and provides repair suggestions
 */

const fs = require('fs');
const net = require('net');
const { execSync, execFileSync } = require('child_process');
const {
  CLIENT_DEFINITIONS,
  LEGACY_PATHS,
  getClientConfigPath,
  isClientInstalled,
  commandExistsOnPath,
  readConfigFile,
  expandPath,
} = require('./config');

/**
 * Check if a port is available
 * @param {number} port Port to check
 * @returns {Promise<{available: bool, error?: string}>}
 */
function checkPort(port) {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.once('error', (err) => {
      if (err.code === 'EADDRINUSE') {
        resolve({ available: false, error: `Port ${port} is in use by another process` });
      } else {
        resolve({ available: false, error: err.message });
      }
    });
    server.once('listening', () => {
      server.close();
      resolve({ available: true });
    });
    server.listen(port, '127.0.0.1');
  });
}

/**
 * Synchronous port check (for CLI)
 * @param {number} port Port to check
 * @returns {{available: bool, error?: string}}
 */
function checkPortSync(port) {
  // Validate port is a safe integer to prevent shell injection
  const portNum = parseInt(port, 10);
  if (!Number.isInteger(portNum) || portNum < 1 || portNum > 65535) {
    return { available: false, error: `Invalid port: ${port}` };
  }
  try {
    // Try to check if something is listening
    const result = execSync(`lsof -ti :${portNum} 2>/dev/null || true`, { // nosemgrep: javascript.lang.security.detect-child-process.detect-child-process -- spawning own gasoline binary for health check
      encoding: 'utf8',
      timeout: 2000,
    }).trim();

    if (result) {
      return { available: false, error: `Port ${port} is in use (PID: ${result.split('\n')[0]})` };
    }
    return { available: true };
  } catch (e) {
    // If lsof fails, assume port is available
    return { available: true };
  }
}

/**
 * Test if gasoline binary is available and working
 * @returns {Object} {ok: bool, path?: string, version?: string, error?: string}
 */
function testBinary() {
  try {
    // Try to find the binary from node_modules
    const path = require('path');
    const os = require('os');
    const platform = os.platform();
    const arch = os.arch();

    const platformMap = {
      'darwin-arm64': '@brennhill/gasoline-darwin-arm64',
      'darwin-x64': '@brennhill/gasoline-darwin-x64',
      'linux-arm64': '@brennhill/gasoline-linux-arm64',
      'linux-x64': '@brennhill/gasoline-linux-x64',
      'win32-x64': '@brennhill/gasoline-win32-x64',
    };

    const key = `${platform}-${arch}`;
    const pkg = platformMap[key];

    if (!pkg) {
      return {
        ok: false,
        error: `Unsupported platform: ${platform}-${arch}`,
      };
    }

    // Try to find binary
    const binaryName = platform === 'win32' ? 'gasoline.exe' : 'gasoline';
    const homeDir = os.homedir();

    // Check several locations
    const candidates = [
      path.join(homeDir, '.npm', '_npx', pkg, 'bin', binaryName),
      path.join(homeDir, 'node_modules', pkg, 'bin', binaryName),
      path.join(__dirname, '..', 'node_modules', pkg, 'bin', binaryName),
      path.join(__dirname, '..', '..', pkg, 'bin', binaryName),
      path.join(__dirname, '..', '..', '..', pkg, 'bin', binaryName),
    ];

    let binaryPath = null;
    for (const candidate of candidates) {
      if (fs.existsSync(candidate)) {
        binaryPath = candidate;
        break;
      }
    }

    if (!binaryPath) {
      return {
        ok: false,
        error: `Gasoline binary not found for platform ${key}`,
      };
    }

    // Test binary with --version
    try {
      const version = execFileSync(binaryPath, ['--version'], {
        encoding: 'utf8',
        stdio: ['pipe', 'pipe', 'pipe'],
      }).trim();

      return {
        ok: true,
        path: binaryPath,
        version: version || 'unknown',
      };
    } catch (e) {
      return {
        ok: false,
        path: binaryPath,
        error: 'Binary found but failed to execute',
      };
    }
  } catch (err) {
    return {
      ok: false,
      error: `Error testing binary: ${err.message}`,
    };
  }
}

/**
 * Diagnose a single file-type client
 * @param {Object} def Client definition
 * @param {boolean} verbose
 * @returns {Object} Tool diagnostic
 */
function diagnoseFileClient(def, verbose) {
  const cfgPath = getClientConfigPath(def);
  const detected = isClientInstalled(def);

  const tool = {
    name: def.name,
    id: def.id,
    type: 'file',
    path: cfgPath,
    detected,
    status: 'error',
    issues: [],
    suggestions: [],
  };

  if (verbose) {
    console.log(`[DEBUG] Checking ${def.name} at ${cfgPath}`);
  }

  if (!detected) {
    tool.status = 'info';
    tool.issues.push('Not installed on this system');
    return tool;
  }

  if (!cfgPath) {
    tool.status = 'info';
    tool.issues.push('No config path for this platform');
    return tool;
  }

  if (!fs.existsSync(cfgPath)) {
    tool.status = 'error';
    tool.issues.push('Config file not found');
    tool.suggestions.push('Run: gasoline-mcp --install');
    return tool;
  }

  const readResult = readConfigFile(cfgPath);
  if (!readResult.valid) {
    tool.issues.push('Invalid JSON');
    tool.suggestions.push('Fix the JSON syntax or run: gasoline-mcp --install');
    return tool;
  }

  if (!readResult.data.mcpServers || !readResult.data.mcpServers.gasoline) {
    tool.issues.push('gasoline entry missing from mcpServers');
    tool.suggestions.push('Run: gasoline-mcp --install');
    return tool;
  }

  tool.status = 'ok';
  return tool;
}

/**
 * Diagnose a CLI-type client
 * @param {Object} def Client definition
 * @param {boolean} verbose
 * @returns {Object} Tool diagnostic
 */
function diagnoseCliClient(def, verbose) {
  const detected = isClientInstalled(def);

  const tool = {
    name: def.name,
    id: def.id,
    type: 'cli',
    detected,
    status: 'error',
    issues: [],
    suggestions: [],
  };

  if (verbose) {
    console.log(`[DEBUG] Checking ${def.name} (CLI: ${def.detectCommand})`);
  }

  if (!detected) {
    tool.status = 'info';
    tool.issues.push(`${def.detectCommand} CLI not found on PATH`);
    return tool;
  }

  // Try to check if gasoline is configured via CLI
  try {
    execFileSync(def.detectCommand, ['mcp', 'get', 'gasoline'], {
      stdio: ['pipe', 'pipe', 'pipe'],
      timeout: 10000,
      env: { ...process.env, CLAUDECODE: undefined },
    });
    tool.status = 'ok';
  } catch {
    tool.status = 'error';
    tool.issues.push('gasoline not configured');
    tool.suggestions.push('Run: gasoline-mcp --install');
  }

  return tool;
}

/**
 * Check for legacy/orphaned config files at old paths
 * @returns {Array<Object>} Warnings for legacy paths found
 */
function checkLegacyPaths() {
  const warnings = [];
  for (const legacy of LEGACY_PATHS) {
    const expanded = expandPath(legacy.path);
    if (fs.existsSync(expanded)) {
      try {
        const readResult = readConfigFile(expanded);
        if (readResult.valid && readResult.data.mcpServers && readResult.data.mcpServers.gasoline) {
          warnings.push({
            path: expanded,
            description: legacy.description,
            message: `Orphaned gasoline config at old path: ${expanded}`,
          });
        }
      } catch {
        // Ignore read errors on legacy paths
      }
    }
  }
  return warnings;
}

/**
 * Run full diagnostics on all client locations
 * @param {boolean} verbose If true, log debug info
 * @returns {Object} Diagnostic report with tools array and summary
 */
function runDiagnostics(verbose = false) {
  const tools = [];

  for (const def of CLIENT_DEFINITIONS) {
    if (def.type === 'cli') {
      tools.push(diagnoseCliClient(def, verbose));
    } else {
      tools.push(diagnoseFileClient(def, verbose));
    }
  }

  // Check binary availability
  const binary = testBinary();

  // Check default port availability (7890)
  const defaultPort = 7890;
  const port = checkPortSync(defaultPort);

  // Check for legacy paths
  const legacyWarnings = checkLegacyPaths();

  // Generate summary
  const okCount = tools.filter(t => t.status === 'ok').length;
  const errorCount = tools.filter(t => t.status === 'error').length;
  const infoCount = tools.filter(t => t.status === 'info').length;

  let summary = `Summary: ${okCount} client${okCount === 1 ? '' : 's'} ready`;
  if (errorCount > 0) {
    summary += `, ${errorCount} need${errorCount === 1 ? 's' : ''} repair`;
  }
  if (infoCount > 0) {
    summary += `, ${infoCount} not detected`;
  }

  return {
    tools,
    binary,
    port: { port: defaultPort, ...port },
    legacyWarnings,
    summary,
  };
}

module.exports = {
  testBinary,
  runDiagnostics,
};

// Purpose: Implement uninstall.js behavior for npm wrapper command flows.
// Why: Keeps distribution-channel behavior consistent and supportable.
// Docs: docs/features/feature/enhanced-cli-config/index.md

/**
 * Uninstall logic for Gasoline MCP CLI
 * Removes gasoline from all detected AI assistant clients
 */

const fs = require('fs');
const { execFileSync } = require('child_process');
const {
  CLIENT_DEFINITIONS,
  MCP_SERVER_NAME,
  LEGACY_MCP_SERVER_NAMES,
  getClientConfigPath,
  getDetectedClients,
  readConfigFile,
  writeConfigFile,
} = require('./config');
const { cleanupInstalledSkills } = require('./skills');

const LEGACY_UNINSTALL_SERVER_NAMES = [
  ...LEGACY_MCP_SERVER_NAMES,
  'strum-browser-devtools',
  'strum-agentic-browser',
  'strum',
];

function knownServerNames() {
  return [MCP_SERVER_NAME, ...LEGACY_UNINSTALL_SERVER_NAMES.filter((name) => name !== MCP_SERVER_NAME)];
}

/**
 * Uninstall from a CLI-type client (e.g. Claude Code via `claude mcp remove`)
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, verbose}
 * @returns {Object} {status, name, method, message}
 */
function uninstallViaCli(def, options) {
  const { dryRun = false, verbose = false } = options;
  const cmd = def.detectCommand;
  const canonicalArgs = [...def.removeArgs];

  if (dryRun) {
    if (verbose) {
      console.log(`[DEBUG] Would run: ${cmd} ${canonicalArgs.join(' ')}`);
    }
    return {
      status: 'removed',
      name: def.name,
      id: def.id,
      method: 'cli',
      message: `Would run: ${cmd} ${canonicalArgs.join(' ')}`,
    };
  }

  const env = { ...process.env };
  delete env.CLAUDECODE;
  const serverNames = knownServerNames();
  let lastErr = null;
  for (const serverName of serverNames) {
    const args = [...canonicalArgs];
    if (args.length > 0) {
      args[args.length - 1] = serverName;
    }
    try {
      execFileSync(cmd, args, {
        env,
        stdio: ['pipe', 'pipe', 'pipe'],
        timeout: 15000,
      });
      return {
        status: 'removed',
        name: def.name,
        id: def.id,
        method: 'cli',
        message: `Removed via ${cmd} CLI`,
      };
    } catch (err) {
      lastErr = err;
      const stderr = err.stderr ? err.stderr.toString() : '';
      const notConfigured = stderr.includes('not found') || stderr.includes('does not exist');
      if (!notConfigured) {
        break;
      }
    }
  }
  const stderr = lastErr && lastErr.stderr ? lastErr.stderr.toString() : '';
  if (stderr.includes('not found') || stderr.includes('does not exist')) {
    return {
      status: 'notConfigured',
      name: def.name,
      id: def.id,
      method: 'cli',
    };
  }
  return {
    status: 'error',
    name: def.name,
    id: def.id,
    method: 'cli',
    message: `CLI uninstall failed: ${lastErr ? lastErr.message : 'unknown error'}`,
  };
}

/**
 * Uninstall from a file-type client
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, verbose}
 * @returns {Object} {status, name, method, path}
 */
function uninstallViaFile(def, options) {
  const { dryRun = false, verbose = false } = options;
  const cfgPath = getClientConfigPath(def);

  if (!cfgPath) {
    return { status: 'notConfigured', name: def.name, id: def.id };
  }

  if (!fs.existsSync(cfgPath)) {
    return { status: 'notConfigured', name: def.name, id: def.id };
  }

  const readResult = readConfigFile(cfgPath);
  if (!readResult.valid) {
    return {
      status: 'error',
      name: def.name,
      id: def.id,
      message: `${def.name}: Invalid JSON, cannot uninstall`,
    };
  }

  const configKey = def.configKey || 'mcpServers';
  const servers = readResult.data[configKey] || {};
  const presentServerNames = knownServerNames().filter((name) => Object.prototype.hasOwnProperty.call(servers, name));
  if (presentServerNames.length === 0) {
    return { status: 'notConfigured', name: def.name, id: def.id };
  }

  if (dryRun) {
    if (verbose) {
      console.log(`[DEBUG] Would remove gasoline from ${cfgPath}`);
    }
    return {
      status: 'removed',
      name: def.name,
      id: def.id,
      method: 'file',
      path: cfgPath,
    };
  }

  const modified = structuredClone(readResult.data);
  for (const name of knownServerNames()) {
    delete modified[configKey][name];
  }

  if (Object.keys(modified[configKey]).length > 0) {
    const skipValidation = configKey !== 'mcpServers';
    writeConfigFile(cfgPath, modified, false, { skipValidation });
  } else {
    fs.unlinkSync(cfgPath);
  }

  if (verbose) {
    console.log(`[DEBUG] Removed gasoline from ${cfgPath}`);
  }

  return {
    status: 'removed',
    name: def.name,
    id: def.id,
    method: 'file',
    path: cfgPath,
  };
}

/**
 * Uninstall from a single client (dispatches by type)
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, verbose}
 * @returns {Object} Result with status, name, method
 */
function uninstallFromClient(def, options) {
  if (def.type === 'cli') {
    return uninstallViaCli(def, options);
  }
  return uninstallViaFile(def, options);
}

/**
 * Execute uninstall across all detected clients
 * @param {Object} options {dryRun, verbose, _clientOverrides}
 * @returns {Object} {success, removed, notConfigured, errors}
 */
function executeUninstall(options = {}) {
  const { dryRun = false, verbose = false } = options;

  const clients = options._clientOverrides !== undefined
    ? options._clientOverrides
    : getDetectedClients();

  const result = {
    success: false,
    removed: [],
    notConfigured: [],
    errors: [],
  };

  for (const def of clients) {
    try {
      const r = uninstallFromClient(def, { dryRun, verbose });

      if (r.status === 'removed') {
        result.removed.push(r);
      } else if (r.status === 'notConfigured') {
        result.notConfigured.push(r.name);
      } else if (r.status === 'error') {
        result.errors.push(r.message || `${r.name}: unknown error`);
      }
    } catch (err) {
      result.errors.push(`${def.name}: ${err.message}`);
      if (verbose) {
        console.log(`[DEBUG] Error uninstalling from ${def.name}: ${err.message}`);
      }
    }
  }

  result.skillCleanup = cleanupInstalledSkills({
    dryRun,
    verbose,
    agents: options.skillAgents,
    scope: options.skillScope,
  });
  result.success = result.removed.length > 0 || result.skillCleanup.removed > 0;
  return result;
}

module.exports = {
  uninstallFromClient,
  executeUninstall,
};

/**
 * Uninstall logic for Gasoline MCP CLI
 * Removes gasoline from all detected AI assistant clients
 */

const fs = require('fs');
const { execFileSync } = require('child_process');
const {
  CLIENT_DEFINITIONS,
  getClientConfigPath,
  getDetectedClients,
  readConfigFile,
  writeConfigFile,
} = require('./config');

/**
 * Uninstall from a CLI-type client (e.g. Claude Code via `claude mcp remove`)
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, verbose}
 * @returns {Object} {status, name, method, message}
 */
function uninstallViaCli(def, options) {
  const { dryRun = false, verbose = false } = options;
  const cmd = def.detectCommand;
  const args = [...def.removeArgs];

  if (dryRun) {
    if (verbose) {
      console.log(`[DEBUG] Would run: ${cmd} ${args.join(' ')}`);
    }
    return {
      status: 'removed',
      name: def.name,
      id: def.id,
      method: 'cli',
      message: `Would run: ${cmd} ${args.join(' ')}`,
    };
  }

  try {
    const env = { ...process.env };
    delete env.CLAUDECODE;

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
    // If the command fails because gasoline isn't configured, treat as notConfigured
    const stderr = err.stderr ? err.stderr.toString() : '';
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
      message: `CLI uninstall failed: ${err.message}`,
    };
  }
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
  if (!readResult.data[configKey] || !readResult.data[configKey].gasoline) {
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

  const modified = JSON.parse(JSON.stringify(readResult.data));
  delete modified[configKey].gasoline;

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

  result.success = result.removed.length > 0;
  return result;
}

module.exports = {
  uninstallFromClient,
  executeUninstall,
};

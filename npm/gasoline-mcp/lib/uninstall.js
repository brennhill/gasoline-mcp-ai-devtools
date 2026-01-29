/**
 * Uninstall logic for Gasoline MCP CLI
 * Removes gasoline from config files while preserving other MCP servers
 */

const fs = require('fs');
const { getConfigCandidates, getToolNameFromPath, readConfigFile, writeConfigFile } = require('./config');

/**
 * Execute uninstall operation
 * @param {Object} options {dryRun: bool, verbose: bool}
 * @returns {Object} {success: bool, removed: [{name, path}], notConfigured: [names], errors: []}
 */
function executeUninstall(options = {}) {
  const { dryRun = false, verbose = false } = options;

  const result = {
    success: false,
    removed: [],
    notConfigured: [],
    errors: [],
  };

  const candidates = getConfigCandidates();

  for (const candidatePath of candidates) {
    const toolName = getToolNameFromPath(candidatePath);

    try {
      // Check if file exists
      if (!fs.existsSync(candidatePath)) {
        result.notConfigured.push(toolName);
        continue;
      }

      // Read config
      const readResult = readConfigFile(candidatePath);
      if (!readResult.valid) {
        result.errors.push(`${toolName}: Invalid JSON, cannot uninstall`);
        continue;
      }

      // Check if gasoline is configured
      if (!readResult.data.mcpServers || !readResult.data.mcpServers.gasoline) {
        result.notConfigured.push(toolName);
        continue;
      }

      if (dryRun) {
        result.removed.push({
          name: toolName,
          path: candidatePath,
        });
        if (verbose) {
          console.log(`[DEBUG] Would remove gasoline from ${candidatePath}`);
        }
        continue;
      }

      // Remove gasoline entry
      const modified = JSON.parse(JSON.stringify(readResult.data)); // Deep copy
      delete modified.mcpServers.gasoline;

      // Write back (or delete if no other servers)
      const hasOtherServers = Object.keys(modified.mcpServers).length > 0;
      if (hasOtherServers) {
        writeConfigFile(candidatePath, modified, false);
      } else {
        // No other servers, delete the file
        fs.unlinkSync(candidatePath);
      }

      result.removed.push({
        name: toolName,
        path: candidatePath,
      });

      if (verbose) {
        console.log(`[DEBUG] Removed gasoline from ${candidatePath}`);
      }
    } catch (err) {
      result.errors.push(`${toolName}: ${err.message}`);
      if (verbose) {
        console.log(`[DEBUG] Error uninstalling from ${toolName}: ${err.message}`);
      }
    }
  }

  result.success = result.removed.length > 0;
  return result;
}

module.exports = {
  executeUninstall,
};

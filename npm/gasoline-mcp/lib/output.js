/**
 * Output formatters for Gasoline MCP CLI
 */

/**
 * Format success message
 */
function success(message, details) {
  let output = `✅ ${message}`;
  if (details) {
    output += `\n   ${details}`;
  }
  return output;
}

/**
 * Format error message
 */
function error(message, recovery) {
  let output = `❌ ${message}`;
  if (recovery) {
    output += `\n   ${recovery}`;
  }
  return output;
}

/**
 * Format warning message
 */
function warning(message, details) {
  let output = `⚠️  ${message}`;
  if (details) {
    output += `\n   ${details}`;
  }
  return output;
}

/**
 * Format info message
 */
function info(message, details) {
  let output = `ℹ️  ${message}`;
  if (details) {
    output += `\n   ${details}`;
  }
  return output;
}

/**
 * Format JSON diff for dry-run
 */
function jsonDiff(before, after) {
  const beforeStr = JSON.stringify(before, null, 2);
  const afterStr = JSON.stringify(after, null, 2);

  return `ℹ️  Dry run: No files will be written\n\nBefore:\n${beforeStr}\n\nAfter:\n${afterStr}`;
}

/**
 * Format install result
 */
function installResult(result) {
  let output = '';
  const installed = result.installed || result.updated || [];
  const total = result.total || 5;

  if (installed.length > 0) {
    output += `✅ ${installed.length}/${total} clients updated:\n`;
    installed.forEach(entry => {
      if (entry.method === 'cli') {
        output += `   ✅ ${entry.name} (via CLI)\n`;
      } else {
        output += `   ✅ ${entry.name} (at ${entry.path})\n`;
      }
    });
  }

  if (result.errors && result.errors.length > 0) {
    output += '\n❌ Errors:\n';
    result.errors.forEach(err => {
      if (typeof err === 'string') {
        output += `   ❌ ${err}\n`;
      } else {
        output += `   ❌ ${err.name}: ${err.message}\n`;
      }
    });
  }

  if (result.notFound && result.notFound.length > 0) {
    output += `\nℹ️  Not configured in: ${result.notFound.join(', ')}\n`;
  }

  return output;
}

/**
 * Format doctor diagnostic report
 */
function diagnosticReport(report) {
  let output = '\n📋 Gasoline MCP Diagnostic Report\n\n';

  report.tools.forEach(tool => {
    if (tool.status === 'ok') {
      output += `✅ ${tool.name}\n`;
      if (tool.type === 'cli') {
        output += `   Configured via CLI - Ready\n\n`;
      } else {
        output += `   ${tool.path} - Configured and ready\n\n`;
      }
    } else if (tool.status === 'error') {
      output += `❌ ${tool.name}\n`;
      if (tool.path) {
        output += `   ${tool.path}\n`;
      }
      if (tool.issues && tool.issues.length > 0) {
        tool.issues.forEach(issue => {
          output += `   Issue: ${issue}\n`;
        });
      }
      if (tool.suggestions && tool.suggestions.length > 0) {
        tool.suggestions.forEach(suggestion => {
          output += `   Fix: ${suggestion}\n`;
        });
      }
      output += '\n';
    } else if (tool.status === 'info') {
      output += `⚪ ${tool.name}\n`;
      if (tool.issues && tool.issues.length > 0) {
        tool.issues.forEach(issue => {
          output += `   ${issue}\n`;
        });
      }
      output += '\n';
    } else if (tool.status === 'warning') {
      output += `⚠️  ${tool.name}\n`;
      if (tool.path) {
        output += `   ${tool.path}\n`;
      }
      if (tool.issues && tool.issues.length > 0) {
        tool.issues.forEach(issue => {
          output += `   Issue: ${issue}\n`;
        });
      }
      if (tool.suggestions && tool.suggestions.length > 0) {
        tool.suggestions.forEach(suggestion => {
          output += `   Suggestion: ${suggestion}\n`;
        });
      }
      output += '\n';
    }
  });

  if (report.binary) {
    if (report.binary.ok) {
      output += `✅ Binary Check\n`;
      output += `   Gasoline binary found at ${report.binary.path}\n`;
      if (report.binary.version) {
        output += `   Version: ${report.binary.version}\n`;
      }
    } else {
      output += `❌ Binary Check\n`;
      output += `   ${report.binary.error}\n`;
    }
    output += '\n';
  }

  if (report.port) {
    if (report.port.available) {
      output += `✅ Port ${report.port.port}\n`;
      output += `   Default port is available\n`;
    } else {
      output += `⚠️  Port ${report.port.port}\n`;
      output += `   ${report.port.error}\n`;
      output += `   Suggestion: Use --port ${report.port.port + 1} or kill the process using the port\n`;
    }
  }

  // Legacy path warnings
  if (report.legacyWarnings && report.legacyWarnings.length > 0) {
    output += '\n⚠️  Legacy Configs Found:\n';
    report.legacyWarnings.forEach(w => {
      output += `   ${w.description}: ${w.path}\n`;
      output += `   This path is no longer used. You can safely remove the gasoline entry.\n`;
    });
  }

  output += `\n${report.summary}\n`;
  return output;
}

/**
 * Format uninstall result
 */
function uninstallResult(result) {
  let output = '';

  if (result.removed.length > 0) {
    output += `✅ Removed from ${result.removed.length} client${result.removed.length === 1 ? '' : 's'}:\n`;
    result.removed.forEach(entry => {
      if (entry.method === 'cli') {
        output += `   ✅ ${entry.name} (via CLI)\n`;
      } else {
        output += `   ✅ ${entry.name} (removed from ${entry.path})\n`;
      }
    });
  } else {
    output += `ℹ️  Gasoline not configured in any clients\n`;
  }

  if (result.notConfigured && result.notConfigured.length > 0) {
    output += `\nℹ️  Not configured in: ${result.notConfigured.join(', ')}\n`;
  }

  if (result.errors && result.errors.length > 0) {
    output += '\n❌ Errors:\n';
    result.errors.forEach(err => {
      output += `   ${err}\n`;
    });
  }

  return output;
}

module.exports = {
  success,
  error,
  warning,
  info,
  jsonDiff,
  installResult,
  diagnosticReport,
  uninstallResult,
};

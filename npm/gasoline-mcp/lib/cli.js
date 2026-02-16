// cli.js — CLI command handler for gasoline-mcp management commands.
// Invoked by the shell wrapper when --install, --config, --doctor, etc. are passed.

const os = require('os');
const config = require('./config');
const output = require('./output');
const install = require('./install');
const skills = require('./skills');
const doctor = require('./doctor');
const uninstall = require('./uninstall');
const {
  EnvWithoutInstallError,
  ForAllWithoutInstallError,
  InvalidEnvFormatError,
} = require('./errors');

// Write a JSON-RPC error to stdout so MCP clients get a clean protocol-level error
function writeMcpError(message) {
  const errorResponse = JSON.stringify({
    jsonrpc: '2.0',
    id: null,
    error: {
      code: -32603,
      message: message,
      data: { isError: true },
    },
  });
  process.stdout.write(errorResponse + '\n');
}

function showConfigCommand() {
  const mcp = install.generateDefaultConfig();
  console.log('📋 Gasoline MCP Configuration\n');
  console.log('Add this to your Claude settings file:\n');
  console.log(JSON.stringify(mcp, null, 2));
  console.log('\n📍 Configuration Locations:');
  console.log('');
  console.log('Claude Code (VSCode):');
  console.log('  ~/.vscode/claude.mcp.json');
  console.log('');
  console.log('Claude:');
  console.log(`  ${os.platform() === 'win32' ? '%USERPROFILE%' : '~'}/.claude.json`);
  console.log('');
  console.log('Cursor:');
  console.log('  ~/.cursor/mcp.json');
  console.log('');
  console.log('Codeium:');
  console.log('  ~/.codeium/mcp.json');
  process.exit(0);
}

async function installCommand(options) {
  try {
    const result = install.executeInstall(options);

    if (result.success) {
      if (options.dryRun) {
        console.log(`ℹ️  Dry run: No files will be written\n`);
      }
      console.log(
        output.installResult({
          updated: result.updated,
          total: result.total,
          errors: result.errors,
          notFound: result.updated.length > 0
            ? []
            : config.getConfigCandidates().map(config.getToolNameFromPath),
        })
      );
      if (!options.dryRun) {
        const skillInstall = await skills.installBundledSkills({
          verbose: options.verbose,
          skillsRepo: options.skillsRepo,
          skillsRef: options.skillsRef,
          skillsPath: options.skillsPath,
          skillsManifestPath: options.skillsManifestPath,
          skillsDir: options.skillsDir,
          skillsNoFallback: options.skillsNoFallback,
        });
        if (!skillInstall.skipped) {
          const s = skillInstall.summary;
          console.log(
            `🧠 Skills installed (${skillInstall.agents.join(', ')} / ${skillInstall.scope}): ` +
            `source=${skillInstall.source}, created=${s.created}, updated=${s.updated}, unchanged=${s.unchanged}, ` +
            `skipped=${s.skipped_user_owned}, legacy_removed=${s.legacy_removed}, errors=${s.errors}`
          );
          for (const warning of skillInstall.warnings || []) {
            console.warn(`⚠️  ${warning}`);
          }
        }
        console.log('✨ Gasoline MCP is ready to use!');
      }
      process.exit(0);
    } else {
      console.error(output.error('Installation failed'));
      result.errors.forEach(err => {
        console.error(`  ${err.name}: ${err.message}`);
        if (err.recovery) {
          console.error(`  Recovery: ${err.recovery}`);
        }
      });
      process.exit(1);
    }
  } catch (err) {
    console.error(err.format ? err.format() : `Error: ${err.message}`);
    process.exit(1);
  }
}

function doctorCommand(verbose) {
  try {
    const report = doctor.runDiagnostics(verbose);
    console.log(output.diagnosticReport(report));
    process.exit(0);
  } catch (err) {
    console.error(err.format ? err.format() : `Error: ${err.message}`);
    process.exit(1);
  }
}

function uninstallCommand(dryRun, verbose) {
  try {
    const result = uninstall.executeUninstall({ dryRun, verbose });

    if (dryRun) {
      console.log(`ℹ️  Dry run: No files will be modified\n`);
    }
    console.log(output.uninstallResult(result));
    process.exit(0);
  } catch (err) {
    console.error(err.format ? err.format() : `Error: ${err.message}`);
    process.exit(1);
  }
}

function showHelp() {
  console.log('Gasoline MCP Server\n');
  console.log('Usage: gasoline-mcp [command] [options]\n');
  console.log('Commands:');
  console.log('  --config, -c          Show MCP configuration and where to put it');
  console.log('  --install, -i         Auto-install to your AI assistant config');
  console.log('  --doctor              Run diagnostics on installed configs');
  console.log('  --uninstall           Remove Gasoline from configs');
  console.log('  --help, -h            Show this help message\n');
  console.log('Options (with --install):');
  console.log('  --dry-run             Preview changes without writing files');
  console.log('  --for-all             Install to all 4 tools (Claude, VSCode, Cursor, Codeium)');
  console.log('  --env KEY=VALUE       Add environment variables to config (multiple allowed)');
  console.log('  --skills-repo VALUE   Skill source repo (owner/repo or GitHub URL)');
  console.log('  --skills-ref VALUE    Git ref when loading skills from --skills-repo');
  console.log('  --skills-path VALUE   Repo path containing skill folders (optional)');
  console.log('  --skills-manifest VALUE Repo path to skills manifest JSON (for example skills/skills.json)');
  console.log('  --skills-dir PATH     Local skills directory override');
  console.log('  --skills-no-fallback  Do not fall back to bundled skills if remote fetch fails');
  console.log('  --verbose             Show detailed operation logs\n');
  console.log('Options (with --uninstall):');
  console.log('  --dry-run             Preview changes without writing files');
  console.log('  --verbose             Show detailed operation logs\n');
  console.log('Examples:');
  console.log('  gasoline-mcp --install                # Install to first matching tool');
  console.log('  gasoline-mcp --install --for-all      # Install to all 4 tools');
  console.log('  gasoline-mcp --install --dry-run      # Preview without changes');
  console.log('  gasoline-mcp --install --env DEBUG=1  # Install with env vars');
  console.log('  gasoline-mcp --install --skills-repo brennhill/gasoline-skills');
  console.log('  gasoline-mcp --install --skills-repo https://github.com/brennhill/gasoline-skills/tree/main/skills');
  console.log('  gasoline-mcp --doctor                 # Check config health');
  console.log('  gasoline-mcp --uninstall              # Remove from all tools\n');
  process.exit(0);
}

function parseSingleValueFlag(args, flagName) {
  const index = args.indexOf(flagName);
  if (index === -1) return null;
  const next = args[index + 1];
  if (!next || next.startsWith('--')) {
    throw new Error(`Missing value for ${flagName}`);
  }
  return next;
}

function parseSkillInstallOptions(args) {
  return {
    skillsRepo: parseSingleValueFlag(args, '--skills-repo'),
    skillsRef: parseSingleValueFlag(args, '--skills-ref'),
    skillsPath: parseSingleValueFlag(args, '--skills-path'),
    skillsManifestPath: parseSingleValueFlag(args, '--skills-manifest'),
    skillsDir: parseSingleValueFlag(args, '--skills-dir'),
    skillsNoFallback: args.includes('--skills-no-fallback'),
  };
}

async function main() {
  const args = process.argv.slice(2);
  const verbose = args.includes('--verbose');
  const dryRun = args.includes('--dry-run');

  // Config command
  if (args.includes('--config') || args.includes('-c')) {
    showConfigCommand();
    return;
  }

  // Install command
  if (args.includes('--install') || args.includes('-i')) {
    if (args.includes('--env') && !(args.includes('--install') || args.includes('-i'))) {
      console.error(output.error('--env only works with --install', 'Usage: gasoline-mcp --install --env KEY=VALUE'));
      process.exit(1);
    }
    if (args.includes('--for-all') && !(args.includes('--install') || args.includes('-i'))) {
      console.error(output.error('--for-all only works with --install', 'Usage: gasoline-mcp --install --for-all'));
      process.exit(1);
    }

    const envVars = {};
    for (let i = 0; i < args.length; i++) {
      if (args[i] === '--env' && i + 1 < args.length) {
        try {
          const parsed = config.parseEnvVar(args[i + 1]);
          envVars[parsed.key] = parsed.value;
        } catch (err) {
          console.error(output.error(err.message, err.recovery));
          process.exit(1);
        }
      }
    }

    let skillOptions = {};
    try {
      skillOptions = parseSkillInstallOptions(args);
    } catch (err) {
      console.error(output.error(err.message, 'Run gasoline-mcp --help for usage.'));
      process.exit(1);
    }

    const options = {
      dryRun,
      forAll: args.includes('--for-all'),
      envVars,
      verbose,
      ...skillOptions,
    };
    await installCommand(options);
    return;
  }

  // Doctor command
  if (args.includes('--doctor')) {
    doctorCommand(verbose);
    return;
  }

  // Uninstall command
  if (args.includes('--uninstall')) {
    uninstallCommand(dryRun, verbose);
    return;
  }

  // Help command
  if (args.includes('--help') || args.includes('-h')) {
    showHelp();
    return;
  }

  // Version (print and exit)
  if (args.includes('--version') || args.includes('-v')) {
    const pkg = require('../package.json');
    console.log(`gasoline-mcp v${pkg.version}`);
    process.exit(0);
  }

  // If we get here with no recognized flags, show help
  console.error('Unknown command. Run gasoline-mcp --help for usage.');
  process.exit(1);
}

main().catch((err) => {
  console.error(err.format ? err.format() : `Error: ${err.message}`);
  process.exit(1);
});

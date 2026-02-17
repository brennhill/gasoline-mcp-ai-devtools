// cli.js — CLI command handler for gasoline-mcp management commands.
// Invoked by the shell wrapper when --install, --config, --doctor, etc. are passed.

const os = require('os');
const config = require('./config');
const output = require('./output');
const install = require('./install');
const skills = require('./skills');
const doctor = require('./doctor');
const uninstall = require('./uninstall');

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
  console.log('Add this to your AI assistant settings:\n');
  console.log(JSON.stringify(mcp, null, 2));
  console.log('\n📍 Supported Clients:\n');

  for (const def of config.CLIENT_DEFINITIONS) {
    const detected = config.isClientInstalled(def);
    const icon = detected ? '✅' : '⚪';

    if (def.type === 'cli') {
      console.log(`${icon} ${def.name} (via ${def.detectCommand} CLI)`);
    } else {
      const cfgPath = config.getClientConfigPath(def);
      if (cfgPath) {
        console.log(`${icon} ${def.name}`);
        console.log(`   ${cfgPath}`);
      } else {
        console.log(`⚪ ${def.name} (not available on this platform)`);
      }
    }
    console.log('');
  }

  console.log('Run: gasoline-mcp --install   (auto-installs to all detected clients)');
  process.exit(0);
}

async function installCommand(options) {
  try {
    const result = install.executeInstall(options);

    if (result.success) {
      if (options.dryRun) {
        console.log(`ℹ️  Dry run: No files will be written\n`);
      }
      console.log(output.installResult(result));
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
        if (typeof err === 'string') {
          console.error(`  ${err}`);
        } else {
          console.error(`  ${err.name}: ${err.message}`);
          if (err.recovery) {
            console.error(`  Recovery: ${err.recovery}`);
          }
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
  console.log('  --config, -c          Show MCP configuration and detected clients');
  console.log('  --install, -i         Auto-install to all detected AI clients');
  console.log('  --doctor              Run diagnostics on installed configs');
  console.log('  --uninstall           Remove Gasoline from all clients');
  console.log('  --help, -h            Show this help message\n');
  console.log('Supported clients:');
  console.log('  Claude Code           via claude CLI (mcp add-json)');
  console.log('  Claude Desktop        config file');
  console.log('  Cursor                config file');
  console.log('  Windsurf              config file');
  console.log('  VS Code               config file\n');
  console.log('Options (with --install):');
  console.log('  --dry-run             Preview changes without writing');
  console.log('  --env KEY=VALUE       Add environment variables to config (multiple allowed)');
  console.log('  --skills-repo VALUE   Skill source repo (owner/repo or GitHub URL)');
  console.log('  --skills-ref VALUE    Git ref when loading skills from --skills-repo');
  console.log('  --skills-path VALUE   Repo path containing skill folders (optional)');
  console.log('  --skills-manifest VALUE Repo path to skills manifest JSON (for example skills/skills.json)');
  console.log('  --skills-dir PATH     Local skills directory override');
  console.log('  --skills-no-fallback  Do not fall back to bundled skills if remote fetch fails');
  console.log('  --verbose             Show detailed operation logs\n');
  console.log('Options (with --uninstall):');
  console.log('  --dry-run             Preview changes without writing');
  console.log('  --verbose             Show detailed operation logs\n');
  console.log('Examples:');
  console.log('  gasoline-mcp --install                # Install to all detected clients');
  console.log('  gasoline-mcp --install --dry-run      # Preview without changes');
  console.log('  gasoline-mcp --install --env DEBUG=1  # Install with env vars');
  console.log('  gasoline-mcp --install --skills-repo brennhill/gasoline-skills');
  console.log('  gasoline-mcp --config                 # Show config and detected clients');
  console.log('  gasoline-mcp --doctor                 # Check config health');
  console.log('  gasoline-mcp --uninstall              # Remove from all clients\n');
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

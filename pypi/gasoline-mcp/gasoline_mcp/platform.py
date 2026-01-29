"""Platform detection and binary execution for Gasoline MCP."""

import sys
import platform
import subprocess
import os


def get_platform():
    """Detect the current platform and return the platform identifier."""
    os_name = sys.platform
    machine = platform.machine().lower()

    if os_name == "darwin":
        if machine == "arm64":
            return "darwin-arm64"
        else:
            return "darwin-x64"
    elif os_name.startswith("linux"):
        if "aarch64" in machine or "arm64" in machine:
            return "linux-arm64"
        else:
            return "linux-x64"
    elif os_name == "win32":
        return "win32-x64"
    else:
        raise RuntimeError(f"Unsupported platform: {os_name} {machine}")


def get_binary_path():
    """Get the path to the platform-specific Gasoline binary."""
    platform_name = get_platform()
    package_name = f"gasoline_mcp_{platform_name.replace('-', '_')}"

    try:
        import importlib.util
        spec = importlib.util.find_spec(package_name)
        if spec and spec.origin:
            binary_name = "gasoline.exe" if sys.platform == "win32" else "gasoline"
            binary_path = os.path.join(os.path.dirname(spec.origin), binary_name)

            if os.path.exists(binary_path):
                return binary_path
    except ImportError:
        pass

    # If we get here, the platform-specific package isn't installed
    raise RuntimeError(
        f"Platform-specific package not found for {platform_name}.\n"
        f"Install with: pip install gasoline-mcp[{platform_name}]\n"
        f"Or for automatic detection: pip install gasoline-mcp && pip install gasoline-mcp[{platform_name}]"
    )


def show_help():
    """Show help message."""
    print("""Gasoline MCP Server

Usage: gasoline-mcp [command] [options]

Commands:
  --config, -c          Show MCP configuration and where to put it
  --install, -i         Auto-install to your AI assistant config
  --doctor              Run diagnostics on installed configs
  --uninstall           Remove Gasoline from configs
  --help, -h            Show this help message

Options (with --install):
  --dry-run             Preview changes without writing files
  --for-all             Install to all 4 tools (Claude, VSCode, Cursor, Codeium)
  --env KEY=VALUE       Add environment variables to config (multiple allowed)
  --verbose             Show detailed operation logs

Options (with --uninstall):
  --dry-run             Preview changes without writing files
  --verbose             Show detailed operation logs

Examples:
  gasoline-mcp --install                # Install to first matching tool
  gasoline-mcp --install --for-all      # Install to all 4 tools
  gasoline-mcp --install --dry-run      # Preview without changes
  gasoline-mcp --install --env DEBUG=1  # Install with env vars
  gasoline-mcp --doctor                 # Check config health
  gasoline-mcp --uninstall              # Remove from all tools
""")
    sys.exit(0)


def show_config():
    """Show configuration information."""
    from . import install, output
    import json

    cfg = install.generate_default_config()

    print("📋 Gasoline MCP Configuration\n")
    print("Add this to your AI assistant settings file:\n")
    print(json.dumps(cfg, indent=2))
    print("\n📍 Configuration Locations:")
    print("")
    print("Claude Code (VSCode):")
    print("  ~/.vscode/claude.mcp.json")
    print("")
    print("Claude Desktop App:")
    print("  ~/.claude/claude.mcp.json")
    print("")
    print("Cursor:")
    print("  ~/.cursor/mcp.json")
    print("")
    print("Codeium:")
    print("  ~/.codeium/mcp.json")
    sys.exit(0)


def run_install(args):
    """Run install command."""
    from . import install, output, config, errors

    # Parse options
    dry_run = "--dry-run" in args
    for_all = "--for-all" in args
    verbose = "--verbose" in args

    # Parse env vars
    env_vars = {}
    for i, arg in enumerate(args):
        if arg == "--env" and i + 1 < len(args):
            try:
                parsed = config.parse_env_var(args[i + 1])
                env_vars[parsed["key"]] = parsed["value"]
            except errors.GasolineError as e:
                print(output.error(e.message, e.recovery))
                sys.exit(1)

    options = {
        "dryRun": dry_run,
        "forAll": for_all,
        "envVars": env_vars,
        "verbose": verbose,
    }

    try:
        result = install.execute_install(options)

        if result["success"]:
            if dry_run:
                print("ℹ️  Dry run: No files will be written\n")
            print(output.install_result({
                "updated": result["updated"],
                "total": result["total"],
                "errors": result["errors"],
                "notFound": [],
            }))
            if not dry_run:
                print("✨ Gasoline MCP is ready to use!")
            sys.exit(0)
        else:
            print(output.error("Installation failed"))
            for err in result["errors"]:
                print(f"  {err['name']}: {err['message']}")
                if err.get("recovery"):
                    print(f"  Recovery: {err['recovery']}")
            sys.exit(1)
    except Exception as e:
        error_msg = str(e)
        if hasattr(e, "format"):
            print(e.format())
        else:
            print(f"Error: {error_msg}")
        sys.exit(1)


def run_doctor(args):
    """Run doctor command."""
    from . import doctor, output

    verbose = "--verbose" in args

    try:
        report = doctor.run_diagnostics(verbose)
        print(output.diagnostic_report(report))
        sys.exit(0)
    except Exception as e:
        error_msg = str(e)
        if hasattr(e, "format"):
            print(e.format())
        else:
            print(f"Error: {error_msg}")
        sys.exit(1)


def run_uninstall(args):
    """Run uninstall command."""
    from . import uninstall, output

    dry_run = "--dry-run" in args
    verbose = "--verbose" in args

    try:
        result = uninstall.execute_uninstall({
            "dryRun": dry_run,
            "verbose": verbose,
        })

        if dry_run:
            print("ℹ️  Dry run: No files will be modified\n")

        print(output.uninstall_result(result))
        sys.exit(0)
    except Exception as e:
        error_msg = str(e)
        if hasattr(e, "format"):
            print(e.format())
        else:
            print(f"Error: {error_msg}")
        sys.exit(1)


def run():
    """Run the Gasoline MCP CLI or binary."""
    from . import output

    args = sys.argv[1:]

    # Config command
    if "--config" in args or "-c" in args:
        show_config()

    # Install command
    if "--install" in args or "-i" in args:
        run_install(args)

    # Doctor command
    if "--doctor" in args:
        run_doctor(args)

    # Uninstall command
    if "--uninstall" in args:
        run_uninstall(args)

    # Help command
    if "--help" in args or "-h" in args:
        show_help()

    # No config command, run the binary
    binary = get_binary_path()

    # Make sure binary is executable
    if not os.access(binary, os.X_OK):
        os.chmod(binary, 0o755)

    # Execute the binary, passing through all arguments
    try:
        result = subprocess.run([binary] + args)
        sys.exit(result.returncode)
    except KeyboardInterrupt:
        sys.exit(130)  # Standard exit code for SIGINT
    except Exception as e:
        print(f"Error running Gasoline: {e}", file=sys.stderr)
        sys.exit(1)

# pylint: disable=duplicate-code
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
        return "darwin-x64"
    if os_name.startswith("linux"):
        if "aarch64" in machine or "arm64" in machine:
            return "linux-arm64"
        return "linux-x64"
    if os_name == "win32":
        return "win32-x64"
    raise RuntimeError(f"Unsupported platform: {os_name} {machine}")


def get_binary_path():
    """Get the path to the platform-specific Gasoline binary."""
    platform_name = get_platform()
    package_name = f"gasoline_mcp_{platform_name.replace('-', '_')}"

    try:
        import importlib.util  # pylint: disable=import-outside-toplevel
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
        f"Platform-specific binary not found for {platform_name}.\n"
        f"Try reinstalling: pip install --force-reinstall gasoline-mcp\n"
        f"Or install the binary directly: pip install gasoline-mcp-{platform_name}"
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
    from . import install  # pylint: disable=import-outside-toplevel
    import json  # pylint: disable=import-outside-toplevel

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


def _parse_env_args(args):
    """Parse --env KEY=VALUE arguments from args list."""
    from . import config, errors, output as out  # pylint: disable=import-outside-toplevel

    env_vars = {}
    for i, arg in enumerate(args):
        if arg == "--env" and i + 1 < len(args):
            try:
                parsed = config.parse_env_var(args[i + 1])
                env_vars[parsed["key"]] = parsed["value"]
            except errors.GasolineError as e:
                print(out.error(e.message, e.recovery))
                sys.exit(1)
    return env_vars


def _print_install_success(result, dry_run):
    """Print successful install result and exit."""
    from . import output  # pylint: disable=import-outside-toplevel

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


def _print_install_failure(result):
    """Print install failure details and exit."""
    from . import output  # pylint: disable=import-outside-toplevel

    print(output.error("Installation failed"))
    for err in result["errors"]:
        print(f"  {err['name']}: {err['message']}")
        if err.get("recovery"):
            print(f"  Recovery: {err['recovery']}")
    sys.exit(1)


def run_install(args):
    """Run install command."""
    from . import install  # pylint: disable=import-outside-toplevel

    options = {
        "dryRun": "--dry-run" in args,
        "forAll": "--for-all" in args,
        "envVars": _parse_env_args(args),
        "verbose": "--verbose" in args,
    }

    try:
        result = install.execute_install(options)
        if result["success"]:
            _print_install_success(result, options["dryRun"])
        else:
            _print_install_failure(result)
    except (OSError, ValueError) as e:
        if hasattr(e, "format"):
            print(e.format())
        else:
            print(f"Error: {e}")
        sys.exit(1)


def run_doctor(args):
    """Run doctor command."""
    from . import doctor, output  # pylint: disable=import-outside-toplevel

    verbose = "--verbose" in args

    try:
        report = doctor.run_diagnostics(verbose, get_binary_path=get_binary_path)
        print(output.diagnostic_report(report))
        sys.exit(0)
    except (OSError, RuntimeError) as e:
        if hasattr(e, "format"):
            print(e.format())
        else:
            print(f"Error: {e}")
        sys.exit(1)


def run_uninstall(args):
    """Run uninstall command."""
    from . import uninstall, output  # pylint: disable=import-outside-toplevel

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
    except (OSError, ValueError) as e:
        if hasattr(e, "format"):
            print(e.format())
        else:
            print(f"Error: {e}")
        sys.exit(1)


def _cleanup_windows():
    """Kill gasoline processes on Windows. Returns list of killed PIDs."""
    import re  # pylint: disable=import-outside-toplevel

    killed = []
    result = subprocess.run(
        ["tasklist", "/FI", "IMAGENAME eq gasoline*", "/FO", "CSV"],
        capture_output=True, text=True, check=False,
    )
    if not result.stdout:
        return killed

    for line in result.stdout.split('\n')[1:]:
        match = re.match(r'"gasoline[^"]*","(\d+)"', line)
        if not match:
            continue
        pid = match.group(1)
        subprocess.run(["taskkill", "/F", "/PID", pid], capture_output=True, check=False)
        killed.append(pid)
    return killed


def _kill_pids_on_port(port):
    """Kill processes listening on a given port. Returns list of killed PIDs."""
    killed = []
    result = subprocess.run(
        ["lsof", "-ti", f":{port}"],
        capture_output=True, text=True, check=False,
    )
    for pid in (result.stdout or "").strip().split('\n'):
        if pid:
            subprocess.run(["kill", "-9", pid], capture_output=True, check=False)
            killed.append(pid)
    return killed


def _cleanup_unix():
    """Kill gasoline processes on Unix. Returns list of killed PIDs."""
    subprocess.run(["pkill", "-f", "gasoline"], capture_output=True, check=False)
    killed = []
    for port in ["7890", "17890"]:
        killed.extend(_kill_pids_on_port(port))
    return killed


def cleanup_old_processes():
    """Kill all running gasoline processes to ensure clean upgrade."""
    try:
        if sys.platform == "win32":
            return _cleanup_windows()
        return _cleanup_unix()
    except (OSError, subprocess.SubprocessError):
        return []


def verify_version(binary_path, expected_version):
    """Verify the installed version matches expected."""
    try:
        result = subprocess.run(
            [binary_path, "--version"],
            capture_output=True,
            text=True,
            timeout=5,
            check=False
        )
        if result.stdout:
            version = result.stdout.strip()
            if expected_version in version:
                print(f"✓ Verified gasoline version: {version}")
                return True
            print(f"Warning: Expected version {expected_version}, got: {version}")
    except (OSError, subprocess.SubprocessError) as e:
        print(f"Could not verify version: {e}")
    return False


def _dispatch_cli_command(args):
    """Dispatch CLI subcommands. Returns True if a command was handled."""
    commands = {
        "--config": show_config,
        "-c": show_config,
        "--install": lambda: run_install(args),
        "-i": lambda: run_install(args),
        "--doctor": lambda: run_doctor(args),
        "--uninstall": lambda: run_uninstall(args),
        "--help": show_help,
        "-h": show_help,
    }
    for flag, handler in commands.items():
        if flag in args:
            handler()
            return True
    return False


def _run_binary(args):
    """Execute the gasoline binary, passing through all arguments."""
    cleanup_old_processes()
    binary = get_binary_path()

    if not os.access(binary, os.X_OK):
        os.chmod(binary, 0o755)  # nosemgrep: python.lang.security.audit.insecure-file-permissions.insecure-file-permissions

    try:
        result = subprocess.run([binary] + args, check=False)
        sys.exit(result.returncode)
    except KeyboardInterrupt:
        sys.exit(130)
    except (OSError, subprocess.SubprocessError) as e:
        print(f"Error running Gasoline: {e}", file=sys.stderr)
        sys.exit(1)


def run():
    """Run the Gasoline MCP CLI or binary."""
    args = sys.argv[1:]
    if not _dispatch_cli_command(args):
        _run_binary(args)

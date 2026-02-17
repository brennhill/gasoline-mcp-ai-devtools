# pylint: disable=duplicate-code
"""Platform detection and binary execution for Gasoline MCP."""

import sys
import platform
import subprocess
import os

KNOWN_PORTS = [17890] + list(range(7890, 7911))


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
  --config, -c          Show MCP configuration and detected clients
  --install, -i         Auto-install to all detected AI clients
  --doctor              Run diagnostics on installed configs
  --uninstall           Remove Gasoline from all clients
  --help, -h            Show this help message

Supported clients:
  Claude Code           via claude CLI (mcp add-json)
  Claude Desktop        config file
  Cursor                config file
  Windsurf              config file
  VS Code               config file

Options (with --install):
  --dry-run             Preview changes without writing
  --env KEY=VALUE       Add environment variables to config (multiple allowed)
  --verbose             Show detailed operation logs

Options (with --uninstall):
  --dry-run             Preview changes without writing
  --verbose             Show detailed operation logs

Examples:
  gasoline-mcp --install                # Install to all detected clients
  gasoline-mcp --install --dry-run      # Preview without changes
  gasoline-mcp --install --env DEBUG=1  # Install with env vars
  gasoline-mcp --config                 # Show config and detected clients
  gasoline-mcp --doctor                 # Check config health
  gasoline-mcp --uninstall              # Remove from all clients
""")
    sys.exit(0)


def show_config():
    """Show configuration information."""
    from . import install  # pylint: disable=import-outside-toplevel
    from . import config as cfg_mod  # pylint: disable=import-outside-toplevel
    import json  # pylint: disable=import-outside-toplevel

    mcp = install.generate_default_config()

    print("📋 Gasoline MCP Configuration\n")
    print("Add this to your AI assistant settings:\n")
    print(json.dumps(mcp, indent=2))
    print("\n📍 Supported Clients:\n")

    for definition in cfg_mod.CLIENT_DEFINITIONS:
        detected = cfg_mod.is_client_installed(definition)
        icon = "✅" if detected else "⚪"

        if definition["type"] == "cli":
            print(f"{icon} {definition['name']} (via {definition['detectCommand']} CLI)")
        else:
            cfg_path = cfg_mod.get_client_config_path(definition)
            if cfg_path:
                print(f"{icon} {definition['name']}")
                print(f"   {cfg_path}")
            else:
                print(f"⚪ {definition['name']} (not available on this platform)")
        print("")

    print("Run: gasoline-mcp --install   (auto-installs to all detected clients)")
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
    print(output.install_result(result))
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
    killed = []
    self_pid = os.getpid()
    parent_pid = os.getppid()

    def _should_skip(cmdline):
        return "python" in cmdline or "node" in cmdline or "npm" in cmdline

    for pattern in ["gasoline-mcp", "dev-console", "gasoline"]:
        result = subprocess.run(
            ["pgrep", "-af", pattern],
            capture_output=True,
            text=True,
            check=False,
        )
        for line in (result.stdout or "").strip().split("\n"):
            if not line:
                continue
            parts = line.split(maxsplit=1)
            if not parts:
                continue
            try:
                pid = int(parts[0])
            except ValueError:
                continue
            cmdline = parts[1] if len(parts) > 1 else ""
            if pid <= 1 or pid in (self_pid, parent_pid):
                continue
            if _should_skip(cmdline):
                continue
            subprocess.run(["kill", "-9", str(pid)], capture_output=True, check=False)
            killed.append(str(pid))
    for port in KNOWN_PORTS:
        killed.extend(_kill_pids_on_port(str(port)))
    return killed


def _normalize_path_for_set(path):
    """Normalize path for stable dedupe across case-insensitive filesystems."""
    return os.path.normcase(os.path.abspath(path))


def _candidate_home_dirs():
    """Return candidate home directories in priority order."""
    homes = []
    for value in [os.environ.get("HOME"), os.environ.get("USERPROFILE")]:
        if value:
            homes.append(value)

    home_drive = os.environ.get("HOMEDRIVE")
    home_path = os.environ.get("HOMEPATH")
    if home_drive and home_path:
        homes.append(os.path.join(home_drive, home_path))

    expanded = os.path.expanduser("~")
    if expanded:
        homes.append(expanded)

    deduped = []
    seen = set()
    for home in homes:
        key = _normalize_path_for_set(home)
        if key in seen:
            continue
        seen.add(key)
        deduped.append(home)
    return deduped


def _pid_roots():
    """Return all roots that may contain gasoline pid files."""
    roots = []
    seen = set()

    for home in _candidate_home_dirs():
        run_root = os.path.join(home, ".gasoline", "run")
        key = _normalize_path_for_set(run_root)
        if key not in seen:
            seen.add(key)
            roots.append(run_root)

    xdg_state_home = os.environ.get("XDG_STATE_HOME")
    if xdg_state_home:
        xdg_root = os.path.join(xdg_state_home, "gasoline", "run")
        key = _normalize_path_for_set(xdg_root)
        if key not in seen:
            seen.add(key)
            roots.append(xdg_root)

    return roots


def _best_effort_remove(path):
    """Remove a file path with fallback chmod retry for Windows read-only files."""
    try:
        os.remove(path)
        return True
    except FileNotFoundError:
        return False
    except PermissionError:
        try:
            os.chmod(path, 0o666)
            os.remove(path)
            return True
        except OSError:
            return False
    except OSError:
        return False


def _cleanup_pid_files():
    """Remove modern and legacy PID files for known ports."""
    homes = _candidate_home_dirs()
    roots = _pid_roots()

    for root in roots:
        try:
            for entry in os.listdir(root):
                if entry.startswith("gasoline-") and entry.endswith(".pid"):
                    _best_effort_remove(os.path.join(root, entry))
                if entry.startswith("dev-console-") and entry.endswith(".pid"):
                    _best_effort_remove(os.path.join(root, entry))
            try:
                if not os.listdir(root):
                    os.rmdir(root)
            except OSError:
                pass
        except OSError:
            pass

    for home in homes:
        try:
            for entry in os.listdir(home):
                if entry.startswith(".gasoline-") and entry.endswith(".pid"):
                    _best_effort_remove(os.path.join(home, entry))
                if entry.startswith(".dev-console-") and entry.endswith(".pid"):
                    _best_effort_remove(os.path.join(home, entry))
        except OSError:
            pass

    for root in roots:
        try:
            for entry in os.listdir(root):
                if entry.startswith("gasoline-") and entry.endswith(".pid"):
                    try:
                        os.remove(os.path.join(root, entry))
                    except OSError:
                        pass
            try:
                if not os.listdir(root):
                    os.rmdir(root)
            except OSError:
                pass
        except OSError:
            pass

    try:
        for entry in os.listdir(home):
            if entry.startswith(".gasoline-") and entry.endswith(".pid"):
                try:
                    os.remove(os.path.join(home, entry))
                except OSError:
                    pass
    except OSError:
        pass

    for port in KNOWN_PORTS:
        for root in roots:
            pid_path = os.path.join(root, f"gasoline-{port}.pid")
            _best_effort_remove(pid_path)
            _best_effort_remove(os.path.join(root, f"dev-console-{port}.pid"))

        for home in homes:
            _best_effort_remove(os.path.join(home, f".gasoline-{port}.pid"))
            _best_effort_remove(os.path.join(home, f".dev-console-{port}.pid"))


def cleanup_old_processes():
    """Kill all running gasoline processes to ensure clean upgrade."""
    try:
        if sys.platform == "win32":
            killed = _cleanup_windows()
        else:
            killed = _cleanup_unix()
        _cleanup_pid_files()
        return killed
    except (OSError, subprocess.SubprocessError):
        _cleanup_pid_files()
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

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


def run():
    """Run the Gasoline MCP binary with the provided arguments."""
    binary = get_binary_path()

    # Make sure binary is executable
    if not os.access(binary, os.X_OK):
        os.chmod(binary, 0o755)

    # Execute the binary, passing through all arguments
    try:
        result = subprocess.run([binary] + sys.argv[1:])
        sys.exit(result.returncode)
    except KeyboardInterrupt:
        sys.exit(130)  # Standard exit code for SIGINT
    except Exception as e:
        print(f"Error running Gasoline: {e}", file=sys.stderr)
        sys.exit(1)

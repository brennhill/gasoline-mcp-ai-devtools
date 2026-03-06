# PyPI Distribution Structure

## Approach: Platform-Specific Wheels (same as NPM optionalDependencies)

### Package Structure:
```
pypi/
├── gasoline-mcp/              # Main package
│   ├── pyproject.toml
│   ├── setup.py
│   ├── gasoline_mcp/
│   │   ├── __init__.py
│   │   ├── __main__.py        # Entry point
│   │   └── platform.py        # Platform detection
│   └── README.md
├── gasoline-mcp-darwin-arm64/  # Platform-specific packages
├── gasoline-mcp-darwin-x64/
├── gasoline-mcp-linux-arm64/
├── gasoline-mcp-linux-x64/
├── gasoline-mcp-win32-x64/
└── publish.py                  # Publish script
```

### How it Works:
1. **Main package** (`gasoline-mcp`):
   - No binary, just Python wrapper
   - Detects platform at runtime
   - Imports platform-specific package
   - Entry point: `gasoline-mcp` command

2. **Platform packages** (e.g., `gasoline-mcp-darwin-arm64`):
   - Contains the Go binary for that platform
   - Listed as extras in main package

3. **Installation**:
   ```bash
   pip install gasoline-mcp
   # Automatically installs correct platform-specific package
   ```

### Benefits:
- Users just run `pip install gasoline-mcp`
- Same UX as NPM version
- Automatic platform detection
- Smaller downloads (only installs one platform)
- Works with Claude Desktop, Cursor, any Python environment

### pyproject.toml Example:
```toml
[project]
name = "gasoline-mcp"
version = "5.1.0"
description = "Browser observability for AI coding agents"
requires-python = ">=3.8"
dependencies = []

[project.optional-dependencies]
darwin-arm64 = ["gasoline-mcp-darwin-arm64==5.1.0"]
darwin-x64 = ["gasoline-mcp-darwin-x64==5.1.0"]
linux-arm64 = ["gasoline-mcp-linux-arm64==5.1.0"]
linux-x64 = ["gasoline-mcp-linux-x64==5.1.0"]
win32-x64 = ["gasoline-mcp-win32-x64==5.1.0"]

[project.scripts]
gasoline-mcp = "gasoline_mcp.__main__:main"

[build-system]
requires = ["setuptools>=61.0", "wheel"]
build-backend = "setuptools.build_meta"
```

### Platform Detection (gasoline_mcp/platform.py):
```python
import sys
import platform
import subprocess

def get_platform():
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
    platform_name = get_platform()
    package_name = f"gasoline_mcp_{platform_name.replace('-', '_')}"
    
    try:
        import importlib.util
        spec = importlib.util.find_spec(package_name)
        if spec and spec.origin:
            import os
            return os.path.join(os.path.dirname(spec.origin), "gasoline")
    except ImportError:
        pass
    
    raise RuntimeError(
        f"Platform-specific package not found. Install with:\n"
        f"  pip install gasoline-mcp[{platform_name}]"
    )

def run():
    binary = get_binary_path()
    subprocess.run([binary] + sys.argv[1:])
```

### Entry Point (__main__.py):
```python
from .platform import run

def main():
    run()

if __name__ == "__main__":
    main()
```

### Publish Process:
```bash
# Build all platform packages
make build-all-platforms

# Build Python wheels
cd pypi/gasoline-mcp && python -m build
cd pypi/gasoline-mcp-darwin-arm64 && python -m build
# ... repeat for each platform

# Publish to PyPI
twine upload dist/*
```

### Configuration in Claude Desktop:
```json
{
  "mcpServers": {
    "gasoline": {
      "type": "stdio",
      "command": "gasoline-mcp",
      "args": ["--port", "7890", "--persist"]
    }
  }
}
```

### Installation Methods:
```bash
# Automatic (recommended)
pip install gasoline-mcp

# Manual platform selection
pip install gasoline-mcp[darwin-arm64]

# From source
git clone https://github.com/brennhill/gasoline-mcp-ai-devtools
cd gasoline && make install-pypi
```

### Comparison:
| Distribution | Install | Size | Users |
|--------------|---------|------|-------|
| NPM | `npx -y gasoline-mcp` | ~5MB | Node.js devs, Claude Code |
| PyPI | `pip install gasoline-mcp` | ~5MB | Python devs, Claude Desktop |
| Homebrew | `brew install gasoline` | ~5MB | macOS power users |
| Binary | Download from releases | ~5MB | Windows users |

### Next Steps to Implement:
1. Create pypi/ directory structure
2. Write pyproject.toml for each package
3. Create platform detection wrapper
4. Update Makefile with pypi targets
5. Test installation
6. Publish to PyPI

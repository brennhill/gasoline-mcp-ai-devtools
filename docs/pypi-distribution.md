# PyPI Distribution

Gasoline is distributed on PyPI using a multi-platform package structure similar to the NPM distribution.

## Architecture

The PyPI distribution consists of 6 packages:

1. **gasoline-mcp** (main package) - Platform detection wrapper
2. **gasoline-mcp-darwin-arm64** - macOS ARM64 binary
3. **gasoline-mcp-darwin-x64** - macOS Intel binary
4. **gasoline-mcp-linux-arm64** - Linux ARM64 binary
5. **gasoline-mcp-linux-x64** - Linux x86_64 binary
6. **gasoline-mcp-win32-x64** - Windows x64 binary

When users run `pip install gasoline-mcp`, the main package automatically installs the appropriate platform-specific binary package based on the detected platform.

## Installation

### End Users

```bash
# Install the main package
pip install gasoline-mcp

# Run the server
gasoline-mcp

# Or with options
gasoline-mcp --port 7890 --persist
```

### MCP Configuration

Add to your `.mcp.json` or MCP settings:

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

## Building from Source

### Prerequisites

```bash
# Install Python build tools
pip install build twine

# Verify Go is installed (1.21+)
go version
```

### Build All Packages

```bash
# Quick build using Makefile
make pypi-build

# Or use the build script
./scripts/build-pypi.sh
```

This will:
1. Build Go binaries for all 5 platforms
2. Copy binaries to platform-specific packages
3. Build Python wheels for all 6 packages

### Manual Build Steps

```bash
# 1. Build Go binaries
make build

# 2. Copy binaries to PyPI packages
make pypi-binaries

# 3. Build Python wheels
cd pypi/gasoline-mcp-darwin-arm64
python3 -m build
cd ../..

# Repeat for each platform package, then build main package
cd pypi/gasoline-mcp
python3 -m build
cd ../..
```

## Publishing

### Test PyPI (Recommended First)

```bash
# Publish to Test PyPI
make pypi-test-publish

# Test installation
pip install --index-url https://test.pypi.org/simple/ gasoline-mcp

# Test execution
gasoline-mcp --version
```

### Production PyPI

```bash
# Ensure version is updated
make sync-version

# Build and publish
make pypi-publish
```

### Manual Publishing

```bash
# Set credentials
export TWINE_USERNAME=__token__
export TWINE_PASSWORD=pypi-xxxxx...

# Publish platform packages first
for pkg in pypi/gasoline-mcp-*/; do
    cd $pkg
    python3 -m twine upload dist/*
    cd ../..
done

# Then publish main package
cd pypi/gasoline-mcp
python3 -m twine upload dist/*
```

## Version Management

All PyPI package versions are synchronized with the main project version defined in the Makefile:

```bash
# Update version in Makefile
VERSION := 5.1.0

# Sync all package versions
make sync-version
```

This updates:
- `pyproject.toml` in all 6 packages
- `__init__.py` version strings
- Optional dependency versions in main package

## Package Structure

### Main Package (`gasoline-mcp`)

```
gasoline_mcp/
├── __init__.py          # Version info
├── __main__.py          # Entry point (main function)
├── platform.py          # Platform detection and binary execution
└── README.md
```

### Platform Packages (`gasoline-mcp-*`)

```
gasoline_mcp_darwin_arm64/
├── __init__.py          # Version info
├── gasoline-darwin-arm64 # Go binary
├── MANIFEST.in          # Include binary in wheel
└── README.md
```

## Platform Detection

The main package detects the platform using `sys.platform` and `platform.machine()`:

| Platform | sys.platform | machine | Package |
|----------|-------------|---------|---------|
| macOS ARM64 | `darwin` | `arm64` | gasoline-mcp-darwin-arm64 |
| macOS Intel | `darwin` | `x86_64` | gasoline-mcp-darwin-x64 |
| Linux ARM64 | `linux` | `aarch64`, `arm64` | gasoline-mcp-linux-arm64 |
| Linux x64 | `linux` | `x86_64`, `amd64` | gasoline-mcp-linux-x64 |
| Windows x64 | `win32` | any | gasoline-mcp-win32-x64 |

## Troubleshooting

### Binary Not Found

If users see "Platform binary not found" errors:

```bash
# Check platform detection
python3 -c "from gasoline_mcp.platform import get_platform; print(get_platform())"

# Manually install platform package
pip install gasoline-mcp-darwin-arm64  # or appropriate platform
```

### Permission Errors on Unix

```bash
# Make binary executable
chmod +x ~/.local/lib/python*/site-packages/gasoline_mcp_darwin_arm64/gasoline-darwin-arm64
```

### Version Mismatches

Ensure all packages have matching versions:

```bash
# Check installed versions
pip list | grep gasoline-mcp

# Reinstall with exact version
pip install --force-reinstall gasoline-mcp==5.1.0
```

## Development

### Local Testing

```bash
# Build packages
make pypi-build

# Create test environment
python3 -m venv test-env
source test-env/bin/activate

# Install from local wheels
pip install pypi/gasoline-mcp/dist/gasoline_mcp-*.whl

# Test execution
gasoline-mcp --version
gasoline-mcp --help
```

### Clean Build Artifacts

```bash
make pypi-clean
```

This removes:
- `build/` directories
- `dist/` directories
- `*.egg-info` directories
- `__pycache__` directories

## Release Checklist

- [ ] Update VERSION in Makefile
- [ ] Run `make sync-version`
- [ ] Build all Go binaries: `make build`
- [ ] Build PyPI packages: `make pypi-build`
- [ ] Test local installation
- [ ] Publish to Test PyPI: `make pypi-test-publish`
- [ ] Test installation from Test PyPI
- [ ] Publish to production: `make pypi-publish`
- [ ] Verify on PyPI.org
- [ ] Test user installation: `pip install gasoline-mcp`
- [ ] Update release notes

## Additional Resources

- [PyPI Package Index](https://pypi.org/project/gasoline-mcp/)
- [Python Packaging Guide](https://packaging.python.org/)
- [Building Platform Wheels](https://packaging.python.org/guides/distributing-packages-using-setuptools/)
- [Twine Documentation](https://twine.readthedocs.io/)

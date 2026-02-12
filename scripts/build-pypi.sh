#!/bin/bash
# Build and prepare PyPI packages for publication
set -euo pipefail

echo "=== Gasoline PyPI Build Script ==="
echo ""

# Check prerequisites
echo "Checking prerequisites..."
if ! command -v python3 &> /dev/null; then
    echo "ERROR: python3 not found. Please install Python 3.8+"
    exit 1
fi

if ! python3 -c "import build" 2>/dev/null; then
    echo "ERROR: Python 'build' module not found."
    echo "Install with: pip install build"
    exit 1
fi

if ! python3 -c "import twine" 2>/dev/null; then
    echo "WARNING: Python 'twine' module not found (needed for publishing)."
    echo "Install with: pip install twine"
fi

echo "✓ Prerequisites OK"
echo ""

# Build Go binaries first
echo "Building Go binaries for all platforms..."
make build
echo "✓ Binaries built"
echo ""

# Copy binaries to PyPI packages
echo "Copying binaries to PyPI packages..."
make pypi-binaries
echo "✓ Binaries copied"
echo ""

# Build Python wheels
echo "Building Python wheels..."
make pypi-build
echo "✓ Wheels built"
echo ""

# List created wheels
echo "=== Build Complete ==="
echo ""
echo "Created wheels:"
find pypi -name "*.whl" -type f | while read -r wheel; do
    echo "  - $wheel"
done
echo ""

# Test local installation instructions
echo "=== Testing ==="
echo ""
echo "To test local installation:"
echo "  1. Create a virtual environment:"
echo "     python3 -m venv test-env"
echo "     source test-env/bin/activate"
echo ""
echo "  2. Install from local wheels:"
echo "     pip install pypi/gasoline-mcp/dist/gasoline_mcp-*.whl"
echo ""
echo "  3. Test the installation:"
echo "     gasoline-mcp --version"
echo ""

# Publishing instructions
echo "=== Publishing ==="
echo ""
echo "To publish to Test PyPI (recommended first):"
echo "  make pypi-test-publish"
echo ""
echo "To publish to production PyPI:"
echo "  make pypi-publish"
echo ""
echo "NOTE: Publishing requires PyPI credentials."
echo "Set environment variables: TWINE_USERNAME and TWINE_PASSWORD"
echo "Or configure ~/.pypirc"

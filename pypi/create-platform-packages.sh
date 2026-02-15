#!/bin/bash
set -euo pipefail

PLATFORMS=("darwin-arm64" "darwin-x64" "linux-arm64" "linux-x64" "win32-x64")
VERSION="0.7.2"

for platform in "${PLATFORMS[@]}"; do
    pkg_name="gasoline-mcp-${platform}"
    pkg_dir="pypi/${pkg_name}"
    pkg_python_name=$(echo "${pkg_name}" | tr '-' '_')
    
    echo "Creating ${pkg_name}..."
    
    # Create package directory
    mkdir -p "${pkg_dir}/${pkg_python_name}"
    
    # Create __init__.py
    cat > "${pkg_dir}/${pkg_python_name}/__init__.py" <<EOF
"""Platform-specific Gasoline binary for ${platform}."""

__version__ = "${VERSION}"
EOF
    
    # Create pyproject.toml
    cat > "${pkg_dir}/pyproject.toml" <<EOF
[project]
name = "${pkg_name}"
version = "${VERSION}"
description = "Gasoline MCP binary for ${platform}"
requires-python = ">=3.8"
license = "AGPL-3.0-only"
authors = [
    {name = "Brennan Hill", email = "noreply@cookwithgasoline.com"}
]

[project.urls]
Homepage = "https://cookwithgasoline.com"
Repository = "https://github.com/brennhill/gasoline-mcp-ai-devtools"

[build-system]
requires = ["setuptools>=61.0", "wheel"]
build-backend = "setuptools.build_meta"

[tool.setuptools]
packages = ["${pkg_python_name}"]
package-data = {"${pkg_python_name}" = ["gasoline*"]}
EOF
    
    # Create README
    cat > "${pkg_dir}/README.md" <<EOF
# ${pkg_name}

Platform-specific binary package for Gasoline MCP (${platform}).

This package is automatically installed as a dependency when you run:

\`\`\`bash
pip install gasoline-mcp
\`\`\`

You do not need to install this package directly.

For more information, see the main [gasoline-mcp](https://pypi.org/project/gasoline-mcp/) package.
EOF
    
    # Create MANIFEST.in to include binary
    cat > "${pkg_dir}/MANIFEST.in" <<EOF
include ${pkg_python_name}/gasoline*
EOF
    
    echo "  ✓ Created ${pkg_name}"
done

echo ""
echo "All platform packages created successfully!"
echo "Next steps:"
echo "  1. Build all platforms: make build-all-platforms"
echo "  2. Copy binaries to pypi/*/gasoline_mcp_*/ directories"
echo "  3. Build Python packages: make build-pypi"
echo "  4. Publish: make publish-pypi"

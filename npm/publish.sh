#!/bin/bash
set -eo pipefail

# Kaboom npm publish script
# Usage: ./publish.sh [--dry-run]

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
DRY_RUN=""

if [ "$1" = "--dry-run" ]; then
  DRY_RUN="--dry-run"
  echo "=== DRY RUN MODE ==="
fi

echo "Building binaries for all platforms..."
cd "$ROOT_DIR"
make build

echo ""
echo "Copying binaries to npm packages..."

# Copy binaries to platform packages
cp dist/gasoline-darwin-arm64    npm/darwin-arm64/bin/gasoline
cp dist/gasoline-darwin-x64      npm/darwin-x64/bin/gasoline
cp dist/gasoline-linux-arm64     npm/linux-arm64/bin/gasoline
cp dist/gasoline-linux-x64       npm/linux-x64/bin/gasoline
cp dist/gasoline-win32-x64.exe   npm/win32-x64/bin/gasoline.exe

# Ensure binaries are executable
chmod +x npm/darwin-arm64/bin/gasoline
chmod +x npm/darwin-x64/bin/gasoline
chmod +x npm/linux-arm64/bin/gasoline
chmod +x npm/linux-x64/bin/gasoline

# Ensure the main bin script is executable
chmod +x npm/kaboom-agentic-browser/bin/kaboom-agentic-browser

echo ""
echo "Copying extension to main npm package..."
mkdir -p npm/kaboom-agentic-browser/extension
cp -r "$ROOT_DIR/extension/"* npm/kaboom-agentic-browser/extension/

echo ""
echo "Publishing platform packages..."

PACKAGES=(
  "darwin-arm64"
  "darwin-x64"
  "linux-arm64"
  "linux-x64"
  "win32-x64"
)

for pkg in "${PACKAGES[@]}"; do
  echo "  Publishing @brennhill/kaboom-agentic-browser-${pkg}..."
  cd "$SCRIPT_DIR/$pkg"
  if [ -n "$DRY_RUN" ]; then
    npm publish --access public "$DRY_RUN"
  else
    npm publish --access public
  fi
done

echo ""
echo "Publishing main package (kaboom-agentic-browser)..."
cd "$SCRIPT_DIR/kaboom-agentic-browser"
if [ -n "$DRY_RUN" ]; then
  npm publish --access public "$DRY_RUN"
else
  npm publish --access public
fi

echo ""
echo "Done! All packages published."
echo ""
echo "Users can now run with:"
echo "  npx kaboom-agentic-browser"

#!/bin/bash
# Fix ES module imports by adding .js extensions
# Required for Chrome MV3 service workers with type="module"
set -euo pipefail

echo "Fixing ESM imports in extension/..."

# Add .js to relative imports in compiled JS files (including root background.js)
find extension -maxdepth 1 -name "*.js" -type f -exec sed -i '' -E \
  "s|from '(\.\.?/[^']+)';|from '\1.js';|g" {} \;

find extension/background extension/inject extension/lib extension/popup extension/content -name "*.js" -type f -exec sed -i '' -E \
  "s|from '(\.\.?/[^']+)';|from '\1.js';|g" {} \;

# Fix double .js.js if any
find extension -name "*.js" -type f -exec sed -i '' \
  "s|\.js\.js'|.js'|g" {} \;

# Fix .ts.js (from type imports that got .js added)
find extension -name "*.js" -type f -exec sed -i '' \
  "s|\.ts\.js'|.js'|g" {} \;

echo "✓ ESM imports fixed"

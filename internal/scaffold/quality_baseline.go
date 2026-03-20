// quality_baseline.go — Writes quality config files (prettier, eslint, vitest) into the scaffolded project.

package scaffold

import (
	"os"
	"path/filepath"
)

// WriteQualityBaseline writes quality configuration files to the project directory.
func WriteQualityBaseline(projectDir string) error {
	files := []struct {
		path    string
		content string
		mode    os.FileMode
	}{
		{".prettierrc", prettierConfig, 0644},
		{"eslint.config.js", eslintConfig, 0644},
		{"tsconfig.json", tsconfigOverrides, 0644},
		{"vite.config.ts", viteConfig, 0644},
		{"vitest.config.ts", vitestConfig, 0644},
		{"scripts/check-components.sh", componentInvariantScript, 0755},
		{"src/App.tsx", appShell, 0644},
		{"src/App.test.tsx", appTestFile, 0644},
		{"src/index.css", indexCSS, 0644},
	}

	for _, f := range files {
		path := filepath.Join(projectDir, f.path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(f.content), f.mode); err != nil {
			return err
		}
	}

	return nil
}

const prettierConfig = `{
  "singleQuote": true,
  "semi": false,
  "tabWidth": 2,
  "trailingComma": "all",
  "printWidth": 100
}
`

const vitestConfig = `import { defineConfig } from 'vitest/config'
import path from 'path'

export default defineConfig({
  test: {
    environment: 'happy-dom',
    globals: true,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      thresholds: {
        lines: 60,
      },
    },
  },
  resolve: {
    alias: {
      '@/': path.resolve(__dirname, './src/'),
    },
  },
})
`

const componentInvariantScript = `#!/usr/bin/env bash
# check-components.sh — Component invariant checker.
# Enforces: no inline style=, no any types, no ../ imports,
# no hardcoded hex colors, no files over 200 LOC.

set -euo pipefail

ERRORS=0
SRC="src/components"

if [ ! -d "$SRC" ]; then
  echo "No components directory found, skipping."
  exit 0
fi

# Check for inline style= attributes
if grep -rn 'style=' "$SRC" --include="*.tsx" --include="*.ts" 2>/dev/null; then
  echo "ERROR: Inline style= found. Use Tailwind classes instead."
  ERRORS=$((ERRORS + 1))
fi

# Check for 'any' type annotations
if grep -rn ': any' "$SRC" --include="*.tsx" --include="*.ts" 2>/dev/null; then
  echo "ERROR: 'any' type found. Use specific types."
  ERRORS=$((ERRORS + 1))
fi

# Check for ../ relative imports
if grep -rn "from '\.\.\/" "$SRC" --include="*.tsx" --include="*.ts" 2>/dev/null; then
  echo "ERROR: Relative ../ imports found. Use @/ path aliases."
  ERRORS=$((ERRORS + 1))
fi

# Check for hardcoded hex colors in className
if grep -rn '#[0-9a-fA-F]\{3,8\}' "$SRC" --include="*.tsx" --include="*.ts" 2>/dev/null | grep -v '\.css' | grep -v '//' ; then
  echo "ERROR: Hardcoded hex colors found. Use theme tokens."
  ERRORS=$((ERRORS + 1))
fi

# Check for files over 200 LOC
for f in $(find "$SRC" -name "*.tsx" -o -name "*.ts"); do
  lines=$(wc -l < "$f")
  if [ "$lines" -gt 200 ]; then
    echo "ERROR: $f has $lines lines (max 200)."
    ERRORS=$((ERRORS + 1))
  fi
done

if [ "$ERRORS" -gt 0 ]; then
  echo "Component invariant check failed with $ERRORS error(s)."
  exit 1
fi

echo "Component invariant check passed."
`

const appShell = `function App() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <main className="container mx-auto px-4 py-8">
        <h1 className="text-3xl font-bold">Welcome</h1>
        <p className="mt-2 text-muted-foreground">Your app is ready. Start building.</p>
      </main>
    </div>
  )
}

export default App
`

const appTestFile = `import { describe, it, expect } from 'vitest'

describe('App', () => {
  it('should be importable', async () => {
    const mod = await import('./App')
    expect(mod.default).toBeDefined()
  })
})
`

const eslintConfig = `import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import reactHooks from 'eslint-plugin-react-hooks'

export default tseslint.config(
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    plugins: {
      'react-hooks': reactHooks,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
    },
  },
  {
    ignores: ['dist/', 'node_modules/'],
  },
)
`

const tsconfigOverrides = `{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["src"]
}
`

const viteConfig = `import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
})
`

const indexCSS = `@import 'tailwindcss';

@theme {
  --color-primary: #2563eb;
  --color-secondary: #64748b;
  --color-accent: #f59e0b;
  --color-background: #ffffff;
  --color-foreground: #0f172a;
  --color-muted: #f1f5f9;
  --color-muted-foreground: #64748b;
}
`

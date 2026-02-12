#!/usr/bin/env node

/**
 * Post-process compiled JavaScript files to add .js extensions to relative imports.
 * Required for Node.js ES module compatibility.
 */

import fs from 'fs'
import path from 'path'
// eslint-disable-next-line n/no-hide-core-modules -- Node built-in import, not hiding a core module
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const EXTENSION_DIR = path.join(__dirname, '../extension')

/**
 * Fix imports in a JavaScript file by adding .js extensions to relative imports.
 */
function fixImportsInFile(filePath) {
  // eslint-disable-next-line security/detect-non-literal-fs-filename -- build script paths derived from local directory traversal
  let content = fs.readFileSync(filePath, 'utf8')
  const original = content

  // Match: from './path/to/module' or from "./path/to/module"
  // But NOT: from './path/to/module.js' or from './path/to/module.json'
  content = content.replace(/from\s+['"`](\.[^'"`]+?)['"`]/g, (match, importPath) => {
    // If it already has an extension, don't modify it
    if (path.extname(importPath)) {
      return match
    }
    // Add .js extension
    return `from '${importPath}.js'`
  })

  // Also fix import statements: import ... from './path/to/module'
  content = content.replace(/import\s[^f]*from\s+['"`](\.[^'"`]+?)['"`]/g, (match, importPath) => {
    if (path.extname(importPath)) {
      return match
    }
    return match.replace(importPath, `${importPath}.js`)
  })

  // Also fix dynamic imports: import('./path/to/module')
  content = content.replace(/import\s*\(\s*['"`](\.[^'"`]+?)['"`]\s*\)/g, (match, importPath) => {
    if (path.extname(importPath)) {
      return match
    }
    return `import('${importPath}.js')`
  })

  if (content !== original) {
    // eslint-disable-next-line security/detect-non-literal-fs-filename -- build script paths derived from local directory traversal
    fs.writeFileSync(filePath, content, 'utf8')
    console.log(`Fixed imports in: ${path.relative(process.cwd(), filePath)}`)
  }
  return
}

/**
 * Recursively process all .js files in a directory.
 */
function processDirectory(dir) {
  // eslint-disable-next-line security/detect-non-literal-fs-filename -- build script paths derived from local directory traversal
  const files = fs.readdirSync(dir)

  for (const file of files) {
    const filePath = path.join(dir, file)
    // eslint-disable-next-line security/detect-non-literal-fs-filename -- build script paths derived from local directory traversal
    const stat = fs.statSync(filePath)

    if (stat.isDirectory()) {
      processDirectory(filePath)
    } else if (file.endsWith('.js') && !file.endsWith('.d.ts')) {
      fixImportsInFile(filePath)
    }
  }
  return
}

// Process the extension directory
console.log('Fixing imports in extension directory...')
processDirectory(EXTENSION_DIR)
console.log('Done!')

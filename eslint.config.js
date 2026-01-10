import js from '@eslint/js'
import security from 'eslint-plugin-security'
import globals from 'globals'
import prettier from 'eslint-config-prettier'

export default [
  // Ignore patterns
  {
    ignores: ['node_modules/', 'dist/', 'tests/e2e/', 'npm/', 'server/', 'docs/', 'demo/', 'extension/lib/*.min.js'],
  },

  // Base recommended rules
  js.configs.recommended,

  // Extension source files (run in browser / service worker)
  {
    files: ['extension/**/*.js'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      globals: {
        ...globals.browser,
        chrome: 'readonly',
        clients: 'readonly',
        registration: 'readonly',
        self: 'readonly',
      },
    },
    plugins: {
      security,
    },
    rules: {
      'no-var': 'error',
      'prefer-const': ['error', { destructuring: 'all' }],
      eqeqeq: ['error', 'always'],
      'prefer-arrow-callback': 'error',
      'no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],

      // Security rules
      'security/detect-object-injection': 'warn',
      'security/detect-non-literal-regexp': 'warn',
      'security/detect-eval-with-expression': 'error',
      'security/detect-no-csrf-before-method-override': 'error',
      'security/detect-possible-timing-attacks': 'warn',

      // Best practices
      'no-eval': 'error',
      'no-implied-eval': 'error',
      'no-new-func': 'error',
      'no-script-url': 'error',
      'no-proto': 'error',
      'no-extend-native': 'error',
      'no-throw-literal': 'error',
      'no-promise-executor-return': 'error',
      'no-constructor-return': 'error',
      'no-template-curly-in-string': 'warn',
      'no-loss-of-precision': 'error',
      'require-atomic-updates': 'error',
    },
  },

  // Extension test files (run in Node.js)
  {
    files: ['tests/extension/**/*.js', 'tests/extension/**/*.mjs'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      globals: {
        ...globals.node,
        globalThis: 'readonly',
        document: 'readonly',
      },
    },
    plugins: {
      security,
    },
    rules: {
      'no-var': 'error',
      'prefer-const': ['error', { destructuring: 'all' }],
      eqeqeq: ['error', 'always'],
      'no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],

      // Relaxed security for tests
      'security/detect-object-injection': 'off',
      'security/detect-non-literal-regexp': 'off',
      'security/detect-possible-timing-attacks': 'off',

      // Best practices
      'no-eval': 'error',
      'no-implied-eval': 'error',
      'no-new-func': 'error',
    },
  },

  // Prettier must be last to disable conflicting format rules
  prettier,
]

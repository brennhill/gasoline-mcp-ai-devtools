import js from '@eslint/js'
import security from 'eslint-plugin-security'
import globals from 'globals'
import prettier from 'eslint-config-prettier'

export default [
  // Ignore patterns
  {
    ignores: [
      'node_modules/',
      'dist/',
      'tests/e2e/',
      'npm/',
      'server/',
      'docs/',
      'demo/',
      // Compiled TypeScript output (linted at the .ts source level)
      'extension/background/',
      'extension/content/',
      'extension/inject/',
      'extension/lib/',
      'extension/popup/',
      'extension/types/',
      'extension/offscreen/',
      // Bundled files
      'extension/content.bundled.js',
      'extension/inject.bundled.js',
      'extension/early-patch.bundled.js',
      'extension/offscreen.bundled.js',
    ],
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
        __GASOLINE_VERSION__: 'readonly',
      },
    },
    plugins: {
      security,
    },
    rules: {
      'no-var': 'error',
      'prefer-const': ['error', { destructuring: 'all' }],
      eqeqeq: ['error', 'always', { null: 'ignore' }],
      'prefer-arrow-callback': 'error',
      'no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' },
      ],

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
    files: ['tests/extension/**/*.js', 'tests/extension/**/*.mjs', 'tests/extension/**/*.cjs'],
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
      eqeqeq: ['error', 'always', { null: 'ignore' }],
      'no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' },
      ],

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

  // CLI test files (CommonJS, run in Node.js)
  {
    files: ['tests/cli/**/*.cjs'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'commonjs',
      globals: {
        ...globals.node,
      },
    },
    rules: {
      'no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' },
      ],
    },
  },

  // Scripts (ESM, run in Node.js)
  {
    files: ['scripts/**/*.js'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      globals: {
        ...globals.node,
      },
    },
    rules: {
      'no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' },
      ],
    },
  },

  // Gasoline CI package (runs in browser)
  {
    files: ['packages/gasoline-ci/**/*.js'],
    languageOptions: {
      ecmaVersion: 2020,
      sourceType: 'script',
      globals: {
        ...globals.browser,
      },
    },
    rules: {
      'no-var': 'off', // Legacy code uses var
      'prefer-const': 'off',
      'prefer-arrow-callback': 'off',
      'no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' },
      ],
    },
  },

  // Gasoline Playwright package (CommonJS, run in Node.js)
  {
    files: ['packages/gasoline-playwright/**/*.js'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'commonjs',
      globals: {
        ...globals.node,
      },
    },
    rules: {
      'no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' },
      ],
    },
  },

  // Prettier must be last to disable conflicting format rules
  prettier,
]

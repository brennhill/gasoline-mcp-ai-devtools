# Framework Smoke Fixtures

This directory contains source fixtures used by smoke module `29-framework-selector-resilience.sh`.

Goals:
- Use real framework runtimes (React, Vue, Svelte, Next.js), not hand-faked DOM pages.
- Regenerate selector ids/classes on each page load to stress selector resilience.
- Keep fixture visual styling on-brand with Gasoline (including the flame mark) while preserving stable test semantics.
- Model hard automation failure modes:
  - delayed hydration and event binding
  - overlay interception (`Accept Cookies`)
  - SPA route remount churn (stale handles)
  - async/delayed content (`Async Save`)
  - virtualized/lazy deep targets (`Framework Target 80`)
- Keep stable user-facing semantics (`placeholder=Enter name`, `text=Submit Profile`) for agent workflows.

Build command:

```bash
npm run smoke:build-framework-fixtures
```

Generated output is written to:

- `cmd/dev-console/testpages/frameworks/react.html`
- `cmd/dev-console/testpages/frameworks/vue.html`
- `cmd/dev-console/testpages/frameworks/svelte.html`
- `cmd/dev-console/testpages/frameworks/next/`
- corresponding `*.bundle.js` assets

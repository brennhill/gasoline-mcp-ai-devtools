# Strum Code Quality Standards

This file ships with every Strum-scaffolded project at `.strum/code-quality-standards.md`. It is appended to every decorated prompt sent through the terminal. The AI must follow these rules — they are not suggestions.

## File Rules

- **Max 300 lines per file.** If a file exceeds 300 lines, split it before adding more code. Extract components, hooks, or utilities into separate files.
- **One component per file.** Never put two React components in the same file. Helper components get their own file.
- **One hook per file.** Custom hooks go in `src/hooks/` with one hook per file. Name the file after the hook: `useContacts.ts`.
- **Collocate tests.** Every `Foo.tsx` has a `Foo.test.tsx` in the same directory. No separate `__tests__/` folders.

## Naming

- **Components:** PascalCase. `ContactForm.tsx`, not `contact-form.tsx` or `contactForm.tsx`.
- **Hooks:** camelCase prefixed with `use`. `useContacts.ts`, not `contacts-hook.ts`.
- **Utilities:** camelCase. `src/lib/formatDate.ts`.
- **Types:** PascalCase, no `I` prefix. `Contact`, not `IContact`.
- **Files match exports.** If the file exports `ContactForm`, the file is `ContactForm.tsx`.

## Imports

- **Always use `@/` path aliases.** `import { Button } from '@/components/ui/button'`, never `../../../components/ui/button`.
- **No barrel exports.** No `index.ts` files that re-export. Import directly from the source file.
- **Group imports:** React first, external packages second, `@/` internal imports third, relative imports last. Blank line between groups.

## Styling

- **Tailwind only.** No inline `style={}`. No CSS modules. No styled-components. No emotion.
- **Theme tokens only.** Use `bg-primary`, `text-foreground`, `border-border`. Never hardcode colors: no `bg-[#2563eb]`, no `text-red-500` (use `text-destructive`).
- **shadcn first.** If shadcn has a component for it, use it. Don't build a custom button, dialog, select, or form input.
- **Responsive from birth.** Every component must work at 375px. Use Tailwind responsive prefixes: `sm:`, `md:`, `lg:`.
- **No magic numbers.** Use Tailwind spacing scale (`p-4`, `gap-6`, `mt-8`), not arbitrary values (`p-[13px]`).

## Components

- **Props interface above component.** Define `interface FooProps {}` directly above the component function, not in a separate types file.
- **Destructure props.** `function ContactForm({ name, onSubmit }: ContactFormProps)`, not `function ContactForm(props: ContactFormProps)`.
- **Default to function declarations.** `function ContactForm()`, not `const ContactForm = () =>`. Arrow functions for inline callbacks only.
- **No `any`.** Ever. Use `unknown` if the type is truly unknown, then narrow it.
- **No `as` type assertions.** If you need a cast, the types are wrong. Fix the types.
- **No `// @ts-ignore` or `// @ts-expect-error`.** Fix the type error.

## State & Data

- **Supabase for persistent data.** Use `src/lib/supabase.ts` for all database operations. Don't build a custom API layer.
- **Row-level security.** Every Supabase table must have RLS policies. Never disable RLS.
- **React Query / SWR pattern.** Use the generated hooks (`useContacts`, `useTodos`) for data fetching. Don't call Supabase directly in components.
- **Optimistic updates.** For user-facing mutations, update the UI immediately and reconcile on server response.
- **No global state unless shared.** Use component state by default. Lift state up only when two components need the same data. Use context only for truly global state (auth, theme).

## Error Handling

- **Error boundaries around routes.** Each page should catch rendering errors without crashing the app.
- **Try/catch on async operations.** Every `await` in an event handler needs error handling.
- **User-visible errors use toast.** `import { toast } from '@/components/ui/toast'`. Never `alert()` or `console.error()` for user-facing errors.
- **Log unexpected errors.** `console.error()` for debugging, toast for user feedback. Both, not one or the other.

## Testing

- **One test file per component.** `ContactForm.test.tsx` next to `ContactForm.tsx`.
- **Test behavior, not implementation.** Test what the user sees and does, not internal state or method calls.
- **Use testing-library patterns.** `getByRole`, `getByText`, `getByLabelText`. Never `getByTestId` unless no semantic alternative exists.
- **Arrange-Act-Assert.** Every test has three clear sections. No test longer than 20 lines.
- **No snapshots.** They break on every change and test nothing useful.

## Accessibility

- **Form inputs need labels.** Every `<input>` has a `<label>` with `htmlFor`. Every shadcn `Input` is wrapped with `Label`.
- **Images need alt text.** Decorative images: `alt=""`. Meaningful images: descriptive alt text.
- **Interactive elements are focusable.** If it's clickable, it must be a `<button>` or `<a>`, not a `<div onClick>`.
- **Color is not the only indicator.** Don't rely on color alone for status (add icons or text).
- **Heading hierarchy.** One `h1` per page, headings don't skip levels.

## Git

- **Small, focused commits.** One logical change per commit. "Add contact form" not "Add contact form, fix header, update styles".
- **Descriptive messages.** `feat: add contact form with validation` not `update` or `wip`.
- **No commented-out code.** Delete it. Git has history.
- **No TODO comments without an issue.** If it's worth a TODO, it's worth a ticket.

## Performance

- **No unnecessary re-renders.** Use `React.memo` only when profiling shows a problem, not preemptively.
- **Lazy load pages.** Use `React.lazy()` for route-level code splitting.
- **Images are optimized.** Use modern formats (WebP/AVIF), appropriate sizes, and loading="lazy".
- **No bundle bloat.** Don't import entire libraries for one function. `import { format } from 'date-fns'`, not `import * as dateFns`.

## What NOT to Do

- Don't create abstractions for one use case. Three similar lines of code is better than a premature abstraction.
- Don't add features the user didn't ask for. Build exactly what was requested.
- Don't refactor code you weren't asked to touch. Stay focused.
- Don't add comments that restate the code. Comments explain WHY, not WHAT.
- Don't create a `utils/` dumping ground. If a utility is used by one component, keep it in that component's file.
- Don't install new dependencies without justification. The scaffolded stack covers 95% of needs.

---
name: review-typescript
description: TypeScript/JavaScript review checklist. Types, null-safety, async, memory leaks, React hooks, strict mode, performance. Use when reviewing or analyzing TS/JS code, ".ts/.tsx/.js" files, React components, Node/Express services.
---

# TypeScript/JavaScript Review

Review and analyze TS/JS code. Combine with deep-analysis and the reviewer agent.

## Types & null safety

- **any**: no `any` leaking across boundaries; `unknown` + narrowing preferred; `@ts-ignore`/`@ts-expect-error` justified.
- **Null**: optional chaining used consistently; properties that can be undefined handled before use; discriminated unions for states.
- **strict mode**: `tsconfig` has `strict: true`; no `noUncheckedIndexedAccess` violations silently ignored.
- **Type narrowing**: `as` casts justified and verified; no casting to lie to the compiler.

## Async & errors

- **Async**: no fire-and-forget promises without `.catch`; `Promise.all` (not sequential `await` in loop) when independent; unhandled rejections surface.
- **Errors**: error paths handled — try/catch or `.catch` present on all async calls; no swallowed errors (`catch {}`).
- **Timeout/abort**: `AbortController` used for fetch and timeouts; no dangling requests after unmount.

## Memory & leaks

- **Event listeners**: added with matching cleanup (`addEventListener` ↔ `removeEventListener`, `window`/`document` listeners in useEffect cleanup).
- **React**: `useEffect` deps correct — no missing deps, no unnecessary re-runs, no stale closures (use refs/callbacks properly); intervals/timeouts cleared in cleanup.
- **Caches**: unbounded in-memory caches without eviction or TTL.
- **DOM refs**: references released; no growing arrays/subscriptions.

## Performance & scaling

- **Rendering**: expensive work inside render vs `useMemo`/`useCallback` justified; list keys stable; O(n²) reconciliation; virtualization for huge lists.
- **Bundle**: large libs imported whole (`import _ from 'lodash'`) instead of tree-shaken paths; heavy `import` in hot module.
- **N+1**: API fetch per list item instead of batch; missing debounce on high-frequency input.
- **CPU**: string concat in loops, regex in hot paths, heavy computation per frame.

## Node/Express specifics

- Middleware order; async handlers wrapped so rejected promises don't crash; input validated at the boundary; no unbounded body size; `helmet`, CORS scoped.
- File uploads/streams closed; no `process.env` secrets in client bundles.

## Rule

Every finding cites `file:line`. Flag with 🔴 bug / 🟡 risk / 🔵 nit / ❓ question.

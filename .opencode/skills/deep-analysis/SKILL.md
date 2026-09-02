---
name: deep-analysis
description: Total code analysis of files, directories and flows. Maps structure, traces execution, finds bugs, security holes, performance and scaling problems. Use when user asks to "analyze", "analiza", "audit total", "understand this codebase", "find problems in", "trace the flow of", or analyzes performance/escalabilidad.
---

# Deep Analysis

Total, rigorous analysis of code, files and flows. Combine reading the code, tracing flows and reasoning about failure before reporting.

## Steps

1. **Scope.** List targets. Note language, size, entrypoints, build/test commands (read AGENTS.md, go.mod, package.json).
2. **Map.** Build a module/dependency graph. Identify the public API and the trust boundaries (where external input enters).
3. **Read fully.** Read whole files, not just names. Take notes per file: responsibility, key funcs with `file:line`.
4. **Trace flows.** Follow each main path end to end: entry → happy path → error path → exit. Record side effects and failure points.
5. **Apply language skill.** Deep-dive per language: review-go, review-typescript, review-shell. For unknown: use the generic checklist below.
6. **Scale test.** Ask "what breaks at 10×": files, concurrency, memory, network, disk.
7. **Report.** Structured markdown (see agent analyst). Every finding cites `file:line`.

## Generic checklist (any language)

- **Bugs**: off-by-one, nil/null deref, wrong condition, error path that isn't handled, state not cleaned up.
- **Errors**: swallowed errors, errors with no context, panic on expected conditions.
- **Resources**: leaks (files, sockets, goroutines, listeners, event handlers), missing close/defer, unbounded growth.
- **Concurrency**: shared mutable state, races, deadlock ordering, blocking calls in loops.
- **Perf**: O(n²) loops, N+1 queries, I/O per item instead of batch, no caching, work in hot path.
- **Scalability**: hardcoded limits, global locks, state that doesn't survive restart, assumptions about input size.
- **Security**: injection, secrets in code/logs, unvalidated input crossing trust boundary, missing limits (timeouts, size caps).
- **Quality**: dead code, duplicated logic, missing tests for critical paths, magic numbers.

## Rules

- Every finding cites `file:line`. Every claim is verified by reading the code.
- Separate: 🔴 breaks behavior · 🟡 fragile · 🔵 improvement · ❓ open question.
- For flows: show the shape of data at each step (what enters, what exits).
- Never claim something is "clean" without checking the risky areas first.
- End with the top 5 quick wins sorted by impact/effort.

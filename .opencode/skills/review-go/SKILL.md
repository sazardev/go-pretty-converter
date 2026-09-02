---
name: review-go
description: Go-specific code review and analysis checklist. Errors, concurrency, memory, idioms, performance. Use when reviewing, analyzing or auditing Go code, ".go" files, or asks about goroutines, channels, mutexes, error handling.
---

# Go Review

Review, analyze and audit Go code. Combine with deep-analysis and the reviewer agent.

## Bugs & correctness

- **Errors**: is the error handled? wrapped with context (`fmt.Errorf("op x: %w", err)`)? never ignored with `_` on critical paths?
- **Nil/zero**: pointer/interface nil checks before use; `map` read without ok; empty slice vs nil semantics.
- **Defer/close**: resources always closed via `defer` in ALL paths — files, `http.Response.Body`, `sql.Rows`, `chromedp` contexts.
- **Context**: `context.Context` is first param, respected by long ops (timeouts, cancellation), not stored in structs; `context.Background()` not used where a real ctx exists.
- **Panics**: no `panic` for expected conditions; `recover` only at a real boundary.

## Concurrency

- **Races**: shared mutable state guarded by mutex/atomic; run `go test -race` and flag anything it would catch.
- **Channels**: buffered/unbuffered choice justified; no send on closed channel; proper close ownership (sender closes, never receiver).
- **Goroutines**: every `go func()` has a lifecycle — WaitGroup/errgroup, no fire-and-forget that outlives the caller; no goroutine leaks on error paths.
- **Mutexes**: no lock held across blocking I/O; consistent lock ordering; RWMutex only where reads dominate.
- **ErrGroup/semaphore**: bounded concurrency for fan-out; no unbounded goroutine spawn per input item.

## Performance & memory

- **Allocations**: escapes to heap in hot loops; `sync.Pool` misuse; strings built with `+` in loops instead of `strings.Builder`.
- **I/O**: batch reads/writes; `bufio`; no read-all on unbounded input (size cap needed).
- **Loops**: O(n²) patterns, re-computing invariants, map lookup inside inner loop when hoistable.
- **Reflection**: `reflect` in hot paths; prefer type switch or generics.
- **Benchmarks**: hot paths should have `Benchmark` tests; flag functions that are hot and untested.

## Idioms & quality

- Interface small, defined at consumer side; avoid `interface{}` (use `any` sparingly, prefer generics).
- Value vs pointer receivers consistent; no copy of mutex; no pointer to loop variable capture.
- `go vet` and `staticcheck` (golangci-lint) pass; exported symbols documented (godoc).
- Tests: table-driven, use `t.Parallel()` only on independent cases, `require` vs `assert` chosen deliberately.

## Rule

Every finding cites `file:line`. Flag with 🔴 bug / 🟡 risk / 🔵 nit / ❓ question. For races or leaks, show the fix pattern, not just the description.

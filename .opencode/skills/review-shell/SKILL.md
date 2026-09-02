---
name: review-shell
description: Shell/bash review checklist. Quoting, word splitting, injection, portability, error handling, temp files, traps. Use when reviewing or analyzing shell scripts, ".sh" files, Makefiles or bash/zsh one-liners.
---

# Shell Review

Review and analyze shell scripts. Combine with deep-analysis and the reviewer agent.

## Safety (highest priority)

- **Quoting**: every variable expansion quoted (`"$var"`); no unquoted expansions in commands, paths or test expressions.
- **Word splitting**: unquoted `$var` that may contain spaces/globs — split, expanded, subject to globbing.
- **Injection**: variables interpolated into `eval`, `sh -c`, `ssh`, `curl -d`, `awk`/`sed` scripts, or `find -exec` — can execute attacker input. Escape or use argv-style (arrays, `"$@"`).
- **`set -euo pipefail`**: missing `set -u` (unbound vars), missing `set -o pipefail` (silent mid-pipe failures), `set -e` disabled for commands whose failure matters.
- **Globs**: `rm *` style with no guard; glob that fails on no-match (nullglob); dangerous wildcards on user input.

## Correctness

- **Exit codes**: last command's status checked where it matters; explicit `exit 0`/`exit 1` at script end; `set -e` + conditionals interplay understood (no failing `&&`/`||` misuse).
- **Word splitting in `for i in $(...)`**: paths with spaces break — use `while read -r` or null-delimited (`.find -print0`).
- **File paths**: spaces handled (`"$@"` style); `IFS` modifications restored; `cd` failures caught.
- **Traps**: `trap cleanup EXIT` for temp files and lock removal; trap with `INT TERM` too.

## Temp files & resources

- `mktemp` used (not predictable `/tmp/foo`); cleaned up on exit via trap; no race in creating lock files (`mkdir` atomic lock or `flock`).
- Pipes closed on error; no unbounded log growth (`>>` forever, missing `tail -n`/rotation).

## Portability & style

- Shebang correct (`#!/usr/bin/env bash` vs `sh`); bash-isms only in bash (`[[ ]]`, arrays) — never in `/bin/sh` scripts.
- Line continuation clean; no `cat file | cmd` (UUOC) where `< file` works; `$(...)` over backticks.
- Runs under `shellcheck` without errors — run it, flag anything it flags.

## Performance

- Loop over lines calling a tool per line (`while read; do grep` → use single `grep`/`awk` pass).
- Avoid launching processes per item in hot loops.

## Rule

Every finding cites `file:line`. Flag with 🔴 bug / 🟡 risk / 🔵 nit / ❓ question. Injection and unquoted-variable issues are always 🔴 or 🟡, never nits.

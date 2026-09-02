---
name: security-review
description: Security audit of code, files or flows. Injection, secrets, auth, trust boundaries, resource limits. Use when user asks for "security review", "seguridad", "audit de seguridad", "is this safe", "vulnerabilidades", or flags untrusted input handling.
---

# Security Review

Audit code for security issues. Combine with deep-analysis and the reviewer agent.

## Checklist

- **Injection**
  - Shell: variables into `exec`/`sh -c`/`system`/subprocess args — must use argv arrays, never string interpolation (Go: `exec.Command` with args; TS: `spawn` with array, no `sh -c`).
  - SQL: parameterized queries only; no string-concatenated SQL.
  - HTML/JS: user content escaped or rendered safely; no raw `innerHTML` with unsanitized input; in MDX/HTML pipelines raw HTML is pass-through by design — treat content as trusted or escape it.
  - Path: user-controlled paths with `..` traversal; `filepath.Clean`/join and confinement to a root.
- **Secrets**: hardcoded keys/passwords/tokens in code, configs committed, or logs. Check git history (`git log -p`) for leaked secrets. Environment variables never logged.
- **Auth/authorization**: access checks on every operation, not just entry; default-deny; missing auth on sensitive endpoints.
- **Trust boundaries**: unvalidated external input entering internals (network, user files, CLI args, env). Validate type, length, format at the boundary. Set explicit limits.
- **Resource exhaustion**: unbounded input size, unlimited retries, no rate limiting, no timeouts on external calls, unbounded concurrency (each input spawns a goroutine/process), zip bombs (read-all on compressed data).
- **Dependencies**: known-vulnerable versions (`go vulncheck`, `npm audit`); lockfiles present.
- **TLS/data at rest**: creds transmitted in clear; sensitive data stored unencrypted without justification.

## Output

Findings as `<file>:L<line> — <problem> — <impact> — <fix>`, grouped:

- 🔴 Critical (exploitable, must fix before ship)
- 🟡 High (conditionally exploitable)
- 🔵 Low/hardening
- ❓ Questions

End with a severity summary and the top fixes in priority order. Any secret found must be reported as a finding but NEVER echoed in full.

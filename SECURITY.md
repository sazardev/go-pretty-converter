# Security Policy

## Reporting a vulnerability

Please **do not** open a public issue for security vulnerabilities.

Instead, report them privately to the maintainers:

- Email: [security contact TBD]
- GitHub: Use the "Report a vulnerability" feature in the Security tab

We aim to respond within 5 business days and publish fixes as patch releases.

## Supported versions

| Version | Supported |
|---------|-----------|
| 0.1.x   | Yes       |

## Scope

`go-pretty-pdf` uses headless Chrome for PDF rendering. While we pass the
`--no-sandbox` flag for compatibility in CI environments, we recommend
running in sandboxed mode for production deployments that process
untrusted HTML content.

Note that custom CSS and HTML templates provided via configuration files
are injected directly into the rendered page. Only use trusted
CSS/template files.

### Trust model: MD/MDX content is not sandboxed

The parser enables raw HTML passthrough (goldmark's unsafe-HTML mode)
so that authors can embed arbitrary HTML in their documents, and the
built-in component transpiler (`<DeepDive>`, `<Warning>`, `<Axiom>`, and
any component registered via `WithComponent`) does not escape inner
content. Any `<script>`, event handler, or `<img>`/`<link>` tag in a
`.md` or `.mdx` file will execute or fetch as part of rendering.

By default, `RenderToPDF` blocks all outbound network requests during
rendering (see `WithNetworkAccess`), which closes the most severe
exfiltration/SSRF vector. Script execution itself is not sandboxed,
however — **only render `.md`/`.mdx` files from authors you trust.** Do
not point this tool at user-submitted or otherwise untrusted content
without additional isolation (e.g. a container with no network egress).

`.txt` files are the exception: they're always HTML-escaped rather than
passed through raw, so a `.txt` file cannot execute a `<script>` tag or
inject markup regardless of who authored it.

# Deployment, caching & performance

How the `go-pretty-converter` site is served, how its caching/compression/security
config is prepared, and how to verify it. This is operational documentation
for anyone self-hosting the generated `_site/` — the same static site the
`docs.yml` workflow deploys to GitHub Pages.

## What gets generated

`go run ./scripts/docsgen` writes `_site/`:

- `index.html`, `docs.html` (all CSS/JS **inline** — no render-blocking
  extra round-trips beyond the Google Fonts stylesheet)
- the theme PDFs/EPUBs, demo PDFs, icons, `og-image.png`, `favicon.svg`
- `sitemap.xml`, `robots.txt`, `llms.txt`, `llms-full.txt`, `humans.txt`,
  `version.json`, `docs-search.json`
- **`_headers`** — cache-control + security headers (see below)

Nothing in `_site/` should be hand-edited; every file is regenerated.

## Where to host it

### GitHub Pages (current)

The `docs.yml` workflow deploys `_site/` automatically. Constraints:

- **No custom headers**: `_headers` is ignored (served as a plain file).
- **No control over gzip/brotli** or HTTP/3.
- Default caching is `max-age=600` with ETag revalidation — fine for a docs
  site, not tunable.

### Cloudflare Pages or Netlify (recommended for speed)

Both hosts read `_site/_headers` and automatically apply:

- **Brotli + gzip compression** (no config needed)
- **HTTP/3** (Cloudflare)
- **CDN edge caching** driven by the `Cache-Control` values in `_headers`
- The security headers below

To switch: point the platform at the repo, build command
`go run ./scripts/docsgen` (Chrome required — install it in the build step),
publish directory `_site`, and keep `docs.yml` for GitHub Pages or drop it.

## The `_headers` config

Committed in `scripts/docsgen/assets/_headers`, copied verbatim into `_site/`
by docsgen. Rules (Cloudflare Pages / Netlify `_headers` syntax):

| Path | Cache-Control |
|---|---|
| `/*` (base) | `public, max-age=0, must-revalidate` |
| `/*.html` | `public, max-age=0, must-revalidate` |
| `/go-pretty-converter-docs-*.pdf` / `.epub`, `/library-demo.pdf`, `/full-demo.pdf` | `public, max-age=86400, stale-while-revalidate=604800` |
| `/og-image.png`, `site.webmanifest` | `public, max-age=86400, stale-while-revalidate=604800` |
| icons (`apple-touch-icon.png`, `favicon-32.png`, `icon-192/512.png`, `favicon.svg`) | `public, max-age=86400, immutable` |
| `llms.txt`, `llms-full.txt`, `humans.txt`, `version.json` | `public, max-age=3600` |

Security headers applied on every response:

```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=(), usb=()
Content-Security-Policy: default-src 'self'; img-src 'self' data:;
  style-src 'self' 'unsafe-inline' https://fonts.googleapis.com;
  font-src 'self' https://fonts.gstatic.com;
  script-src 'self' 'unsafe-inline';
  connect-src 'self' https://fonts.googleapis.com;
  frame-ancestors 'none'; base-uri 'self'; form-action 'self'
```

The CSP allows the site's inline `<style>`/`<script>` and the Google Fonts
pair; `frame-ancestors 'none'` blocks clickjacking.

## What the site already does for speed

- Google Fonts load **async** (`media="print" onload`, `<noscript>` fallback)
  so the LCP text paints without waiting for the font stylesheet.
- `preconnect` to `fonts.googleapis.com` and `fonts.gstatic.com`.
- CSS/JS inlined into the HTML (no blocking subresources).
- No external images at render time (icons/favicon/og are local files).
- 17 theme PDFs/EPUBs are pre-rendered and served as static files — the
  "download these docs as a PDF" button is a plain link, not a job.

## Verifying

```bash
# Headers on an HTML page (Cloudflare Pages / Netlify):
curl -sI https://your-site.example.com/docs.html

# Compression (look for content-encoding: br or gzip):
curl -sI -H "Accept-Encoding: br,gzip" https://your-site.example.com/docs.html

# Full Lighthouse / PageSpeed run:
npx lighthouse https://your-site.example.com/ --view
```

On GitHub Pages, `curl -I` shows GitHub's default `Cache-Control: max-age=600`
and no `content-encoding` — expected and documented above.

## Measured performance

End-to-end rendering numbers (3,000 docs → 3,535-page PDF in ~23 s, etc.)
live in [BENCHMARKS.md](../BENCHMARKS.md). The site itself builds in ~6 s
(17 theme PDFs + EPUBs rendered in parallel).

## Related

- [SEO.md](../SEO.md) — indexing runbook and Google/Bing submission steps
- `.github/workflows/docs.yml` — the deploy pipeline

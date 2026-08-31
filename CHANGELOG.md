# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Kindle output** (`pretty-pdf kindle`, or `build --format kindle`/`pdf,epub,kindle`): builds the same EPUB the `epub` command produces, then converts it to a Kindle-ready MOBI (default) or AZW3 file via Calibre's `ebook-convert` — Amazon retired its own KindleGen tool in 2022, so Calibre is now the standard converter. Calibre is not bundled; install it separately and put `ebook-convert` on `PATH`, or pass `--calibre-path` / `PRETTY_PDF_CALIBRE_PATH`. New Go API: `prettypdf.FormatKindle`, `WithCalibreExecPath`, `PDF.RenderKindle`/`NeedsCalibre`, and the standalone `kindle` package (`kindle.Write`, `kindle.ResolveCalibre`).
- **`pretty-pdf analyze`**: a static content analyzer, no Chrome or Calibre required, that flags patterns which render poorly (or break outright) across PDF, EPUB, and Kindle — distinct from `check`'s frontmatter/structure validation. Reports three severities: **errors** (`broken-internal-anchor`, `duplicate-element-id`, `image-file-not-found`), **warnings** (`heading-level-skip`, `multiple-h1`, `wide-table`, `image-missing-alt`, `long-code-line`, `deep-list-nesting`, `long-chapter-no-subheadings`), and **improvements** (`no-headings`, `image-external-url`, `image-oversized`, `no-tags`). Supports `--strict` (fail on warnings too), `--json`, and threshold flags (`--max-table-columns`, `--max-code-line-length`, `--max-list-depth`, `--long-chapter-words`). New Go API: the `analyze` package (`analyze.Analyze`, `analyze.Issue`, `analyze.Severity`, `analyze.Options`).

## [0.11.0] - 2026-08-12

### Added

- **More exports from `scripts/docsgen`**: one EPUB per builtin theme (`go-pretty-pdf-docs-<theme>.epub`) plus a canonical default, raw markdown sources (`README.md`, `docs.md`, `CHANGELOG.md`), a client/agent search index (`docs-search.json`), a machine-readable build manifest (`version.json`), a plain-text `sitemap.txt`, and a full performance report (`report.json`).
- **Aggressive performance reporting in `docsgen`**: the summary now shows wall time, artifact count/size, throughput (artifacts/s and MiB/s), phase-sum parallelism ratio, per-group artifact tables with mean/p95/min/max/stddev, and an ASCII-bar phase timeline. `--bench` additionally re-renders the theme PDFs at `jobs=1` to headline the real parallel speedup, and `_site/report.json` persists every metric in machine-readable form.
- **Ten new quality-audit checks**, expanding the automatic PDF audit from 6 rules to 16:
  - `overflow-y` — content taller than a fixed-height box, clipped on print.
  - `image-low-res` — an image rendered at more than ~2x its intrinsic width, so it will look pixelated on paper.
  - `broken-anchor` — an `<a href="#fragment">` with no matching element (dead in-document links break the TOC and PDF bookmarks).
  - `duplicate-id` — the same `id` attribute used twice.
  - `toc-mismatch` — a TOC entry with no target id, or a body section with no TOC entry.
  - `font-load-fail` — a requested font family the browser can't resolve (missing local font, or a Google Font blocked by the default network lockdown).
  - `page-break-inside-risk` — a table/code block without `page-break-inside: avoid`, which print can slice mid-row.
  - `line-break-risk` — a block with `orphans`/`widows` below 2, which can strand a single line at the top/bottom of a page.
  - `unused-component` — a component registered via `WithComponent()` that no document used (a typo'd tag is the usual cause).
  - `pdf-eof-missing` — the finished PDF lacks its `%%EOF` marker (truncated/corrupt output).
- **`render.SeverityError`** and the corrupt-output checks now report at `error` severity, so callers can distinguish "the PDF is genuinely corrupt" from advisory layout warnings (`pdf-empty`, `pdf-eof-missing`, `page-count`).
- **WCAG 2.2 contrast thresholds** in `low-contrast`: 4.5:1 for normal text and 3:1 for large text (≥18.66px, or ≥14px bold), replacing the old flat 2.2:1 cutoff that only caught obviously unreadable pairs.
- **`mdx.Parser.ComponentUsage()` / `ComponentNames()` / `ResetComponentUsage()`** so callers can inspect which custom components a parse actually exercised, and **`render` PDF-byte auditing** now tolerates Chrome's trailing-newline `%%EOF`.
- **`render.NewBrowser`, `render.RenderToPDFWithAuditBrowser`, and `prettypdf.WithSharedBrowser`**: boot one headless Chrome allocator (with startup-tuned flags) and render many documents against it instead of launching a fresh Chrome per `Build`. Useful for keeping a persistent browser across a batch of renders; measured on this machine it does not beat running each render concurrently, which remains the main parallelism lever. The new browser setup also became the default for `RenderToPDFWithAuditContext`, so every render picks up the reduced-startup flag set.
- **Large-document audit sampling**: the DOM audit's style-reading checks (`overflow`, `low-contrast`, `page-break-inside-risk`, `line-break-risk`, `heading-clip-risk`, TOC coverage, font-load) now sample the first few thousand matching elements instead of walking the entire tree, since layout reads (`getComputedStyle`, `scrollWidth/Height`) dominate on documents with tens of thousands of nodes and the findings are deduped/capped anyway. Documents render far faster at scale (measured: 5,000-document PDF went from >120s timeout to ~60s).
- **Speed flags for very large documents**: `--no-outline` (`prettypdf.WithGenerateDocumentOutline(false)`) and `--no-tagged-pdf` (`prettypdf.WithGenerateTaggedPDF(false)`) disable Chrome's post-print outline/bookmark build and accessibility tagging, the two most expensive post-print steps. Measured on a ~500-page document: both off is ~30% faster than both on (~1.1s vs ~1.6s); on docsgen's full theme-PDF set they cut wall time ~10%. Defaults keep both on — bookmarks and PDF/UA accessibility are features.
- **Full CLI help overhaul**: every command's `--help` now explains what it does, exit status where relevant, performance guidance for large books, and realistic example commands; `theme --help` lists all 17 built-in theme names.
- **`_headers` deploy config**: the generated site now ships a `_headers` file (per-path `Cache-Control` + security headers incl. CSP) that Cloudflare Pages and Netlify honor out of the box.

### Changed

- The audit report's `unused-component` finding is attached to `PDF.LastAudit()` after `Build`, so the same report carries both visual and authoring signals.
- `version` now respects `--no-color` and prints a single `v` prefix (it previously rendered the default `v0.10.0` as `vv0.10.0`).
- The docs page shares the landing page's appbar (brand, links, theme dropdown, GitHub button, mobile hamburger) and its sidebar, hero, and content styling were aligned with the landing page's visual language; the docs default theme is now `default`.

### Fixed

- The `duplicate-id` message used the raw DOM node in its text; it now names both elements (`#dup` on … (also on `#dup`)).
- The new `%%EOF` check originally flagged Chrome's well-formed output because Chrome appends a newline after the marker; the check now tolerates trailing whitespace.
- `docsgen`'s `report.json` could panic with `index out of range` when computing the fastest/slowest artifact names (indices into the ok-durations slice were confused with indices into the results slice); the scan now tracks names directly.
- **The docs site's theme switcher showed `classic`'s palette for `default`/`minimal`/`modern`**: the generated `:root` fallback rode on `classic`, whose block sits mid-list, so its unconditional `:root` rule (same specificity as the `[data-site-theme]` blocks, later in source order) overrode the earlier palettes. The `:root` fallback now rides on `default`, the first theme in `theme.List()` order.
- **CLI-reference fidelity**: corrected the documented flag name `--formats` (the flag is `--format`), the `lint.max_heading_depth` default (5, not 3), and added the undocumented `pdf-empty` audit check to the audit table.

## [0.10.0] - 2026-07-16

### Added

- **Multi-format `build`** (`--format pdf,epub`): render PDF and EPUB in a
  single pass from one source. A base `--out` name (no extension) is given
  both extensions automatically, and an `out.pdf`/`out.epub` value pins the
  matching format while deriving the other. New library option
  `prettypdf.WithFormats(...)` and the standalone `pretty-pdf epub` command
  share the same pipeline.
- **SVG and WebP cover images** for `--cover-image`/`render.cover_image`,
  alongside the existing PNG/JPG/JPEG formats.
- **Custom paper dimensions** in `render.paper` (and the matching CLI
  plumbing): `6x9in`, `6x9`, `6in x 9in`, `152.4mm x 228.6mm` — exact
  print-on-demand trim sizes beyond the named `letter`/`legal`/`A4`.

## [0.9.0] - 2026-07-15

### Added

- **`PDF.Warnings()`**: returns the non-fatal configuration warnings recorded by `New` (an unresolvable theme name, an unreadable `--css`/`--template` file) — each of these already printed to stderr, but had no way to be detected programmatically without scraping output.

### Fixed

- **A checksum-mismatched Chrome download was left behind on Windows instead of being removed**: `downloadFile`'s "remove the partial/corrupt file on error" cleanup was registered as a `defer` *before* the "close the file handle" `defer`, so it ran first (defers execute LIFO) and tried to delete the destination file while it was still open — a no-op on POSIX, but Windows refuses to delete an open file, so the corrupt download stuck around for a later step to unwittingly pick up. The defers are now registered in the correct order.
- **`New()` swallowed configuration warnings unless `--verbose`/`WithVerbose(true)` was set**: a typo'd theme name or an unreadable `--css`/`--template` file silently fell back to the previous value with zero indication anything was wrong, since the warning was only ever printed to stderr behind the verbose flag. Warnings are now always printed; see `PDF.Warnings()` above for programmatic access. `New()`'s `error` return is unchanged — an unresolved theme is still non-fatal by design.
- **`{{var}}` substitution was non-deterministic and could chain into other variables**: `substituteVars` replaced one variable at a time by looping over the vars map (Go's map iteration order is randomized) and doing a sequential `ReplaceAll` on the *already-substituted* text, so a variable whose value itself contained another variable's `{{placeholder}}` could get expanded a second time — the same input could render differently across runs. Substitution is now a single `strings.Replacer` pass over the original text, so every placeholder expands exactly once, deterministically. `Parser.vars` is also now mutex-guarded for concurrent `SetVars`/`ParseFile` use.
- **Nested components of the same type left a stray closing tag**: `<Warning><Warning>...</Warning></Warning>` only matched the innermost pair (the transpile regex is intentionally non-greedy), leaving a literal `</Warning>` in the rendered output. Same-name nesting is now unwound innermost-first by repeating the replacement until no match remains. Component tags also now accept single-quoted attribute values and attributes other than `title` (e.g. `<Warning id="x" title='Heads up'>`), instead of only the exact literal `title="..."` shape.
- **The native header/footer strip could pick up colors from document body text, not just the real theme**: `pageChrome` extracted `--pdf-*` CSS variables from the *entire* composed document (cover, TOC, every chapter), and that parser matches the pattern wherever it appears — so a chapter merely discussing or showing that CSS syntax (e.g. a code sample) could silently override the theme's actual background/muted-text color used to paint the header/footer. Vars are now extracted from the document's `<style>` block only.
- **A custom theme's raw CSS (or a `--css` file) could break out of the `<style>` block**: unlike the color/font override values `theme.Resolve` already sanitizes, a `.theme.yml`'s `css:` escape hatch and `--css`/`--template` file content were inserted into `<style>{{.CSS}}</style>` completely unescaped (by design, so legitimate CSS survives intact) — including the literal sequence `</style`, which would end the style element early and let arbitrary markup follow. That sequence is now neutralized before insertion; nothing else in the CSS is touched.
- **A failed EPUB rebuild could destroy a previously good `.epub` file**: `epub.Write` opened `outputPath` directly with `os.Create`, truncating it immediately, before anything had actually been written successfully — a failure partway through (a bad chapter, a full disk, an interrupted process) left a corrupt/incomplete archive in its place. `Write` now builds the archive in a temp file and renames it into place only on success, matching the same pattern already used for cover+body PDF merging.
- **Canceling a `context.Context` mid-`Build`/`Validate` had no effect until the render stage**: `Build` never checked `ctx` between parsing, validation, and composing, and `Validate` accepted a `ctx` parameter but never referenced it at all. Both now check `ctx.Err()` between stages (and per-document during validation) and stop early once canceled.
- **`--cover-image` had no upper bound on file size or pixel dimensions**: the image was read fully into memory and its decoded pixel dimensions passed straight through as `PrintToPDF`'s exact paper size, so an unusually large file — or one whose header merely claims huge dimensions — could trigger an oversized in-memory read and an oversized print request. Now capped at 32MB and 20000px per side.
- **Auto-downloaded Chrome builds silently skipped their integrity check when GCS omitted the MD5 header**: `chromemgr.downloadFile` verifies the download against the `X-Goog-Hash` GCS reports, but when that header was absent it fell through with no message at all, even though the file is about to be `chmod +x`'d and executed. Now surfaced as a warning via the existing progress callback.
- EPUB's `renderCoverXHTML`/`renderNavXHTML` template errors are now wrapped with context, matching `renderChapterXHTML`.

## [0.8.0] - 2026-07-15

### Added

- **Custom cover image** (`--cover-image`/`render.cover_image`): a PDF cover built from a `.png`/`.jpg`/`.jpeg` file, full-bleed and sized to exactly the image's own pixel dimensions (a square image gets a square cover page) instead of the document's configured paper size — the rest of the document is unaffected. `Page.printToPDF` only accepts one paper size per call, so the cover is printed as its own single-page PDF and stitched in front of the normally-rendered body via a raw page merge (new dependency: `github.com/pdfcpu/pdfcpu`, which preserves each side's own page size rather than forcing a uniform one). Replaces the theme's text cover outright, regardless of `theme_options.sections.cover` or option/config ordering. New library API: `prettypdf.WithCoverImage`, `render.Options.CoverImagePath`, `config.RenderConfig.CoverImage`.
- **`epub` command**: converts the same MDX source into a single EPUB 3 file — no headless Chrome involved at all, unlike `build`. Each document becomes its own XHTML chapter in reading order; the frontmatter `[X.Y.Z]` ID hierarchy that drives the PDF's table of contents now also drives a real nested `nav.xhtml` outline (plus a flattened `toc.ncx` for older readers). `--cover-image`/`render.cover_image` is shared with `build`: the same image becomes a standalone full-bleed cover page. New dependency-free package `github.com/sazardev/go-pretty-pdf/epub` (`epub.Write`), building the archive with only the standard library (`archive/zip`, `crypto/rand` for the required `dc:identifier` UUID). goldmark's renderer now emits XHTML-style self-closing void elements (`<img/>`, `<hr/>`, `<br/>`) globally — still valid HTML5, so the PDF pipeline is unaffected, but required for EPUB's chapter files to be well-formed XHTML.

### Fixed

- **`build` silently failed on large books with `chromedp render: page load error net::ERR_ABORTED`**: rendering navigated Chrome to a `data:text/html;charset=utf-8;base64,...` URI holding the entire composed document, which Chrome (via chromedp/CDP's `Page.navigate`) aborts once the encoded payload crosses roughly 2MB — with no message indicating size was the cause. A book with a few hundred thousand words of prose and code crosses that threshold easily. Rendering now writes the composed HTML to a temporary file and navigates to it via a `file://` URL instead (removed once the page is captured), which has no such ceiling; the existing default-network-blocking behavior (`NetworkAccess: false` still blocks external `http(s)://` resources) is unaffected. No public API changed. New regression test `TestRenderToPDFLargeDocumentPastOldDataURILimit` generates a document past the old limit and confirms it renders.
- **`context.Context` cancellation was ignored during rendering**: `PDF.Build(ctx)` never propagated `ctx` into the Chrome render — canceling it (client disconnect, SIGINT wired to context cancellation) had no effect and the render ran to completion or `Options.Timeout` regardless. New `render.RenderToPDFWithAuditContext(ctx, ...)` roots the Chrome allocator in the caller's context; `Build` now uses it. `render.RenderToPDF`/`RenderToPDFWithAudit` keep their exact previous signatures (rooted in `context.Background()`) for API stability.
- **Data race on live-reload's SSE channel** (`pretty-pdf serve`): an `/events` connection read `reloadCh` with no lock while a rebuild concurrently closed and reassigned it, which could make an in-flight connection observe a stale, already-closed channel and never see a subsequent reload. Also hardened `notifyReload` itself to close-and-replace under a single lock instead of a separate read-then-write pair, which could otherwise panic with "close of closed channel" under concurrent calls.
- **Data race on watch-mode build stats** (`pretty-pdf watch`): `WatchStats` was mutated from the rebuild loop and read from the Ctrl+C signal handler with no synchronization. Now guarded by a mutex.
- **A failed `--cover-image` merge could destroy a previously good PDF**: `mergeCoverAndBody` truncated the output file immediately via `os.Create`, before the pdfcpu merge had actually succeeded. A merge failure partway through (malformed cover PDF, pdfcpu error) left an empty/partial file in place of the last good output. Now written to a temp file and renamed into place only on success.

### Security

- **HTML injection via the `id` frontmatter field**: `mdx.AnchorID` passed the `id` field straight through into `id`/`href` HTML attributes with no escaping (`compose.ComposeHTML`/`buildTOC` bypassed `html/template`'s autoescaping by building markup with `fmt.Fprintf` + `template.HTML`). A crafted `id` like `1.0.0"><script>...` could inject arbitrary markup into the composed document. `AnchorID` now sanitizes to a safe HTML-id charset at the source, plus defense-in-depth escaping at both call sites.
- **CSS injection via theme color/font overrides**: `theme.Resolve` interpolated `Options.Colors`/`Options.Fonts` values (and Google Fonts family names) straight into generated CSS/`url(...)` with no escaping, letting a value containing `;`/`{`/`}`/`'`/`)` break out of its declaration and inject arbitrary rules (e.g. re-enabling a hidden `.cover`). Values are now sanitized before use.
- **Malformed/malicious EPUB chapter content**: goldmark's raw-HTML passthrough (`WithUnsafe`) means an MDX author's literal `<br>` or `&nbsp;`/`&mdash;` reaches `epub.Write` unmodified — valid enough for Chrome's lenient HTML parser (the PDF path), but not well-formed XML, breaking strict EPUB readers. Chapter bodies are now reparsed/re-serialized through `x/net/html` before templating, which always self-closes void elements and resolves entities to literal text.
- **Auto-downloaded Chrome binary had no integrity check**: `chromemgr.EnsureChrome`'s one-time download fetched `chrome-headless-shell` over HTTPS with no verification before `chmod +x` and executing it. The Chrome for Testing manifest publishes no signed checksum, but the binaries are served from Google Cloud Storage, which reports the object's own MD5 via `X-Goog-Hash`; the download is now checked against it (catches transport corruption and a class of tampering proxy), and a mismatched/corrupt download is removed rather than left on disk.
- **`golang.org/x/image` bumped to v0.43.0**: fixes two known panics decoding a malformed/large WEBP image (GO-2026-5061, GO-2026-4961), reachable via `--cover-image`'s dimension probing (`render.coverImageDimensionsIn`).
- **`golang.org/x/net` bumped to v0.55.0**: the new `epub.xhtmlifyFragment` (above) calls `x/net/html`'s `ParseFragment`/`Render`, which at v0.54.0 carried five known issues (GO-2026-5025/5027/5028/5029/5030 — a DoS parsing arbitrary HTML, an XSS via duplicate attributes, and related HTML-parsing correctness bugs) in the exact code path `xhtmlifyFragment` exercises on every chapter's body.

## [0.7.0] - 2026-07-07

### Added

- **`gruvbox` builtin theme** (17 total): retro warm dark palette inspired by the popular Gruvbox editor theme (`#282828` background, orange `#fe8019` accent, monospace throughout), requested by name.
- `theme.ExtractCSSVars(css)`: shared parser for `--pdf-*` custom property declarations, extracted out of `scripts/docsgen` so `render` can reuse it too.
- **Automatic PDF quality audit**: `build` now runs a best-effort visual/structural audit right after rendering and reports what it finds — advisory only, it never fails the build. Checks: `overflow-x` (content wider than its box that print will clip instead of wrap), `broken-image`, `empty-content` (near-zero visible text, usually a sign composition silently produced nothing), `low-contrast` (visible text too close in color to its effective background), `heading-clip-risk` (a heading that forces a page break without enough top margin to clear the print engine's header strip — the general form of the bug fixed below, now caught automatically for custom themes/CSS too), and `page-count` (the generated PDF has no detectable pages). Surfaced as a real `Warnings` count and itemized list in `build`'s terminal output and `--json`'s `warnings` array. New library API: `render.RenderToPDFWithAudit` (returns an `*render.AuditReport` alongside the existing error; `render.RenderToPDF`'s signature is unchanged) and `PDF.LastAudit()`.

### Fixed

- **Dark themes no longer print with a white border**: `page.PrintToPDF`'s margin area sits outside the page's paintable content box, so Chrome never fills it with the document's own background — every dark theme (`dark`, `blueprint`, `gruvbox`, ...) rendered as a dark rectangle floating inside a plain white page. Left/right margins are now 0 (the reading margin moved to CSS `padding` on `<body>` instead, which the theme's background paints straight through for true edge-to-edge bleed), and the native top/bottom margin — kept only because that's the one place the running header/page-number footer can render — is now painted with the theme's own `--pdf-bg`/`--pdf-muted` instead of hardcoded white/gray, so it blends into the page instead of showing up as a separate band. `displayHeaderFooter` is now only enabled when a header or page numbers are actually wanted, so documents with both disabled get a fully clean, gap-free page.
- **`@page { margin: ... }` in `base.css` was silently overriding every configured margin**: this Chromium version honors an `@page` margin (even `margin: 0`) over `Page.printToPDF`'s imperative `marginTop`/`marginBottom`/`marginLeft`/`marginRight`, so `render.Options` and `go-pretty-pdf.yml`'s `render.margin_*` had no visible effect whensoever they disagreed with `@page` — silently, with no error. `@page` no longer declares a margin at all; the imperative API parameters are now the single real source of truth.
- **`margin_top`/`margin_bottom`/`margin_left`/`margin_right: "0mm"` in `go-pretty-pdf.yml` was silently ignored**: `WithFullConfig` detected "was a margin configured?" from the *parsed* value being non-zero, making an explicit `0mm` indistinguishable from the field being absent — a config asking for a true full-bleed page got the default margins instead. Now gated on the config *string* being non-empty.
- **Chapter titles and the "Table of Contents" heading rendered clipped, overlapping the running header**: every `h1` that starts a fresh page (`page-break-before: always`, or the TOC's own heading right after the forced break following the cover) had `margin-top: 0`, putting its text flush against the top of the page's content box. With a header/page-number footer enabled, chrome-headless-shell clips roughly the first 0.3in of whatever sits there — confirmed by disabling the header (clean render) and by testing plain body text landing on a fresh page through natural pagination instead of a forced break (also clean), so the defect is specific to content flush against a *forced* page break while a header/footer is displayed. `h1` now keeps a `0.35in` top margin, comfortably clearing the dead zone on every theme (none of which override it), guarded by `TestBaseCSSH1HasTopMarginBuffer`.

Known limitation: Chrome reserves a small, fixed ~0.2in strip at the very top/bottom of the page whenever a header/footer is displayed at all, regardless of the configured margin or the header/footer template's own CSS — confirmed by direct testing, not something this project's CSS can override. It's colored to match the theme so it no longer looks like a stray white band, but on a dark theme with a header or page numbers enabled you may still see a hairline. Disable both (`--no-header --no-page-numbers`, or `theme_options.sections.header`/`page_numbers: false`) and set all four margins to `0mm`/`0in` for a page with zero gap on every side.

## [0.6.0] - 2026-07-07

### Added

- **8 new builtin themes**, bringing the total to 16 — each required only a CSS file and a registry entry in `theme/builtin.go` to appear correctly in the CLI (`pretty-pdf theme list`) and the docs website (switcher + its own dogfooded PDF), with zero other files touched:
  - `sepia` — warm, soft palette for long reading sessions
  - `terminal` — all-monospace, terminal-inspired, with a `$ ` prompt-style cover
  - `blueprint` — dark technical palette with cyan highlights and a dashed cover border
  - `ivy` — classic Ivy League university letterhead (forest green and gold)
  - `government` — formal official-document palette (navy and bronze, centered headings, double rules)
  - `resume` — clean ATS-friendly sans-serif for CVs; disables cover/TOC/page numbers/header by default
  - `legal` — stark, formal brief style with no color used as decoration
  - `latex` — mathematical/scientific paper look with automatic, chapter-scoped section numbering (1., 1.1, 1.2, 2., ...) via CSS counters
- Five new theme categories: `warm`, `technical`, `institutional`, `resume`, `formal`.
- `theme.Theme.Accented`: marks builtin themes (classic, modern, corporate, editorial, terminal, blueprint, ivy, government) that use their accent color as a bold structural element (cover border, accent blockquote) rather than just for links.
- `resumeSections`: a `ResolvedSections` preset (all sections off) for themes meant for short, single-flow documents.

### Changed

- **docs site theme automation**: `scripts/docsgen` now derives every theme's colors, fonts, and accent treatment straight from `theme.List()` and each theme's own CSS at build time (see `themevars.go`), instead of a hand-maintained, hardcoded copy of the palette in `site.css`. Adding a builtin theme to `theme/builtin.go` is now the only step needed for it to appear in the docs site's theme switcher, get correct swatch colors, and get its own dogfooded "docs as a PDF" download — nothing to keep in sync by hand. This is also what caused the "incoherent blue" bug fixed earlier: the site's copy had drifted from the real theme CSS.

## [0.5.0] - 2026-07-07

### Added

- **Automatic Chrome management** (`chromemgr` package): `pretty-pdf build`/`watch` no longer require Chrome/Chromium to be installed manually. If none is found, a small official "chrome-headless-shell" build (Google's Chrome for Testing distribution) is downloaded and cached under the OS user cache dir on first use, then reused on every later run — no different from what Playwright/Puppeteer do. A system-installed Chrome/Chromium is always preferred and used as-is when present. New `--chrome-path` flag / `PRETTY_PDF_CHROME_PATH` env var let users pin a specific binary and skip detection entirely. Covers linux/amd64, darwin/amd64, darwin/arm64, windows/amd64 (linux/arm64 has no upstream prebuilt binary yet and falls back to a clear error asking for a manual install).
- **`render.Options.ChromeExecPath`** and **`prettypdf.WithChromeExecPath`**: point rendering at a specific Chrome/Chromium binary instead of chromedp's default discovery.
- **Named theme constants**: `NameDefault`, `NameMinimal`, `NameModern`, `NameClassic`, `NameCorporate`, `NameDark`, `NameAcademic`, `NameEditorial` exported from `theme` package — custom theme code and tests can now reference themes by constant instead of raw strings, eliminating goconst lint warnings across the codebase

### Changed

- **Chrome startup timeout**: raised `chromedp.WSURLReadTimeout` to 45s in `RenderToPDF` and 15s in `CheckChromeAvailable`, plus boosted `CheckChromeAvailable` context timeout from 10s to 20s — prevents spurious "websocket url timeout reached" failures on cold/loaded CI runners

### Fixed

- **goconst lint**: all magic string literals for builtin theme names and categories replaced with named constants in `theme/builtin.go`; tests updated to use constants and share a `testCustomThemeName` const

## [0.4.0] - 2026-07-06

### Added

- **Theme engine overhaul**: `theme` package now has a proper engine with `Resolve()`, `ResolveByName()` — merges base CSS + theme CSS + `:root` custom property overrides (colors, fonts, density) + section toggles
- **8 builtin themes**: default, minimal, modern, classic, corporate, dark, academic, editorial — each with dedicated CSS files embedded via `//go:embed`, with categories (professional, editorial, dark, academic, minimal)
- **Custom theme system**: `<name>.theme.yml` files extending a builtin theme, discovered in `./themes/` (project-local) and `~/.config/pretty-pdf/themes` (global) — full YAML schema with `extends`, `colors`, `fonts`, `sections`, `density`, and raw `css` escape hatch
- **`theme` CLI subcommand** with 4 subcommands:
  - `theme list` — shows builtin + custom themes with descriptions
  - `theme show <name>` — prints fully-resolved CSS to stdout
  - `theme new <name>` — scaffolds a starter `.theme.yml` (with `--from` and `--global` flags)
  - `theme add <path>` — imports existing `.theme.yml` or `.css` files as managed custom themes (with `--as` and `--global` flags)
- **Section toggles** (cover, TOC, page numbers, header) controlled via:
  - CLI flags: `--no-cover`, `--no-toc`, `--no-page-numbers`, `--no-header`
  - Config: `theme_options.sections.cover`, `.toc`, `.page_numbers`, `.header` (nullable booleans — unset = theme default)
  - Template gating: `{{if .ShowCover}}` / `{{if .ShowTOC}}` wrapping the cover block and TOC in `template.html`
  - CSS gating: `.cover{display:none !important;}` / `.toc{display:none !important;}` appended by `Resolve()` for disabled sections
  - `render.Options.PageNumbers` and `render.Options.ShowHeader` — when disabled, Chrome header/footer templates render `<div></div>` (empty)
- **Color/font customization**: `--color-*` and `--font-*` CLI flags + `theme_options.colors`/`fonts` in config — drives `--pdf-*` CSS custom properties in a `:root` block
- **Density control**: `--density compact|normal|relaxed` CLI flag + `theme_options.density` — adjusts `--pdf-line-height` and `--pdf-space-scale`
- **Google Fonts support**: `fonts.google_fonts` in theme YAML/config, fetched only when `allow_network_fonts: true` (network disabled by default for security)
- **`WithThemeName(name, opts)` option**: resolves a theme by name (builtin, custom, or file path) with full opts customization, wiring section toggles into `composeOpts`/`renderOpts`
- **`WithNetworkAccess(bool)` wired into CLI**: `--allow-network-fonts` flag enables outbound Chrome requests
- **Config struct**: `ThemeOptionsConfig`, `ColorsConfig`, `FontsConfig`, `SectionsConfig` with full YAML serialization
- **Test suite**: 20 new tests across `theme/`, `pdf_test.go`, `compose/compose_test.go`, `render/render_test.go`, `config/config_test.go`

### Changed

- `build` and `theme` CLI commands now carry expanded help text with full `Long` descriptions, `Example` blocks, and dynamic theme name listing
- `README.md` theme section updated: lists all 8 builtin themes, documents `WithThemeName(name, opts)`, adds CLI usage examples for theme customization
- `theme.Theme` struct: now includes `Description`, `Category`, `Sections` (resolved defaults), and `CSS` comes from dedicated asset files instead of raw Go strings
- `WithTheme(t)` — now applies CSS only (no template); section toggles must be set separately
- `WithConfigCSSAndTemplate` — resolves `cfg.Theme` via `ResolveByName` with full `ThemeOptionsConfig` customization before applying explicit CSS/template file overrides
- Old hardcoded `theme.Minimal.CSS` inline string replaced by `//go:embed assets/minimal.css`
- `cmd/pretty-pdf/main.go` — `--theme` flag usage now lists all builtin theme names dynamically from `theme.List()`
- `cmd/pretty-pdf/config.go` — maps CLI flags to `cfg.ThemeOptions` (colors, fonts, sections, density, network)
- `docs/cli.md` — comprehensive docs for all new flags, config fields, theme subcommands, custom theme workflow

### Removed

- Inline `minimalCSS` string in `theme/theme.go` — CSS now lives in `theme/assets/*.css`

### Security

- Google Fonts (`fonts.google_fonts`) require explicit `allow_network_fonts: true` — network access remains blocked by default during headless Chrome rendering

## [0.3.0] - 2026-07-06

### Added

- `WithFullConfig(cfg)` option: applies the entire `config.Config` struct (source, output, title, subtitle, author, CSS/template, theme, vars, render settings) in a single call
- `WithNetworkAccess(bool)` option: control whether headless Chrome can make outbound network requests during rendering (default: `false`)
- `config.ParsePaperSize(name)` and `config.ParseCSSUnit(s)` exported functions (moved from `cmd/pretty-pdf/config.go`)
- `config.PaperLetter` constant for YAML config comparisons
- `render.Options.NetworkAccess` field, with default network blocking via `chromedp/cdproto/network`
- Concurrent-safe `ComponentRegistry` (`sync.RWMutex` protecting the handler map)
- `headerTitleSet` tracking in `PDF`: prevents `New()` from overwriting an explicit header title with the document title
- Deferred warning buffer in `PDF`: `WithConfigCSSAndTemplate` file-read failures are collected and flushed by `New()` after all options run, making warning output order-independent from `WithVerbose`
- Comprehensive test suite: `pdf_test.go` (16 tests), `compose/compose_test.go` (6 tests), `compose/toc_test.go` (3 tests), `config/units_test.go` (2 tests), `config/units_test.go` (2 tests), `mdx/component_test.go` (1 race test), `mdx/parser_test.go` (7 tests), `render/render_test.go` (4 tests)
- Showcase book: 8-document MDX example under `examples/showcase/` with 8 custom components (`Callout`, `Badge`, `Steps`, `Card`, `Stat`, `Timeline`, `Quote`, `Progress`)
- Showcase integration test: `examples/showcase_test.go` verifies compose output and full PDF rendering
- Trust model documentation in `README.md`, `SECURITY.md`, and package-level `doc.go`

### Changed

- `cmd/pretty-pdf/buildOpts` simplified to `WithFullConfig` + `WithValidator` (removed duplicated config-to-option mapping)
- `WithConfigCSSAndTemplate` file-read warnings now deferred to `New()` instead of printed inline (order-independent from `WithVerbose`)

### Security

- **Network access blocked by default** during headless Chrome rendering: prevents SSRF/exfiltration from untrusted MDX content via `<script>`, `<img>`, `<link>`, etc.
- Detailed trust model documented across `README.md`, `SECURITY.md`, and `doc.go`

## [0.2.1] - 2026-07-03

### Fixed

- **Data URI corruption**: switched from `url.QueryEscape` to base64 encoding for the HTML data URI passed to Chrome — `QueryEscape` converts spaces to `+`, which Chrome does not decode in data URIs, so every space in rendered text showed up as a literal `+`

## [0.2.0] - 2026-07-03

### Added

- `serve` subcommand: preview MDX as HTML in the browser with live reload
- `completion` subcommand: generate shell completion scripts (bash, zsh, fish, powershell)
- `--bare` flag on `init`: minimal non-interactive project scaffolding
- `--port` flag on `serve`: configure HTTP server port
- `ValidateAll(docs)` method on `PDF` and `Validator` interface for batch validation
- `TestAnchorID` test covering bracket-stripping behavior
- `.component-warning-title` CSS class for `<Warning>` title styling
- Template placeholders (`{{BOOK_TITLE}}`, `{{AUTHOR_NAME}}`, `{{SOURCE_DIR}}`, `{{THEME}}`) in init scaffold files
- Support for h4 and h5 heading levels in MDX documents, with matching TOC styling and CSS

### Changed

- `AnchorID` uses `strings.Trim` instead of `strings.ReplaceAll` for bracket removal
- `PrintValidationSummary` now shows per-file breakdown (passed/errored/warned)
- `render.DefaultOptions()` drives margin defaults in config instead of hardcoded 0.8
- `FindConfig` resolves from `os.Getwd()` instead of relative `.`
- Warning component uses `html.EscapeString` from stdlib instead of custom `escapeHTML`

### Removed

- `AnchorIDRaw` function (unused)
- `PrintSummary` method from `PipelineProgress` (unused)
- Deprecated `LintConfig` fields: `RequireIDFormat`, `RequireLowercaseFilenames`, `CheckBrokenLinks`
- Version banner from `init`, `check`, and `watch` commands (noise reduction)
- Custom `dirname` helper in render — replaced by `filepath.Dir`
- Per-doc validation loop in `build.go` — replaced by single `ValidateAll` call

### Fixed

- `url.PathEscape` → `url.QueryEscape` for correct data URI encoding in Chrome rendering

## [0.1.0] - 2026-07-02

### Added

- MDX parser based on goldmark with YAML frontmatter support
- Custom component transpiler: `<DeepDive>`, `<Warning>`, `<Axiom>` built-in
- Variable substitution (`{{key}}`) before parsing
- HTML composition with embedded template and print CSS
- Table of Contents auto-generated from `[X.Y.Z]` IDs
- Headless Chrome PDF rendering via chromedp
- Theme system with Default and Minimal built-in themes
- CLI with cobra: `build`, `check`, `init`, `watch`, `version` commands
- Animated pipeline UI with lipgloss styles, spinner, and progress panels
- Interactive init wizard using huh form library
- File watcher with 300ms debounce for live rebuilds
- YAML config file (`go-pretty-pdf.yml`) with auto-discovery
- Configurable lint validator: required frontmatter, ID format, duplicates, heading depth
- Paper size presets: A4, Letter, Legal
- CSS unit parser: mm, cm, in, pt, px
- Comprehensive example suite with custom components, themes, and CSS
- 18 functional options on the root PDF type
- Step-by-step API: ParseDir, ValidateDoc, ComposeHTML, Render
- Partial parsing: per-file errors collected, valid docs proceed
- GitHub Actions CI (lint, test, vet, build on 3 OS) and release pipeline (goreleaser)
- Local Makefile with lint, test, build, and release-dry-run targets

[0.11.0]: https://github.com/sazardev/go-pretty-pdf/releases/tag/v0.11.0
[0.10.0]: https://github.com/sazardev/go-pretty-pdf/releases/tag/v0.10.0
[0.9.0]: https://github.com/sazardev/go-pretty-pdf/releases/tag/v0.9.0
[0.8.0]: https://github.com/sazardev/go-pretty-pdf/releases/tag/v0.8.0
[0.7.0]: https://github.com/sazardev/go-pretty-pdf/releases/tag/v0.7.0
[0.6.0]: https://github.com/sazardev/go-pretty-pdf/releases/tag/v0.6.0
[0.5.0]: https://github.com/sazardev/go-pretty-pdf/releases/tag/v0.5.0
[0.4.0]: https://github.com/sazardev/go-pretty-pdf/releases/tag/v0.4.0
[0.3.0]: https://github.com/sazardev/go-pretty-pdf/releases/tag/v0.3.0
[0.2.1]: https://github.com/sazardev/go-pretty-pdf/releases/tag/v0.2.1
[0.2.0]: https://github.com/sazardev/go-pretty-pdf/releases/tag/v0.2.0
[0.1.0]: https://github.com/sazardev/go-pretty-pdf/releases/tag/v0.1.0

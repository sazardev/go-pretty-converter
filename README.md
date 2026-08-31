# go-pretty-pdf

[![Go Reference](https://pkg.go.dev/badge/github.com/sazardev/go-pretty-pdf.svg)](https://pkg.go.dev/github.com/sazardev/go-pretty-pdf)
[![CI](https://github.com/sazardev/go-pretty-pdf/actions/workflows/ci.yml/badge.svg)](https://github.com/sazardev/go-pretty-pdf/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sazardev/go-pretty-pdf)](https://goreportcard.com/report/github.com/sazardev/go-pretty-pdf)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Transform a directory of MDX files into a beautiful, print-ready PDF via headless Chrome — plus EPUB 3 and Kindle (MOBI/AZW3) output.

**Library + CLI.** Use it as a composable Go library or as a standalone command-line tool.

**Fast.** A 3,000-document book becomes a 3,535-page print-ready PDF in ~23 seconds — and
validating those 3,000 documents takes 142 ms. Measured, reproducible numbers in
[BENCHMARKS.md](BENCHMARKS.md).

## Install

### CLI (binary)

```bash
go install github.com/sazardev/go-pretty-pdf/cmd/pretty-pdf@latest
```

### Library

```bash
go get github.com/sazardev/go-pretty-pdf
```

### Requirements

- **Go 1.26+**
- **Chrome or Chromium** — optional, only for PDF output. If none is found on your system, `pretty-pdf` automatically downloads and caches a small headless-only Chrome build the first time you run it (like Playwright/Puppeteer do). Already have Chrome installed? It's used as-is, nothing is downloaded. Prefer to control this yourself? Pass `--chrome-path /path/to/chrome` or set `PRETTY_PDF_CHROME_PATH`. Auto-download currently covers linux/amd64, darwin/amd64, darwin/arm64, and windows/amd64 — on linux/arm64 (no official build exists yet) install Chromium via your package manager and point `--chrome-path` at it.
- **Calibre** — optional, only for Kindle output (`--format kindle` / `pretty-pdf kindle`). Not bundled or auto-downloaded — install it from [calibre-ebook.com](https://calibre-ebook.com/download) so `ebook-convert` is on your `PATH`, or point `--calibre-path` / `PRETTY_PDF_CALIBRE_PATH` at it.

## Quick start

### CLI

```bash
# Scaffold a new book project (interactive wizard)
pretty-pdf init my-book

# Build a PDF
pretty-pdf build --source my-book --out my-book.pdf

# Watch for changes and rebuild
pretty-pdf watch --source my-book --out my-book.pdf

# Build a Kindle-ready ebook (needs Calibre's ebook-convert on PATH)
pretty-pdf kindle --source my-book --out my-book.mobi

# Validate MDX files
pretty-pdf check --source my-book

# Flag content that will render poorly on PDF/EPUB/Kindle (errors, warnings, improvements)
pretty-pdf analyze --source my-book
```

### Library

```go
package main

import (
	"context"
	"log"

	prettypdf "github.com/sazardev/go-pretty-pdf"
)

func main() {
	pdf, err := prettypdf.New(
		prettypdf.WithSourceDir("./docs"),
		prettypdf.WithOutputFile("output.pdf"),
		prettypdf.WithTitle("My Documentation"),
		prettypdf.WithAuthor("Jane Doe"),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := pdf.Build(context.Background()); err != nil {
		log.Fatal(err)
	}
}
```

## How it works

```
MD/MDX files → Parse frontmatter & markdown → Transpile components → Compose HTML → Render PDF
```

1. **Parse** — goldmark parses `.md`/`.mdx` files with YAML frontmatter. Fenced code blocks (` ```go `, ` ```python `, ...) are syntax-highlighted via [Chroma](https://github.com/alecthomas/chroma), using a style paired to each theme's tone (e.g. Dracula for the `dark` theme, the Gruvbox style for the `gruvbox` theme, GitHub's light style everywhere else)
2. **Transpile** — custom components (`<DeepDive>`, `<Warning>`, `<Axiom>`) become styled HTML
3. **Compose** — HTML assembled with embedded template + CSS + auto-generated Table of Contents
4. **Render** — headless Chrome prints to PDF with headers, footers, and PDF bookmarks, then an automatic quality audit checks the result for overflowing content, broken images, low-contrast text, near-empty output, dead links, duplicate ids, broken TOC entries, unloaded fonts, at-risk page breaks, and headings at risk of being clipped by the print engine (see `pretty-pdf build`'s `Warnings` output, or `render.RenderToPDFWithAudit` in the library API)

Documents are sorted by their `[X.Y.Z]` frontmatter ID, not filename.

A `.md`/`.mdx` file doesn't strictly need a `---` frontmatter block either:
if one is missing entirely, `id` and `title` are generated automatically
from the filename, the same convention `.txt` uses below (`02-getting
-started.mdx` → id `[2.0.0]`, title "Getting Started"; no numeric prefix →
the next free major version, so it never collides with an explicitly
numbered doc). The content itself still gets full markdown rendering —
components, raw HTML, everything — unlike `.txt`. A `---` block that *is*
present but fails to parse as YAML is still a hard error, so a typo in
real frontmatter doesn't silently get treated as "no frontmatter".

`.txt` files are also accepted for freeform writing with zero setup: they
have no frontmatter, so `id` and `title` are generated automatically from
the filename (`03-field-notes.txt` → id `[3.0.0]`, title "Field Notes";
no numeric prefix → the next free major version, so it never collides
with an explicitly numbered doc). Content is treated as literal text —
blank lines become paragraphs, single line breaks become `<br>` — and is
always HTML-escaped, so `.txt` loses component/raw-HTML customization but
in exchange never executes anything the author typed. Good for a quick
note dropped into an otherwise structured `.mdx` book.

## Trust model

MDX is parsed with raw HTML passthrough enabled, and custom components
don't escape their inner content — this lets authors embed arbitrary
HTML/CSS for rich documents, but it also means a `.md` or `.mdx` file can
contain a `<script>` tag that will execute during rendering. By default,
headless Chrome's network access is blocked while rendering (see
`WithNetworkAccess`), so scripts can't exfiltrate data or fetch remote
content — but they still run. **Only build PDFs from source files you
trust.** See [SECURITY.md](SECURITY.md)
for details.

## Performance

`go-pretty-pdf` keeps the Go-side pipeline (parse + validate + compose)
sub-second even on large books; headless Chrome's print step is what drives
PDF wall time. Measured on an i5-13500 (WSL2) with Chromium 150:

| Source docs | `check` | `epub` | PDF pages | PDF wall time | PDF throughput |
|------------:|--------:|-------:|----------:|--------------:|---------------:|
| 100         | 30 ms   | 27 ms  | 121       | 0.57 s        | 211 pages/s    |
| 1,000       | 62 ms   | 88 ms  | 1,180     | 2.99 s        | 394 pages/s    |
| 3,000       | 142 ms  | 248 ms | 3,535     | 23.02 s       | 154 pages/s    |

Full table, hardware, and a reproducible recipe:
[BENCHMARKS.md](BENCHMARKS.md). To measure your own machine:

```bash
PRETTY_PDF_CHROME_PATH=/usr/bin/chromium go run ./scripts/benchmark --color=false
```

## MDX format

```mdx
---
id: "[1.0.0]"
title: "Getting Started"
subtitle: "A simple introduction"
tags: [example, intro]
difficulty: "beginner"
status: complete
completeness: 100
depends_on: []
---

# Welcome to Your Book

This is the first chapter.

## Variables

You can use {{key}} syntax for variable substitution: running {{product}} v{{version}}.
```

Required frontmatter fields: `id` (format `[X.Y.Z]`), `title`.

## Built-in components

| Component | Usage | Appearance |
|-----------|-------|------------|
| `<DeepDive>` | `<DeepDive title="Details">...</DeepDive>` | Blue info panel |
| `<Warning>` | `<Warning title="Note">...</Warning>` | Orange warning panel |
| `<Axiom>` | `<Axiom>...</Axiom>` | Green italic quote |

Register custom components via `WithComponent()`:

```go
prettypdf.WithComponent("Callout", func(attrs map[string]string, inner string) string {
	level := attrs["level"]
	return fmt.Sprintf(`<div class="callout callout-%s">%s</div>`, level, inner)
})
```

## Configuration

Create a `go-pretty-pdf.yml` in your project:

```yaml
title: "My Book"
subtitle: "A Complete Guide"
author: "Jane Doe"
source: book
output: out.pdf
theme: default

css: custom.css
template: custom-template.html

vars:
  product: "go-pretty-pdf"
  version: "1.0"

lint:
  require_frontmatter: [id, title]
  require_id_format: "[X.Y.Z]"
  no_duplicate_ids: true
  max_heading_depth: 3

render:
  timeout: 60s
  paper: a4
  margin_top: 20mm
  margin_bottom: 20mm
  margin_left: 15mm
  margin_right: 15mm
  header_title: "{{title}}"
```

## Library API

```go
// Constructor with functional options
pdf, err := prettypdf.New(opts...)

// All-in-one build pipeline
pdf.Build(ctx)

// Step-by-step pipeline
docs, _ := pdf.ParseDir()
errs := pdf.ValidateDoc(doc)
html, _ := pdf.ComposeHTML(docs)
pdf.Render(html)

// Quality audit from the most recent Build/Render call (nil if neither ran yet)
audit := pdf.LastAudit()

// Validation-only
errs, _ := pdf.Validate(ctx)

// Lower-level: render straight to PDF and get the audit report back
report, err := render.RenderToPDFWithAudit(html, "out.pdf", render.DefaultOptions())
```

### Available options

| Option | Description |
|--------|-------------|
| `WithSourceDir(dir)` | MDX source directory (default: `book`) |
| `WithOutputFile(path)` | Output PDF path (default: `out.pdf`) |
| `WithTitle(title)` | Document title |
| `WithSubtitle(sub)` | Document subtitle |
| `WithAuthor(author)` | Document author |
| `WithCSS(css)` | Custom CSS content string |
| `WithTemplate(html)` | Custom HTML template string |
| `WithTheme(t)` | Apply a raw `theme.Theme` (no customization/section toggles) |
| `WithThemeName(name, opts)` | Resolve a theme by name (builtin, custom, or file path) with color/font/section customization |
| `WithComponent(name, handler)` | Register custom MDX component |
| `WithValidator(v)` | Custom validation logic |
| `WithTimeout(d)` | Chrome render timeout (default: 60s) |
| `WithHeaderTitle(t)` | PDF header title |
| `WithVerbose(bool)` | Enable verbose logging |
| `WithVars(map)` | Variable substitution map |
| `WithRenderMargins(t,b,l,r)` | PDF margins in inches |
| `WithPaperSize(w,h)` | Paper size in inches |
| `WithConfig(cfg)` | Apply source/output/title/subtitle/author from config |
| `WithConfigCSSAndTemplate(cfg)` | Load CSS/template from config file paths |
| `WithFullConfig(cfg)` | Apply the entire config struct (source, CSS/template, theme, vars, render settings) in one call |
| `WithNetworkAccess(bool)` | Allow headless Chrome to make network requests while rendering (default: `false`, blocked) |

## Themes

Seventeen built-in themes, each a palette/typography layer over one shared
structural stylesheet — clean and professional by default, easy to
customize without writing CSS, and extendable with your own custom themes:

`default` &middot; `minimal` &middot; `modern` &middot; `classic` &middot; `corporate` &middot; `dark` &middot; `academic` &middot; `editorial` &middot; `sepia` &middot; `terminal` &middot; `blueprint` &middot; `ivy` &middot; `government` &middot; `resume` &middot; `legal` &middot; `latex` &middot; `gruvbox`

```bash
# Pick a theme, tweak colors/fonts/density, drop sections you don't want
pretty-pdf build --theme corporate \
  --color-primary "#0ea5e9" --font-heading "Georgia, serif" \
  --no-cover --no-page-numbers --density compact

# Scaffold your own reusable theme
pretty-pdf theme new my-report --from corporate
pretty-pdf theme list
```

```go
prettypdf.WithThemeName("corporate", theme.Options{
	Colors:   theme.Colors{Primary: "#0ea5e9"},
	Sections: theme.Sections{Cover: theme.BoolPtr(false)},
})
```

Custom themes live in `<name>.theme.yml` files (project-local `./themes/`
or a global themes directory) and `extends` a builtin theme. Full reference,
all customization fields, and the `pretty-pdf theme` command family:
see [docs/cli.md#themes](docs/cli.md#themes).

## CLI reference

```
pretty-pdf build     Build a PDF from MDX source files
pretty-pdf epub      Build an EPUB from MDX source files (no Chrome required)
pretty-pdf kindle    Build a Kindle-ready MOBI/AZW3 file (needs Calibre's ebook-convert)
pretty-pdf check     Validate MDX files without building
pretty-pdf analyze   Static cross-format rendering-quality analysis (no Chrome/Calibre)
pretty-pdf theme     List, inspect, and manage themes
pretty-pdf init      Scaffold a new book project (interactive wizard)
pretty-pdf watch     Watch for changes and rebuild automatically
pretty-pdf serve     Preview MDX as HTML with live reload (no Chrome required)
pretty-pdf version   Print the version number
```

Run `pretty-pdf <command> --help` for the full flag list of any command.

Global flags: `--config`, `--source`, `--verbose`, `--no-color`, `--quiet`

## License

MIT — see [LICENSE](LICENSE).

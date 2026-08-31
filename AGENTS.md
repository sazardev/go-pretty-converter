# AGENTS.md

## Overview

`go-pretty-pdf` transforms a directory of MDX files into a print-ready PDF, EPUB, and/or Kindle
(MOBI/AZW3) ebook via headless Chrome (Kindle via Calibre's `ebook-convert`, invoked separately).
It is both a Go library (`github.com/sazardev/go-pretty-pdf`) and a CLI tool.

## Commands

```bash
go run ./cmd/pretty-pdf build --source ./docs --out out.pdf   # build a PDF
go run ./cmd/pretty-pdf check --source ./docs                  # validate only (the command is `check`, not `validate`)
go run ./cmd/pretty-pdf analyze --source ./docs                # static cross-format rendering-quality analysis, no Chrome/Calibre
go run ./cmd/pretty-pdf epub --source ./docs --out out.epub    # EPUB build, no Chrome required
go run ./cmd/pretty-pdf kindle --source ./docs --out out.mobi  # Kindle build, needs Calibre's ebook-convert on PATH
go run ./cmd/pretty-pdf theme list                             # theme command family: list | show | new | add
go test ./...
go test ./mdx/... -run TestParserParseFile -v
make lint            # golangci-lint v2 (golangci-lint binary must be installed)
make test            # go test -race ./...
go run ./scripts/docsgen   # regenerate _site/ (gitignored GH Pages output, needs Chrome)
```

`--source` defaults to `book/` (the bundled demo book). CI runs `go mod tidy` with a
`git diff --exit-code` check, so keep `go.mod`/`go.sum` tidy (`make tidy`). Full CI order:
tidy → lint → test (-race, 3 OS matrix) → vet → vulncheck → build.

## Architecture

```
cmd/pretty-pdf/    CLI entrypoint (cobra): build, check, analyze, init, watch, serve, epub, kindle, theme, version, completion
pdf.go             Root package — public API: New(), Build(), ParseDir(), ComposeHTML(), Render(), Validate(), LastAudit()
mdx/               MDX parser (goldmark-based), custom component transpiler, validator interface
analyze/           Static cross-format (PDF/EPUB/Kindle) rendering-quality analysis over parsed docs, no Chrome/Calibre
compose/           HTML composition: TOC, go:embed'd template.html + print.css
render/            Chrome headless PDF rendering via chromedp + automatic quality audit
theme/             17 builtin themes over a shared base.css, custom .theme.yml themes, section toggles
epub/              EPUB builder (no Chrome), shared with render path via theme
kindle/            Kindle (MOBI/AZW3) builder: EPUB via epub/, converted through Calibre's ebook-convert
chromemgr/         Chrome binary resolution + auto-download (chrome-headless-shell)
config/            go-pretty-pdf.yml parsing, units (mm/in/pt) handling
```

### Pipeline

`Parse MDX` → `Transpile custom components` → `Compose HTML` (embed assets, TOC) → `Render PDF` (Chrome headless)

## Requirements

- **Go 1.26+**.
- Chrome is NOT strictly required: `chromemgr` auto-downloads a headless Chrome build on first
  render (resolution: `--chrome-path` / `PRETTY_PDF_CHROME_PATH` → system Chrome → cached → download).
  On `linux/arm64` no prebuilt exists, so Chrome must be installed and passed via `--chrome-path`.
- Render-path tests skip automatically when no Chrome is found; chromemgr download tests are gated
  behind `CHROMEMGR_INTEGRATION=1`.
- `make lint` enforces import grouping via goimports (`.golangci.yml`): stdlib + third-party first,
  then `github.com/sazardev/go-pretty-pdf` imports in a separate group. `examples/` and `bin/` are
  excluded from lint/format checks.

## Key conventions

- Documents are sorted by their `[X.Y.Z]` frontmatter `id`, **not** by filename or filesystem order.
- Frontmatter is optional: if a `---` block is entirely missing, `id`/`title` are generated from the
  filename (`02-getting-started.mdx` → `[2.0.0]`, "Getting Started"). A `---` block present but invalid
  YAML is still a hard error. `.txt` files are also accepted (no frontmatter, literal text, HTML-escaped).
- Built-in custom components: `<DeepDive>`, `<Warning>`, `<Axiom>`. Additional components registrable via `WithComponent()`.
- Embedded assets live in `compose/assets/`, `theme/assets/`, `epub/assets/` and load at compile time via `//go:embed` — no runtime file reads.
- Trust model: raw HTML is passed through and components don't escape inner content — a `.mdx` can run
  `<script>` during rendering. Network is blocked by default (`WithNetworkAccess(false)`), but only build
  PDFs from trusted sources.

## Doc conventions

`docs/cli.md` is hand-maintained (not generated) but is *consumed* by `scripts/docsgen` to build
`_site/`, so stale flags propagate into the site. Command flags live in `docs/cli.md`; theme tables
in `README.md` (count of builtin themes, etc.) drift easily — verify against `theme/assets/` before
claiming a number.

## Versioning & releases

SemVer with a `v` tag prefix; single source of truth is `version/version.go` (all build paths
inject it: Makefile via `git describe`, goreleaser via the tag). `make bump-patch|bump-minor|bump-major`
commits `version.go` and tags `vX.Y.Z` but does **not** touch `CHANGELOG.md` — move `[Unreleased]`
into a dated `## [x.y.z]` section first, and keep the footer version-links in sync. Tag pushes
(`v*`) trigger `.github/workflows/release.yml`. Full runbook: `CONTRIBUTING.md`.

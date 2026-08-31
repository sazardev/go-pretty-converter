# go-pretty-converter — System Memory

## Purpose
Transform MDX source files into print-ready PDF via headless Chrome.
Both a Go library (`prettyconverter`) and CLI tool (`pretty-converter`).

---

## Pipeline

```
Raw MDX → substituteVars() ({{var}}) → goldmark parse → Transpile custom components → Compose HTML (TOC + template + CSS) → Render PDF (headless Chrome)
```

---

## Package Map

```
cmd/pretty-converter/          CLI entrypoint (cobra) — build, check, init, version, watch
cmd/pretty-converter/output/   DX output package: lipgloss styles, spinner, banner, panels, pipeline progress
config/                  YAML config loader — Config struct, Load(), FindConfig(), Default()
pdf.go                   Root package prettyconverter — New(), Build(), Validate(), 18 functional options
                         ↑ step-by-step: ParseDir(), ValidateDoc(), ComposeHTML(), Render()
mdx/                     Parser (goldmark), component transpiler, DefaultValidator, Document type
                         ↑ partial parsing: ParseFileError, ParseErrors (collect per-file, continue)
compose/                 HTML composition — template.html + print.css (go:embed), TOC builder
render/                  Chrome headless PDF rendering via chromedp
theme/                   Theme struct with Default and Minimal built-in themes
version/                 Canonical version string (single source of truth, ldflags target)
scripts/bump/            SemVer bump script — reads/writes version/version.go
```

---

## Config File (`go-pretty-converter.yml`)

Auto-discovered in CWD (or via `--config` flag). Precedence: **CLI flags > config values > code defaults**.

### All Fields

```yaml
title: "My Book"
subtitle: "A Guide"
author: "Jane Doe"
source: book
output: out.pdf
theme: default

css: custom.css
template: custom-template.html

vars:
  api_version: "v2.1"
  company: "Acme Corp"

lint:
  require_frontmatter: [id, title]
  require_id_format: "[X.Y.Z]"
  no_duplicate_ids: true
  max_heading_depth: 3
  require_lowercase_filenames: false
  check_broken_links: false

render:
  timeout: 60s
  paper: a4
  margin_top: 20mm
  margin_bottom: 20mm
  margin_left: 15mm
  margin_right: 15mm
  header_title: "{{title}}"
```

### Defaults (when field is absent)

| Field | Default |
|-------|---------|
| `source` | `"book"` |
| `output` | `"out.pdf"` |
| `title` | `"Document"` |
| `author` | `"go-pretty-converter"` |
| `theme` | `""` (→ default theme) |
| Lint: require_frontmatter | `[id, title]` |
| Lint: no_duplicate_ids | `true` |
| Lint: max_heading_depth | `3` |
| Render: timeout | `60s` |
| Render: paper | `A4` (8.27×11.69 in) |
| Render: margins | top/bottom=0.8in, left/right=0.6in |

### CSS/Template Path Resolution

**Critical**: CSS and template file paths in the YAML config are resolved **relative to the config file's directory**, not CWD. This prevents breakage when running from a different working directory.

- `loadConfig()` at `main.go:111` resolves paths:
  - Config file paths (`cfg.CSS`, `cfg.Template`): joined with `filepath.Dir(configPath)`
  - CLI flag paths (`--css`, `--template`): resolved via `filepath.Abs()`
  - `FindConfig()` at `config/config.go:72` returns **absolute path** via `filepath.Abs()`
- **Gotcha**: CSS/template files must exist in config or be explicitly set via CLI; there's NO auto-detection of `custom.css` in the source dir

---

## CLI Commands

### `pretty-converter build`

Animated pipeline: banner → pre-flight → parse (spinner) → validate (spinner) → compose (spinner) → render (spinner) → build summary panel.

Flags:
| Flag | Type | Overrides config | Extras |
|------|------|-----------------|--------|
| `--config` | string | — | |
| `--source`, `-s` | string | `cfg.Source` | |
| `--out`, `-o` | string | `cfg.Output` | |
| `--title` | string | `cfg.Title` | |
| `--subtitle` | string | `cfg.Subtitle` | |
| `--author` | string | `cfg.Author` | |
| `--theme` | string | `cfg.Theme` | |
| `--css` | string | `cfg.CSS` | |
| `--template` | string | `cfg.Template` | |
| `--timeout` | string | `cfg.Render.Timeout` | |
| `--verbose`, `-v` | bool | — | |
| `--json` | bool | — | silent JSON output |
| `--no-color` | bool | — | ASCII-only output |
| `--quiet` | bool | — | suppress config display |

**Flow** (`build.go:19`):
```
runBuild()
  → output.NoColor() if --no-color
  → output.PrintBanner(version)          # ASCII art "GO → PDF"
  → runPreFlight(cfg)                    # check Chrome, source, output, CSS, template
  → output.PrintPreFlight(results)       # ✓/✗ per check
  → abort if any non-warning failure
  → NewPipelineProgress(4 steps)
  → for each step: Start (spinner) → work → Done/Fail
  → output.PrintBuildSummary(BuildStats)  # styled panel
```

Pre-flight checks (runPreFlight, `build.go:198`):
- Chrome/Chromium available (calls `render.CheckChromeAvailable()`)
- Source directory exists + has .mdx files
- Output directory writable (auto-creates if needed)
- CSS file exists (if configured, warning on fail)
- Template file exists (if configured, warning on fail)

**Step-by-step API** (`pdf.go`):
```
pdf.ParseDir()       → []*mdx.Document, error (partial: docs + err list)
pdf.ValidateDoc(doc) → []mdx.ValidationError (runs DefaultValidator per doc)
pdf.ComposeHTML(docs)→ string (HTML), error
pdf.Render(html)     → error (headless Chrome)
```

### `pretty-converter check`

Styled validation: spinner for parsing → results panel.

Flags: `--config`, `--source`, `--strict`, `--verbose`, `--json`, `--no-color`

- `--strict`: promotes heading depth warnings to errors
- `--verbose`: prints warnings during config loading

Output: colored summary `N error(s), M warning(s)` via `PrintValidationSummary`.
Exit code 1 if errors > 0.

### `pretty-converter init [dir]`

**Interactive mode** (default): huh form wizard asking for title, author, theme (default/minimal), source dir, confirms before scaffolding.

**JSON/bare mode** (`--json`): scaffold with all defaults, no prompts.

Scaffolds:
- `go-pretty-converter.yml` (default config)
- `[1.0.0]-introduction.mdx`
- `[1.1.0]-getting-started.mdx`
- `[1.1.1]-installation.mdx` (has `{{product}}` / `{{company}}` var example)

Fails if target directory already exists.

### `pretty-converter watch`

fsnotify recursive watcher. 300ms debounce. Animated banners per rebuild.

Ctrl+C handler prints summary panel (builds, errors, last build time).

Flags: same as `build` + `--config`.

### `pretty-converter version`

Prints `pretty-converter <version>`. Version source: `github.com/sazardev/go-pretty-converter/version.Version`.
Defaults to `"dev"` in `version/version.go`, override at build via `-ldflags "-X github.com/sazardev/go-pretty-converter/version.Version=X.Y.Z"`.

### Global Flags

| Flag | Affects |
|------|---------|
| `--json` | build, check, init — silent structured output |
| `--no-color` | all — ASCII-only (tearmenv.Ascii) |
| `--quiet` | build — suppress config info display |
| `--verbose`, `-v` | all |

---

## Core API (`pdf.go`)

### Options Table

| Option | Signature | Sets | Default |
|--------|-----------|------|---------|
| `WithSourceDir` | `(dir string)` | source | `"book"` |
| `WithOutputFile` | `(path string)` | output | `"out.pdf"` |
| `WithTitle` | `(t string)` | compose title | `"Document"` |
| `WithSubtitle` | `(s string)` | compose subtitle | `""` |
| `WithAuthor` | `(a string)` | compose author | `"go-pretty-converter"` |
| `WithCSS` | `(css string)` | compose CSS content | embedded `print.css` |
| `WithTemplate` | `(html string)` | compose template | embedded `template.html` |
| `WithTheme` | `(t theme.Theme)` | CSS + Template | default |
| `WithComponent` | `(name string, handler mdx.ComponentHandler)` | registers **(appends)** | DeepDive, Warning, Axiom |
| `WithValidator` | `(v mdx.Validator)` | validator | nil (none) |
| `WithTimeout` | `(d time.Duration)` | render timeout | 60s |
| `WithHeaderTitle` | `(t string)` | PDF header title | compose Title |
| `WithVerbose` | `(v bool)` | verbose logging | false |
| `WithVars` | `(vars map[string]string)` | parser var substitution | nil |
| `WithRenderMargins` | `(t,b,l,r float64)` | PDF margins (inches) | 0.8/0.8/0.6/0.6 |
| `WithPaperSize` | `(w,h float64)` | PDF paper (inches) | A4 (8.27×11.69) |
| `WithConfig` | `(cfg *config.Config)` | source, output, title, subtitle, author | — |
| `WithConfigCSSAndTemplate` | `(cfg *config.Config)` | CSS file, template file, theme | — |

### `WithConfigCSSAndTemplate` Details

Reads `cfg.CSS` and `cfg.Template` from disk using `os.ReadFile`. Errors are silently swallowed unless verbose is true (prints to stderr). Maps `cfg.Theme` to theme constants ("default" → `theme.Default`, "minimal" → `theme.Minimal`).

### `WithComponent` Fix

Originally replaced the entire parser (`p.parser = mdx.NewParser(...)`). Now calls `p.parser.RegisterComponent(name, handler)` which appends without losing previously registered components or built-in defaults.

### Options Order Matters

Options execute in the order they're passed to `New()`. `WithVerbose` should come **before** `WithConfigCSSAndTemplate` so verbose logging of file read errors works.

### `New()` Constructor

```
New(opts...)
  → Defaults: source="book", output="out.pdf", parser=NewParser(), composeOpts=DefaultOptions(), renderOpts=DefaultOptions()
  → Auto-sets: renderOpts.HeaderTitle = composeOpts.Title
  → Applies all opts in order
  → Returns *PDF, nil
```

### `Build(ctx)` Pipeline

```
parser.ParseDir(sourceDir)                  # walk .mdx files, parse, transpile, sort by [X.Y.Z]
  → if errors: log partial failures, continue with valid docs
  → if validator: validator.ValidateAll(docs)
  → compose.ComposeHTML(docs, composeOpts)   # TOC + template + CSS
  → render.RenderToPDF(html, output, renderOpts)  # headless Chrome
```

### `Validate(ctx)` Pipeline

```
parser.ParseDir(sourceDir)
  → if validator: validator.ValidateAll(docs)
  → return errors (no compose/render)
```

### Step-by-step API (added in DX overhaul)

```
pdf.ParseDir()       →  ([]*mdx.Document, error)
  Delegates to parser.ParseDir, returns partial docs + ParseErrors.
  Suppresses frontmatter-not-found per-file (not an error).

pdf.ValidateDoc(doc) →  []mdx.ValidationError
  Runs pdf.validator.Validate(doc) if set.

pdf.ComposeHTML(docs)→  (string, error)
  Delegates to compose.ComposeHTML(docs, pdf.composeOpts).

pdf.Render(html)     →  error
  Delegates to render.RenderToPDF with configured opts.
```

---

## MDX Parser (`mdx/parser.go`)

### Parser Struct

```go
type Parser struct {
    md         goldmark.Markdown
    components *ComponentRegistry
    vars       map[string]string
}
```

### ParseFile Flow
1. `os.ReadFile(path)`
2. `substituteVars(raw)` — replaces `{{key}}` with `vars[key]` before goldmark
3. Goldmark conversion (GFM, meta, unsafe HTML, auto heading IDs)
4. Extract frontmatter from goldmark context via `meta.Get(ctx)`
5. `components.Transpile(html)` — regex replacement of custom component tags

### ParseDir Flow
1. Walk directory for `.mdx` (case-insensitive suffix)
2. `parseFile` each
3. Sort by `SortKey()` = `[X.Y.Z]` → `[3]int` (major.minor.patch)

### Variable Substitution

`substituteVars(raw string) string` runs **before** goldmark parsing, so vars work in:
- Frontmatter YAML values (e.g. `header_title: "{{product}} v{{version}}"`)
- Body text
- Code blocks
- Component attributes

Syntax: `{{key}}` — simple string replacement using `strings.ReplaceAll(raw, "{{"+k+"}}", v)`.

Set via `WithVars(map)` parser option or `prettyconverter.WithVars(map)` root option.

### Components

Registered in `ComponentRegistry`. Handlers transpiled via regex after goldmark HTML.

Built-in:
- `<DeepDive title="...">` → `<aside class="component-deep-dive">` (blue)
- `<Warning title="...">` → `<div class="component-warning">` (orange)
- `<Axiom>` → `<blockquote class="component-axiom">` (green italic)

`RegisterComponent(name, handler)` on Parser allows non-destructive addition.

---

## Validator (`mdx/validator.go`)

### DefaultValidator

```go
type DefaultValidator struct {
    RequireFrontmatter []string
    RequireIDFormat    string
    NoDuplicateIDs     bool
    MaxHeadingDepth    int
}

func NewDefaultValidator() *DefaultValidator
func (v *DefaultValidator) Validate(doc *Document) []ValidationError
func (v *DefaultValidator) ValidateAll(docs []*Document) []ValidationError
```

### Rules

| Rule | Severity | Condition |
|------|----------|-----------|
| Required frontmatter field | Error | Field missing or empty string |
| ID format | Error | Doesn't match `^\[\d+\.\d+\.\d+\]$` |
| Duplicate IDs | Error | Same `[X.Y.Z]` in >1 file |
| Heading depth | Warning (Error with `--strict`) | HTML has `<hN>` with N > `MaxHeadingDepth` |

`ValidateAll()` runs `Validate()` per doc, then checks cross-doc rules (duplicates).

### ValidationError

```go
type ValidationError struct {
    File    string
    Field   string
    Message string
}
```

---

## Render (`render/render.go`)

Headless Chrome via `chromedp`. HTML encoded as `data:text/html;charset=utf-8,...` URI.

PDF features enabled: `PrintBackground`, `DisplayHeaderFooter`, `GenerateDocumentOutline` (bookmarks), `GenerateTaggedPDF` (accessibility).

### Paper Size Mapping (CLI)

| Name | Width×Height (inches) |
|------|----------------------|
| A4 | 8.27 × 11.69 |
| Letter | 8.5 × 11 |
| Legal | 8.5 × 14 |

### CSS Unit Parser (`main.go:405`)

Converts CSS unit strings to inches:
- `mm` → / 25.4
- `cm` → / 2.54
- `in` → value (identity)
- `pt` → / 72.0
- `px` → / 96.0

---

## Theme System (`theme/theme.go`)

```go
type Theme struct {
    Name     string
    CSS      string   // Override CSS (empty = use embedded)
    Template string   // Override template (empty = use embedded)
}
```

Built-in: `Default` (empty → embedded assets), `Minimal` (stripped-down CSS, default template).

---

## Testing

```bash
go test ./...                    # 26 tests, all pass
go test ./mdx/... -v             # Component (6), parser/file/dir (3), sort key (1), validator (6), vars (2), custom comp (1)
go test ./config/... -v          # Config defaults (1), Load (1), partial defaults (1), FindConfig (1)
```

### Test Breakdown

| Package | Test Count | What |
|---------|-----------|------|
| `mdx/` | 9 | Component transpile (6), ParseFile (1), ParseDir (1), SortKey (1) |
| `mdx/` (validator) | 6 | DefaultValidator (4), ValidateAll/duplicates (1), heading depth/strict (1) |
| `mdx/` (vars + partial parse) | 3 | Vars substitution (1), vars with components (1), partial parse errors (1) |
| `config/` | 4 | Default(), Load(), LoadDefaultsOnMissingKeys(), FindConfig() |

---

## Key File Reference

| File | Lines | Role |
|------|-------|------|
| `cmd/pretty-converter/main.go` | ~110 | Thin cobra entry — global vars, root/builtin/version/cmd/watcher commands |
| `cmd/pretty-converter/config.go` | ~120 | `loadConfig()`, `buildOpts()`, `parseCSSUnit()`, `validatorFromConfig()`, `parserFromConfig()` |
| `cmd/pretty-converter/build.go` | ~332 | `runBuild()` animated pipeline, `runBuildJSON()`, `runPreFlight()`, `countMDXFiles()`, `formatBytes()` |
| `cmd/pretty-converter/check.go` | ~100 | `runCheck()` styled validation with spinner + summary panel |
| `cmd/pretty-converter/init.go` | ~160 | `runInit()` huh interactive wizard, `runInitBare()` JSON mode, `scaffoldWithConfig()` |
| `cmd/pretty-converter/watch.go` | ~140 | `runWatch()` fsnotify recursive watcher with debounce + stats |
| `cmd/pretty-converter/output/styles.go` | ~120 | Lipgloss styles: colors, symbols, Panel/Success/Error/Warn/Info helpers, `NoColor()` |
| `cmd/pretty-converter/output/spinner.go` | ~60 | Animated goroutine spinner with `ack` channel sync |
| `cmd/pretty-converter/output/banner.go` | ~30 | ASCII art "GO → PDF" banner |
| `cmd/pretty-converter/output/panels.go` | ~110 | `BuildStats`, `PrintBuildSummary`, `PrintValidationSummary`, `PreFlightResult`, `PrintPreFlight` |
| `cmd/pretty-converter/output/progress.go` | ~165 | `PipelineProgress` (Start/Done/Fail/Skip/PrintSummary), `WatchStats`, `PrintWatchBanner/Rebuild/Summary` |
| `cmd/pretty-converter/initassets/*` | 4 files | Scaffold templates for `init` command |
| `pdf.go` | ~220 | Root API, 18 options, Build/Validate pipeline, step-by-step methods, WithComponent fix |
| `version/version.go` | 3 | Canonical version string `var Version = "0.1.0"` |
| `scripts/bump/bump.go` | ~65 | SemVer bump script (patch/minor/major), reads/writes version/version.go |
| `doc.go` | ~18 | Package doc for pkg.go.dev |
| `LICENSE` | 21 | MIT license |
| `README.md` | ~240 | Full project documentation with badges, install, usage, API |
| `CHANGELOG.md` | ~30 | Keep a Changelog format, v0.1.0 initial release |
| `CONTRIBUTING.md` | ~45 | Contribution guide |
| `SECURITY.md` | ~25 | Security policy |
| `config/config.go` | 82 | Config struct, YAML Load(), FindConfig(), Default() |
| `config/config_test.go` | 145 | 4 config tests |
| `mdx/parser.go` | ~210 | Goldmark parser, variable substitution, RegisterComponent, `ParseFileError`, `ParseErrors` |
| `mdx/validator.go` | ~105 | DefaultValidator with 4 lint rules |
| `mdx/validator_test.go` | ~252 | Validator + variable tests |
| `mdx/mdx.go` | 133 | Document type, frontmatter accessors, ID utilities |
| `mdx/component.go` | 104 | Component transpilation (DeepDive, Warning, Axiom) |
| `mdx/mdx_test.go` | 314 | Original parser/component tests |
| `compose/compose.go` | 107 | HTML composition, template execution, keywords |
| `compose/toc.go` | 45 | TOC builder from `[X.Y.Z]` hierarchy |
| `compose/assets/template.html` | 29 | Default HTML template (go:embed) |
| `compose/assets/print.css` | 277 | Default print CSS (go:embed) |
| `render/render.go` | 126 | Chrome headless PDF render |
| `theme/theme.go` | 87 | Theme struct, Default & Minimal |
| `examples/full-demo/go-pretty-converter.yml` | 35 | Full demo config with all sections including CSS+template refs |
| `examples/full-demo/custom.css` | 208 | Demo custom CSS (dark cover, blue accents, code dark mode) |
| `examples/full-demo/custom-template.html` | 30 | Demo custom template (minimal, dark cover) |
| `examples/full-demo/*.mdx` | 4 files | Demo content exercising vars, components, frontmatter |

---

## Dependencies

```
github.com/chromedp/cdproto              Chrome DevTools Protocol types
github.com/chromedp/chromedp              Chrome headless automation
github.com/charmbracelet/lipgloss         CLI styling (colors, panels, formatting)
github.com/charmbracelet/huh              Interactive form UI (init wizard)
github.com/charmbracelet/bubbles          Bubble tea components (spinner frames)
github.com/charmbracelet/bubbletea        TUI framework (huh dependency)
github.com/fsnotify/fsnotify              File system watcher (watch mode)
github.com/mattn/go-isatty                Terminal detection (lipgloss dep)
github.com/muesli/termenv                 Terminal profiles, ASCII fallback (NoColor)
github.com/spf13/cobra                    CLI framework
github.com/spf13/pflag                    Flag parsing (cobra dep)
github.com/yuin/goldmark                 Markdown parser
github.com/yuin/goldmark-meta            YAML frontmatter for goldmark
gopkg.in/yaml.v3                          YAML config file parsing
```

---

## Known Fixes & Gotchas

1. **CSS/template paths in config** resolved relative to config file directory, NOT CWD
2. **CLI flag paths** (`--css`, `--template`) resolved via `filepath.Abs()`
3. **`FindConfig()`** returns absolute path (avoids `go run` on Windows issues)
4. **`os.ReadFile` failures** in `WithConfigCSSAndTemplate` silently swallowed unless verbose
5. **Options order**: `WithVerbose` before `WithConfigCSSAndTemplate` for file-read error logging
6. **No auto-detect of CSS/template files** — must be explicitly configured in YAML or CLI
7. **`WithComponent` appends** (no longer replaces parser)
8. **Config without CSS/Template fields** → uses embedded assets or theme
9. **Source default** is `"book"` (not CWD) — aligns with `init` scaffold
10. **Spinner deadlock fixed**: `Done()`/`Fail()` used `<-s.result` but goroutine never sent. Replaced with `ack` channel — goroutine closes `ack` on exit (via defer), `Done`/`Fail` wait on `<-s.ack` before printing
11. **`.gitignore` `pretty-converter` pattern** was too broad (matched `cmd/pretty-converter/` dir). Fixed to `/pretty-converter` (root-anchored only)
12. **Pre-flight order**: Chrome check first (hard failure), then source/output/CSS/template (warnings can pass)
13. **Partial parsing**: `ParseDir`/`ParseAll` never abort on per-file errors. Returns partial docs + `ParseErrors`. Caller decides whether to continue. Frontmatter-not-found logged as debug-level, not error
14. **Watch mode debounce**: 300ms debounce via `time.AfterFunc`. Subsequent events within window reset the timer. Prevents double-builds on editor save (fsnotify fires multiple events per save)
15. **Version canonical source**: `version/version.go` (not a `var` in `main.go`). All ldflags target `github.com/sazardev/go-pretty-converter/version.Version`. Update via `make bump-patch|minor|major`.
16. **Makefile is cross-platform**: Since v0.1.0, `make build` automatically appends `.exe` on Windows. Uses `$(GOOS)` detection.
17. **Bump script reads version file**: `scripts/bump/bump.go` parses `version/version.go` via regex, bumps, writes back. Called by Makefile bump targets.
18. **Docs workflow deploys to gh-pages**: `.github/workflows/docs.yml` builds both demo PDFs (library + CLI) and deploys to GitHub Pages with an index.html landing page.

---

## Project Files (added for publication)

| File | Purpose |
|------|---------|
| `LICENSE` | MIT license |
| `README.md` | Full docs — badges, install, CLI usage, library API, config reference |
| `CHANGELOG.md` | Keep a Changelog format, v0.1.0 initial |
| `doc.go` | Package doc (package `prettyconverter`) for pkg.go.dev |
| `CONTRIBUTING.md` | Dev setup, conventions, commit style |
| `SECURITY.md` | Vulnerability reporting policy |

## CI/CD Pipeline

### GitHub Actions

| Workflow | File | Trigger |
|----------|------|---------|
| **CI** | `.github/workflows/ci.yml` | Push/PR to `main`/`master` |
| **Release** | `.github/workflows/release.yml` | Tag `v*` pushed |
| **Docs** | `.github/workflows/docs.yml` | Push to `main`/`master` (paths: examples, compose/assets, README) |
| **Dependabot** | `.github/dependabot.yml` | Weekly — gomod + GitHub Actions |

### CI Jobs (parallel)

- **tidy** — `go mod tidy` + `git diff --exit-code` (ensures go.mod/go.sum are clean)
- **lint** — `golangci-lint` with 15 linters (5m timeout)
- **test** — `go test -race -coverprofile` on ubuntu, macOS, windows (fail-fast: false)
- **vet** — `go vet ./...`
- **vulncheck** — `govulncheck` via `go run`
- **build** — `go build -ldflags="-s -w"` on ubuntu, macOS, windows (fail-fast: false)

### Release Pipeline

```
tag v* → test (matrix, with -race) → goreleaser (linux/darwin/windows amd64+arm64) → GitHub Release + changelog + checksums
```

### Docs Pipeline (GitHub Pages)

```
push to main → build library demo PDF → build full-demo CLI PDF → create index.html → deploy to gh-pages
```

Hosted at: `https://sazardev.github.io/go-pretty-converter/`
Uploads: `library-demo.pdf`, `full-demo.pdf`, `index.html`

### Linters (`./golangci.yml`)

`bodyclose`, `errcheck`, `goconst`, `gofmt`, `goimports`, `gosimple`, `govet` (all except fieldalignment), `ineffassign`, `misspell`, `nilerr`, `prealloc`, `staticcheck`, `tenv`, `unconvert`, `unused`

### Release Automation (`./goreleaser.yml`)

- Builds: linux (amd64/arm64), darwin (amd64/arm64), windows (amd64)
- ldflags: `-X github.com/sazardev/go-pretty-converter/version.Version={{ .Version }}`
- Archives: `tar.gz` (unix), `zip` (windows)
- Changelog: auto, excludes docs/test/chore/merge
- Checksums: `checksums.txt`

### Makefile (local dev)

| Target | Description |
|--------|-------------|
| `make help` | Show all targets |
| `make lint` | `golangci-lint run --timeout=5m` |
| `make fmt` | `go fmt ./...` |
| `make tidy` | `go mod tidy` |
| `make vulncheck` | `govulncheck` |
| `make test` | `go test -race ./...` |
| `make test-verbose` | `go test -race -v ./...` |
| `make test-cover` | test + HTML coverage report + func summary |
| `make build` | build `bin/pretty-converter` with version (cross-platform: auto `.exe` on Windows) |
| `make build-release` | stripped build |
| `make install` | `go install` with ldflags → `$GOPATH/bin` |
| `make version-info` | Print current version |
| `make bump-patch` | `scripts/bump/bump.go patch` → commit + tag |
| `make bump-minor` | `scripts/bump/bump.go minor` → commit + tag |
| `make bump-major` | `scripts/bump/bump.go major` → commit + tag |
| `make release-dry-run` | goreleaser --snapshot --skip=publish |
| `make clean` | rm bin/, coverage.out, coverage.html, out.pdf |

### Version Injection

Canonical source: `version/version.go` → `var Version = "0.1.0"`
Overridden at build via: `-ldflags "-X github.com/sazardev/go-pretty-converter/version.Version=<version>"`

Works in:
- GitHub Actions CI (`go build -ldflags="-s -w"`)
- goreleaser (`ldflags` in `.goreleaser.yml`)
- Makefile (`make build` reads git describe)
- `go install` via `make install`

The bump script (`scripts/bump/bump.go`) reads `version/version.go`, bumps patch/minor/major, writes back, and the Makefile target commits + tags.

### `.gitignore` Updates

Added `bin/`, `coverage.out`, `coverage.html`, `examples/full-demo/out.pdf`, `examples/full-demo/demo.pdf`,
`_site/` (GitHub Pages), `sbom.spdx.json`, IDE dirs (`.idea/`, `.vscode/`), OS files (`.DS_Store`, `Thumbs.db`)

### Removed Files (cleanup)

`-p/` (empty dir), `pretty-converter.exe` (binary in root), `.ignore` (empty), `skills-lock.json` (not standard)

package prettypdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/sazardev/go-pretty-pdf/compose"
	"github.com/sazardev/go-pretty-pdf/config"
	"github.com/sazardev/go-pretty-pdf/epub"
	"github.com/sazardev/go-pretty-pdf/kindle"
	"github.com/sazardev/go-pretty-pdf/mdx"
	"github.com/sazardev/go-pretty-pdf/render"
	"github.com/sazardev/go-pretty-pdf/theme"
)

type OutputFormat string

const (
	FormatPDF    OutputFormat = "pdf"
	FormatEPUB   OutputFormat = "epub"
	FormatKindle OutputFormat = "kindle"
)

func ParseFormats(s string) ([]OutputFormat, error) {
	parts := strings.Split(s, ",")
	seen := map[OutputFormat]bool{}
	var formats []OutputFormat
	for _, p := range parts {
		f := OutputFormat(strings.TrimSpace(p))
		if f == "" {
			continue
		}
		switch f {
		case FormatPDF, FormatEPUB, FormatKindle:
		default:
			return nil, fmt.Errorf("unsupported format %q (supported: pdf, epub, kindle)", f)
		}
		if !seen[f] {
			seen[f] = true
			formats = append(formats, f)
		}
	}
	if len(formats) == 0 {
		formats = []OutputFormat{FormatPDF}
	}
	return formats, nil
}

type PDF struct {
	sourceDir       string
	outputFile      string
	formats         []OutputFormat
	parser          *mdx.Parser
	composeOpts     compose.Options
	renderOpts      render.Options
	epubOpts        epub.Options
	calibrePath     string
	validator       mdx.Validator
	verbose         bool
	pendingWarnings []string
	warnings        []string
	headerTitleSet  bool
	lastAudit       *render.AuditReport
	configErr       error
	// sharedBrowser, when set via WithSharedBrowser, is a Chrome allocator
	// context reused across Build calls instead of booting a new Chrome
	// process per render. Each Build creates its own browser tab
	// (chromedp.NewContext) on the shared allocator, so concurrent Build
	// calls are safe. Callers rendering many PDFs (e.g. docsgen) use this
	// to amortize the ~400ms Chrome startup cost once across N documents.
	// The caller owns the allocator and must cancel it after.
	sharedBrowser context.Context
}

type ComposeOptions = compose.Options

func DefaultComposeOptions() ComposeOptions {
	return compose.DefaultOptions()
}

func ComposeHTML(docs []*mdx.Document, opts ComposeOptions) (string, error) {
	return compose.ComposeHTML(docs, opts)
}

type Option func(*PDF)

func WithSourceDir(dir string) Option {
	return func(p *PDF) {
		p.sourceDir = dir
	}
}

func WithOutputFile(path string) Option {
	return func(p *PDF) {
		p.outputFile = path
	}
}

func WithFormats(formats ...OutputFormat) Option {
	return func(p *PDF) {
		if len(formats) > 0 {
			p.formats = formats
		}
	}
}

func WithEpubLanguage(lang string) Option {
	return func(p *PDF) {
		p.epubOpts.Language = lang
	}
}

// WithCalibreExecPath pins the FormatKindle pipeline to a specific
// Calibre ebook-convert executable instead of resolving one from PATH.
// Leave unset (or pass "") to resolve ebook-convert from PATH at build
// time — see the kindle package for the full resolution order.
func WithCalibreExecPath(path string) Option {
	return func(p *PDF) {
		p.calibrePath = path
	}
}

func WithTitle(title string) Option {
	return func(p *PDF) {
		p.composeOpts.Title = title
	}
}

func WithSubtitle(subtitle string) Option {
	return func(p *PDF) {
		p.composeOpts.Subtitle = subtitle
	}
}

func WithAuthor(author string) Option {
	return func(p *PDF) {
		p.composeOpts.Author = author
	}
}

func WithCSS(css string) Option {
	return func(p *PDF) {
		p.composeOpts.CSS = css
		p.epubOpts.CSS = css
	}
}

func WithTemplate(html string) Option {
	return func(p *PDF) {
		p.composeOpts.Template = html
	}
}

// WithTheme applies a raw builtin/synthetic Theme's CSS as-is, with no
// customization (colors/fonts/sections/density) and no section toggles
// applied. It shares composeOpts.CSS with WithCSS and WithThemeName —
// whichever of these options is applied last wins, since New() applies
// options in the order they're passed. Most callers should prefer
// WithThemeName, which resolves section toggles (cover/TOC/page
// numbers/header) into composeOpts/renderOpts too.
func WithTheme(t theme.Theme) Option {
	return func(p *PDF) {
		if t.CSS != "" {
			p.composeOpts.CSS = t.CSS
			p.epubOpts.CSS = t.CSS
		}
	}
}

// WithThemeName resolves a theme by name — a builtin ("default",
// "corporate", ...), a custom theme discovered in ./themes/ or the global
// themes directory, or a direct path to a .theme.yml/.css file — applies
// opts customization (colors, fonts, density, network fonts), and wires
// the resulting section toggles (cover, TOC, page numbers, header) into
// composeOpts/renderOpts. The theme is resolved through both the PDF and
// the EPUB pipelines, so a later EPUB build (Build with FormatEPUB or
// RenderEpub) carries the same theme, not the default stylesheet.
func WithThemeName(name string, opts theme.Options) Option {
	return func(p *PDF) {
		p.applyTheme(name, opts)
	}
}

// applyTheme resolves name through both the PDF (ResolveByName) and EPUB
// (ResolveByNameForEPUB) theme pipelines. The PDF result drives the
// section toggles (cover/TOC/page numbers/header) plus composeOpts.CSS;
// the EPUB result goes to epubOpts.CSS so both output formats get the same
// theme. Resolve failures are recorded as non-fatal warnings.
func (p *PDF) applyTheme(name string, opts theme.Options) {
	cwd, _ := os.Getwd()
	css, sections, err := theme.ResolveByName(name, opts, cwd)
	if err != nil {
		p.pendingWarnings = append(p.pendingWarnings, fmt.Sprintf("theme %q: %v", name, err))
		return
	}
	p.composeOpts.CSS = css
	p.composeOpts.ShowCover = sections.Cover
	p.composeOpts.ShowTOC = sections.TOC
	p.renderOpts.PageNumbers = sections.PageNumbers
	p.renderOpts.ShowHeader = sections.Header

	epubCSS, err := theme.ResolveByNameForEPUB(name, opts, cwd)
	if err != nil {
		p.pendingWarnings = append(p.pendingWarnings, fmt.Sprintf("theme %q (EPUB): %v", name, err))
		return
	}
	p.epubOpts.CSS = epubCSS
}

func WithComponent(name string, handler mdx.ComponentHandler) Option {
	return func(p *PDF) {
		p.parser.RegisterComponent(name, handler)
	}
}

func WithValidator(v mdx.Validator) Option {
	return func(p *PDF) {
		p.validator = v
	}
}

func WithTimeout(d time.Duration) Option {
	return func(p *PDF) {
		p.renderOpts.Timeout = d
	}
}

// WithGenerateDocumentOutline toggles whether PDF bookmarks/outline are
// built from the document's headings. Building the outline is post-print
// work over the whole PDF; disable for a meaningful speedup on very large
// documents at the cost of losing in-PDF navigation bookmarks.
func WithGenerateDocumentOutline(enabled bool) Option {
	return func(p *PDF) {
		p.renderOpts.GenerateDocumentOutline = enabled
	}
}

// WithGenerateTaggedPDF toggles PDF accessibility tagging (PDF/UA). Tagging
// is the most expensive post-print step on large documents; disable for a
// real speedup when accessibility metadata isn't needed.
func WithGenerateTaggedPDF(enabled bool) Option {
	return func(p *PDF) {
		p.renderOpts.GenerateTaggedPDF = enabled
	}
}

// WithSharedBrowser makes subsequent Build calls render PDFs on the given
// Chrome allocator instead of booting a fresh Chrome process each time.
// The allocator must come from render.NewBrowser, stay alive across all
// Build calls, and be canceled by the caller once done. Each Build opens
// its own tab on the allocator, so concurrent Build calls share one Chrome
// process safely. This amortizes the per-launch Chrome startup cost across
// many renders — most useful when a process renders many documents
// (docsgen's per-theme PDFs, a batch job, a test harness).
func WithSharedBrowser(browserCtx context.Context) Option {
	return func(p *PDF) {
		p.sharedBrowser = browserCtx
	}
}

// WithHeaderTitle sets the PDF page header text. If never called, New()
// defaults it to the document title (WithTitle/composeOpts.Title).
func WithHeaderTitle(title string) Option {
	return func(p *PDF) {
		p.renderOpts.HeaderTitle = title
		p.headerTitleSet = true
	}
}

func WithVerbose(v bool) Option {
	return func(p *PDF) {
		p.verbose = v
	}
}

func WithVars(vars map[string]string) Option {
	return func(p *PDF) {
		p.parser.SetVars(vars)
	}
}

func WithRenderMargins(top, bottom, left, right float64) Option {
	return func(p *PDF) {
		p.renderOpts.MarginTop = top
		p.renderOpts.MarginBottom = bottom
		p.renderOpts.MarginLeft = left
		p.renderOpts.MarginRight = right
	}
}

func WithPaperSize(width, height float64) Option {
	return func(p *PDF) {
		p.renderOpts.PaperWidth = width
		p.renderOpts.PaperHeight = height
	}
}

// WithCoverImage replaces the theme's text cover with a full-bleed page
// built from imagePath (.png/.jpg/.jpeg). That cover page is sized to the
// image's own pixel dimensions exactly — a square image gets a square
// cover page — while every other page keeps its configured paper size. The
// theme's own text cover (title/subtitle/metadata) is suppressed
// regardless of theme/section settings and regardless of the order
// options are applied in — see New(), which enforces this once after every
// Option has run.
func WithCoverImage(imagePath string) Option {
	return func(p *PDF) {
		p.renderOpts.CoverImagePath = imagePath
	}
}

// WithNetworkAccess controls whether headless Chrome may make outbound
// network requests while rendering. It defaults to false: the composed
// HTML is a self-contained data URI, so network access is blocked to
// prevent SSRF/exfiltration from untrusted MDX content (e.g. a malicious
// <img> or <script> tag). Enable it only if your documents intentionally
// reference remote images, fonts, or other resources by URL.
func WithNetworkAccess(enabled bool) Option {
	return func(p *PDF) {
		p.renderOpts.NetworkAccess = enabled
	}
}

// WithChromeExecPath pins rendering to a specific Chrome/Chromium binary
// instead of chromedp's default system discovery. Leave unset (or pass "")
// to keep the default behavior. See the chromemgr package for resolving
// this automatically, including downloading a browser when none is
// installed.
func WithChromeExecPath(path string) Option {
	return func(p *PDF) {
		p.renderOpts.ChromeExecPath = path
	}
}

func WithConfig(cfg *config.Config) Option {
	return func(p *PDF) {
		if cfg.Source != "" {
			p.sourceDir = cfg.Source
		}
		if cfg.Output != "" {
			p.outputFile = cfg.Output
		}
		if cfg.Title != "" {
			p.composeOpts.Title = cfg.Title
		}
		if cfg.Subtitle != "" {
			p.composeOpts.Subtitle = cfg.Subtitle
		}
		if cfg.Author != "" {
			p.composeOpts.Author = cfg.Author
		}
	}
}

// themeOptionsFromConfig converts cfg.ThemeOptions (as loaded from
// go-pretty-pdf.yml or set by CLI flags) into theme.Options.
func themeOptionsFromConfig(cfg *config.Config) theme.Options {
	to := cfg.ThemeOptions
	return theme.Options{
		Colors: theme.Colors{
			Primary:    to.Colors.Primary,
			Accent:     to.Colors.Accent,
			Text:       to.Colors.Text,
			Muted:      to.Colors.Muted,
			Background: to.Colors.Background,
		},
		Fonts: theme.Fonts{
			Heading:       to.Fonts.Heading,
			Body:          to.Fonts.Body,
			Code:          to.Fonts.Code,
			GoogleImports: to.Fonts.GoogleFonts,
		},
		Sections: theme.Sections{
			Cover:       to.Sections.Cover,
			TOC:         to.Sections.TOC,
			PageNumbers: to.Sections.PageNumbers,
			Header:      to.Sections.Header,
		},
		Density:           theme.Density(to.Density),
		AllowNetworkFonts: to.AllowNetworkFonts,
	}
}

// WithConfigCSSAndTemplate resolves cfg.Theme (with cfg.ThemeOptions
// customization) and then loads CSS/template content from cfg.CSS/
// cfg.Template, which — being explicit file overrides — take priority over
// the theme and replace its CSS/template outright. Read/resolve failures
// are recorded as warnings and flushed to stderr by New() once all options
// have been applied, so ordering relative to WithVerbose does not matter.
func WithConfigCSSAndTemplate(cfg *config.Config) Option {
	return func(p *PDF) {
		if cfg.Theme != "" {
			p.applyTheme(cfg.Theme, themeOptionsFromConfig(cfg))
		}
		if cfg.CSS != "" {
			data, err := os.ReadFile(cfg.CSS)
			if err == nil {
				p.composeOpts.CSS = string(data)
				p.epubOpts.CSS = string(data)
			} else {
				p.pendingWarnings = append(p.pendingWarnings, fmt.Sprintf("reading CSS file %s: %v", cfg.CSS, err))
			}
		}
		if cfg.Template != "" {
			data, err := os.ReadFile(cfg.Template)
			if err == nil {
				p.composeOpts.Template = string(data)
			} else {
				p.pendingWarnings = append(p.pendingWarnings, fmt.Sprintf("reading template file %s: %v", cfg.Template, err))
			}
		}
	}
}

// WithFullConfig applies every field of cfg: source/output/title/subtitle
// /author (via WithConfig), CSS/template/theme (via WithConfigCSSAndTemplate),
// variable substitution (cfg.Vars), and render settings (cfg.Render:
// timeout, paper size, margins, header title). Unlike WithConfig and
// WithConfigCSSAndTemplate, which only cover a subset of Config, this is
// the single option needed to fully apply a loaded go-pretty-pdf.yml.
func WithFullConfig(cfg *config.Config) Option {
	return func(p *PDF) {
		WithConfig(cfg)(p)
		WithConfigCSSAndTemplate(cfg)(p)

		if len(cfg.Vars) > 0 {
			p.parser.SetVars(cfg.Vars)
		}

		if cfg.Render.Timeout != "" {
			if d, err := time.ParseDuration(cfg.Render.Timeout); err == nil {
				p.renderOpts.Timeout = d
			} else {
				p.configErr = fmt.Errorf("invalid render timeout %q: %v (expected a duration like 30s or 1m)", cfg.Render.Timeout, err)
			}
		}

		if cfg.Render.Paper != "" {
			if w, h, ok := config.ParsePaperSize(cfg.Render.Paper); ok {
				p.renderOpts.PaperWidth = w
				p.renderOpts.PaperHeight = h
			} else {
				p.configErr = fmt.Errorf("invalid paper size %q: use a named size (letter, legal, a4) or custom dimensions (e.g. 6x9in, 152.4mm x 228.6mm)", cfg.Render.Paper)
			}
		}

		// Checked per-field on the *string* being non-empty, not on the
		// parsed value being non-zero: margin_top: "0mm" is a legitimate,
		// meaningful choice (e.g. for a full-bleed dark theme) and must not
		// be indistinguishable from "not set in the config file at all".
		// p.renderOpts already holds render.DefaultOptions() from New(), so
		// an unset field is simply left at its default. A value that can't
		// be parsed as a CSS length is a hard config error rather than a
		// silent fallback to 0 — a 0 margin would silently collapse the
		// layout instead of telling the author their config is wrong.
		if cfg.Render.MarginTop != "" {
			if v, ok := config.ParseCSSUnitStrict(cfg.Render.MarginTop); ok {
				p.renderOpts.MarginTop = v
			} else {
				p.configErr = fmt.Errorf("invalid margin_top %q: expected a CSS length like 20mm, 1in, or 0mm", cfg.Render.MarginTop)
			}
		}
		if cfg.Render.MarginBot != "" {
			if v, ok := config.ParseCSSUnitStrict(cfg.Render.MarginBot); ok {
				p.renderOpts.MarginBottom = v
			} else {
				p.configErr = fmt.Errorf("invalid margin_bottom %q: expected a CSS length like 20mm, 1in, or 0mm", cfg.Render.MarginBot)
			}
		}
		if cfg.Render.MarginLeft != "" {
			if v, ok := config.ParseCSSUnitStrict(cfg.Render.MarginLeft); ok {
				p.renderOpts.MarginLeft = v
			} else {
				p.configErr = fmt.Errorf("invalid margin_left %q: expected a CSS length like 20mm, 1in, or 0mm", cfg.Render.MarginLeft)
			}
		}
		if cfg.Render.MarginRight != "" {
			if v, ok := config.ParseCSSUnitStrict(cfg.Render.MarginRight); ok {
				p.renderOpts.MarginRight = v
			} else {
				p.configErr = fmt.Errorf("invalid margin_right %q: expected a CSS length like 20mm, 1in, or 0mm", cfg.Render.MarginRight)
			}
		}

		if cfg.Render.CoverImage != "" {
			p.renderOpts.CoverImagePath = cfg.Render.CoverImage
		}

		if cfg.Render.HeaderTitle != "" {
			headerTitle := cfg.Render.HeaderTitle
			for k, v := range cfg.Vars {
				headerTitle = strings.ReplaceAll(headerTitle, "{{"+k+"}}", v)
			}
			p.renderOpts.HeaderTitle = headerTitle
			p.headerTitleSet = true
		}
	}
}

func New(opts ...Option) (*PDF, error) {
	p := &PDF{
		sourceDir:   "book",
		outputFile:  "out.pdf",
		formats:     []OutputFormat{FormatPDF},
		parser:      mdx.NewParser(),
		composeOpts: compose.DefaultOptions(),
		renderOpts:  render.DefaultOptions(),
		epubOpts:    epub.DefaultOptions(),
	}

	for _, o := range opts {
		o(p)
	}

	if !p.headerTitleSet {
		p.renderOpts.HeaderTitle = p.composeOpts.Title
	}

	// A custom cover image always wins over the theme's text cover,
	// regardless of the order WithCoverImage/WithThemeName/WithFullConfig
	// were applied in — otherwise a theme option resolved after
	// WithCoverImage would silently re-enable the text cover on top of it.
	if p.renderOpts.CoverImagePath != "" {
		p.composeOpts.ShowCover = false
	}

	if p.epubOpts.Title == "" || p.epubOpts.Title == "Document" {
		p.epubOpts.Title = p.composeOpts.Title
	}
	if p.epubOpts.Subtitle == "" {
		p.epubOpts.Subtitle = p.composeOpts.Subtitle
	}
	if p.epubOpts.Author == "" || p.epubOpts.Author == "go-pretty-pdf" {
		p.epubOpts.Author = p.composeOpts.Author
	}
	if p.renderOpts.CoverImagePath != "" {
		p.epubOpts.CoverImage = p.renderOpts.CoverImagePath
	}

	// Always surfaced, not just when verbose: these mark a theme/CSS/
	// template option that failed to apply and silently fell back to the
	// previous value, which is worth knowing about regardless of
	// verbosity level. New() still returns a nil error here — a bad theme
	// name is intentionally non-fatal (see TestWithThemeNameUnknownWarns)
	// — but Warnings() lets a caller check programmatically instead of
	// only scraping stderr.
	for _, w := range p.pendingWarnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}
	p.warnings = p.pendingWarnings
	p.pendingWarnings = nil

	if p.configErr != nil {
		return nil, p.configErr
	}

	return p, nil
}

// Warnings returns the non-fatal configuration warnings recorded by New —
// e.g. an unresolvable theme name or an unreadable --css/--template file —
// each of which caused that option to fall back rather than apply. They are
// also printed to stderr by New itself; this accessor exists for callers
// that want to detect the condition programmatically instead.
func (p *PDF) Warnings() []string {
	return p.warnings
}

func (p *PDF) Build(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if p.verbose {
		fmt.Printf("Parsing MDX files in %s...\n", p.sourceDir)
	}

	docs, err := p.parser.ParseDir(p.sourceDir)
	if err != nil && len(docs) == 0 {
		return fmt.Errorf("parsing: %w", err)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: some files failed to parse: %v\n", err)
	}

	if p.verbose {
		fmt.Printf("Found %d document(s)\n", len(docs))
	}

	if err = ctx.Err(); err != nil {
		return err
	}

	if p.validator != nil {
		p.logVerbose("Running validation...")
		var allErrs []mdx.ValidationError
		warnings := 0
		for _, doc := range docs {
			for _, e := range p.validator.Validate(doc) {
				// Content findings (excess heading depth) are advisory,
				// mirroring the CLI's `check` semantics (where they're only
				// fatal with --strict): they're printed but never fail a
				// build. Structural frontmatter errors still do.
				if e.Field == mdx.ContentField {
					fmt.Printf("  - (warning) %v\n", e)
					warnings++
					continue
				}
				allErrs = append(allErrs, e)
			}
		}
		if len(allErrs) > 0 {
			for _, e := range allErrs {
				fmt.Printf("  - %v\n", e)
			}
			return fmt.Errorf("validation failed: %d error(s)", len(allErrs))
		}
		if warnings > 0 {
			p.logVerbose(fmt.Sprintf("Validation passed with %d content warning(s)", warnings))
		}
		p.logVerbose("Validation passed")
	}

	if err = ctx.Err(); err != nil {
		return err
	}

	for _, f := range p.formats {
		if err = ctx.Err(); err != nil {
			return err
		}
		switch f {
		case FormatPDF:
			p.logVerbose("Composing HTML...")
			html, err := compose.ComposeHTML(docs, p.composeOpts)
			if err != nil {
				return fmt.Errorf("composing HTML: %w", err)
			}
			p.logVerbose(fmt.Sprintf("Rendering PDF to %s...", p.outputFile))
			var report *render.AuditReport
			if p.sharedBrowser != nil {
				// Open a fresh tab on the shared allocator; each render
				// gets its own browser context so concurrent Build calls
				// on the same allocator don't step on each other.
				browserCtx, tabCancel := chromedp.NewContext(p.sharedBrowser)
				report, err = render.RenderToPDFWithAuditBrowser(browserCtx, html, p.outputFile, p.renderOpts)
				tabCancel()
			} else {
				report, err = render.RenderToPDFWithAuditContext(ctx, html, p.outputFile, p.renderOpts)
			}
			if err != nil {
				return fmt.Errorf("rendering PDF: %w", err)
			}
			report = appendUnusedComponentIssues(report, p)
			p.lastAudit = report
		case FormatEPUB:
			epubPath := p.epubOutputPath()
			p.logVerbose(fmt.Sprintf("Writing EPUB to %s...", epubPath))
			if err := epub.Write(docs, p.epubOpts, epubPath); err != nil {
				return fmt.Errorf("writing EPUB: %w", err)
			}
		case FormatKindle:
			kindlePath := p.kindleOutputPath()
			p.logVerbose(fmt.Sprintf("Converting to Kindle format at %s...", kindlePath))
			if err := p.RenderKindle(ctx, docs, kindlePath); err != nil {
				return fmt.Errorf("writing Kindle file: %w", err)
			}
		}
	}

	return nil
}

func (p *PDF) epubOutputPath() string {
	ext := strings.ToLower(filepath.Ext(p.outputFile))
	if ext == ".pdf" {
		return strings.TrimSuffix(p.outputFile, ext) + ".epub"
	}
	if ext == ".epub" {
		return p.outputFile
	}
	return p.outputFile + ".epub"
}

// kindleOutputPath mirrors epubOutputPath: a .pdf/.epub p.outputFile maps
// to the .mobi variant, an already-Kindle extension (.mobi/.azw3) is kept
// as-is, and anything else gets ".mobi" appended.
func (p *PDF) kindleOutputPath() string {
	ext := strings.ToLower(filepath.Ext(p.outputFile))
	switch ext {
	case ".pdf", ".epub":
		return strings.TrimSuffix(p.outputFile, ext) + ".mobi"
	case ".mobi", ".azw3":
		return p.outputFile
	default:
		return p.outputFile + ".mobi"
	}
}

func (p *PDF) Formats() []OutputFormat {
	return p.formats
}

func (p *PDF) NeedsChrome() bool {
	for _, f := range p.formats {
		if f == FormatPDF {
			return true
		}
	}
	return false
}

// NeedsCalibre reports whether FormatKindle is among the configured
// formats, meaning Build will need Calibre's ebook-convert on PATH (or
// pinned via WithCalibreExecPath).
func (p *PDF) NeedsCalibre() bool {
	for _, f := range p.formats {
		if f == FormatKindle {
			return true
		}
	}
	return false
}

func (p *PDF) RenderEpub(docs []*mdx.Document, outputPath string) error {
	return epub.Write(docs, p.epubOpts, outputPath)
}

// RenderKindle converts docs into a Kindle-ready file at outputPath,
// reusing the same EPUB options (theme, metadata, cover) as RenderEpub
// but piping the result through Calibre's ebook-convert. See the kindle
// package for the conversion pipeline and Calibre resolution order.
func (p *PDF) RenderKindle(ctx context.Context, docs []*mdx.Document, outputPath string) error {
	return kindle.Write(ctx, docs, kindle.Options{EPUB: p.epubOpts, CalibrePath: p.calibrePath}, outputPath)
}

func (p *PDF) Validate(ctx context.Context) ([]mdx.ValidationError, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	docs, err := p.parser.ParseDir(p.sourceDir)
	if err != nil {
		return nil, err
	}

	if p.validator == nil {
		return nil, fmt.Errorf("no validator configured")
	}

	if err = ctx.Err(); err != nil {
		return nil, err
	}

	var allErrs []mdx.ValidationError
	for _, doc := range docs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		errs := p.validator.Validate(doc)
		allErrs = append(allErrs, errs...)
	}

	return allErrs, nil
}

func (p *PDF) ParseDir() ([]*mdx.Document, error) {
	return p.parser.ParseDir(p.sourceDir)
}

func (p *PDF) ValidateDoc(doc *mdx.Document) []mdx.ValidationError {
	if p.validator == nil {
		return nil
	}
	return p.validator.Validate(doc)
}

func (p *PDF) ValidateAll(docs []*mdx.Document) []mdx.ValidationError {
	if p.validator == nil {
		return nil
	}
	return p.validator.ValidateAll(docs)
}

func (p *PDF) ComposeHTML(docs []*mdx.Document) (string, error) {
	return compose.ComposeHTML(docs, p.composeOpts)
}

func (p *PDF) Render(html string) error {
	return p.RenderWithContext(context.Background(), html)
}

// RenderWithContext is Render with the browser rooted in ctx instead of
// context.Background(): canceling ctx (client disconnect, SIGINT wired to
// context cancellation) tears down the in-flight Chrome render rather than
// running to completion or to opts.Timeout regardless. opts.Timeout still
// applies as an upper bound layered on top of ctx.
func (p *PDF) RenderWithContext(ctx context.Context, html string) error {
	report, err := render.RenderToPDFWithAuditContext(ctx, html, p.outputFile, p.renderOpts)
	if err != nil {
		return err
	}
	p.lastAudit = report
	return nil
}

// LastAudit returns the visual/structural audit report from the most
// recent Build or Render call, or nil if neither has run yet. See
// render.AuditReport and render/audit.go for what it checks.
func (p *PDF) LastAudit() *render.AuditReport {
	return p.lastAudit
}

// checkUnusedComponent is the audit check name for a custom component
// registered via WithComponent() that no document used. Shared by the
// audit report and its tests.
const checkUnusedComponent = "unused-component"

// appendUnusedComponentIssues augments a render audit report with one
// "unused-component" warning per custom component registered via
// WithComponent() that no parsed document actually used. It's a pure
// authoring check (nothing about the rendered pixels), so it can't live in
// the DOM audit — but it belongs in the same report so callers see every
// quality signal in one place. Builtin components (DeepDive, Warning,
// Axiom) are excluded: they're registered by default and a book that never
// uses them is normal, not a mistake.
func appendUnusedComponentIssues(report *render.AuditReport, p *PDF) *render.AuditReport {
	if report == nil {
		report = &render.AuditReport{}
	}

	builtin := map[string]bool{
		"DeepDive": true,
		"Warning":  true,
		"Axiom":    true,
	}

	usage := p.parser.ComponentUsage()
	for _, name := range p.parser.ComponentNames() {
		if builtin[name] {
			continue
		}
		if usage[name] == 0 {
			report.Issues = append(report.Issues, render.Issue{
				Check:    checkUnusedComponent,
				Severity: render.SeverityWarning,
				Message:  fmt.Sprintf("component <%s> was registered via WithComponent() but no document used it — check the tag spelling in your MDX", name),
			})
		}
	}
	return report
}

func (p *PDF) logVerbose(msg string) {
	if p.verbose {
		fmt.Println(msg)
	}
}

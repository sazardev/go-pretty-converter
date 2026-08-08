package render

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/sazardev/go-pretty-pdf/theme"
)

type Options struct {
	Timeout       time.Duration
	HeaderTitle   string
	MarginTop     float64
	MarginBottom  float64
	MarginLeft    float64
	MarginRight   float64
	PaperWidth    float64
	PaperHeight   float64
	NetworkAccess bool
	PageNumbers   bool
	ShowHeader    bool
	// CoverImagePath, when non-empty, prepends a full-bleed cover page
	// built from this image (.png/.jpg/.jpeg) before the rest of the
	// document. That cover page's dimensions match the image's own pixel
	// dimensions exactly (see coverImageDimensionsIn) rather than
	// PaperWidth/PaperHeight — a square image gets a square cover page —
	// while every other page keeps the configured paper size untouched.
	CoverImagePath string
	// ChromeExecPath, when non-empty, pins chromedp to this specific
	// Chrome/Chromium/chrome-headless-shell binary instead of letting it
	// search the system's default install locations. Callers resolving a
	// browser via chromemgr.EnsureChrome pass its result straight through
	// here; leaving it empty preserves the previous default-discovery
	// behavior exactly.
	ChromeExecPath string
	// GenerateDocumentOutline, when true, asks Chrome to build the PDF
	// bookmarks/outline tree from the document's headings. This is
	// post-print work over the entire PDF and can be a significant chunk
	// of render time on very large documents. Defaults to true (bookmarks
	// are a headline feature); set false to trade them for speed.
	GenerateDocumentOutline bool
	// GenerateTaggedPDF, when true, asks Chrome to tag the PDF with
	// accessibility structure (PDF/UA). This is post-print work over the
	// entire PDF — the most expensive of the post-print steps on huge
	// documents. Defaults to true (accessibility is valuable); set false
	// for a meaningful speedup on very large documents.
	GenerateTaggedPDF bool
}

func DefaultOptions() Options {
	return Options{
		Timeout:     60 * time.Second,
		HeaderTitle: "Document",
		// Left/right are 0 so the page background can bleed edge to edge —
		// the visual reading margin instead comes from CSS padding on
		// <body> (see theme/assets/base.css), which the theme's own
		// background color paints straight through. Top/bottom stay as a
		// real Chrome print margin (not CSS padding): that's the only
		// space the running header/page-number footer can render into,
		// and — unlike body padding, which only applies to the first/last
		// page per the CSS Fragmentation spec — a native margin repeats
		// identically on every page. RenderToPDF colors that strip to
		// match the theme, so it doesn't show up as a white band either.
		MarginTop:    0.6,
		MarginBottom: 0.6,
		MarginLeft:   0,
		MarginRight:  0,
		PaperWidth:   8.27,
		PaperHeight:  11.69,
		// NetworkAccess defaults to false: the rendered HTML is a
		// self-contained data URI, so outbound network requests are
		// blocked unless explicitly enabled.
		NetworkAccess: false,
		PageNumbers:   true,
		ShowHeader:    true,
		// Both post-print steps default on (bookmarks and accessibility
		// are features), but are switchable off for speed on large docs.
		GenerateDocumentOutline: true,
		GenerateTaggedPDF:       true,
	}
}

// navigationURLFor writes htmlContent to a temporary file and returns a
// file:// URL pointing at it, along with a cleanup func that removes the
// file once the caller is done navigating to it.
//
// This used to be a "data:text/html;charset=utf-8;base64,..." URI instead
// of a temp file. That worked for small-to-medium books, but Chrome (at
// least via chromedp/CDP's Page.navigate) silently aborts navigation to a
// data: URI once the encoded payload crosses roughly 2MB ("chromedp
// render: page load error net::ERR_ABORTED"), with no indication that
// size is the cause. A book with a few hundred thousand words of prose
// and code easily produces a composed HTML document past that threshold.
// Writing to a real file sidesteps the limit entirely and has no
// practical downside: it's still a local, sandboxed, single-use file,
// cleaned up immediately after the page is captured.
func navigationURLFor(htmlContent string) (navURL string, cleanup func(), err error) {
	tmpFile, err := os.CreateTemp("", "go-pretty-pdf-*.html")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp html file: %w", err)
	}

	cleanup = func() { _ = os.Remove(tmpFile.Name()) }

	if _, writeErr := tmpFile.WriteString(htmlContent); writeErr != nil {
		_ = tmpFile.Close()
		cleanup()
		return "", nil, fmt.Errorf("writing temp html file: %w", writeErr)
	}
	if closeErr := tmpFile.Close(); closeErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("closing temp html file: %w", closeErr)
	}

	absPath, err := filepath.Abs(tmpFile.Name())
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("resolving temp html file path: %w", err)
	}

	// url.URL's own String() method takes care of platform differences in
	// file URL formatting (e.g. the extra leading slash before a Windows
	// drive letter, "file:///C:/...") rather than hand-building the string.
	fileURL := &url.URL{Scheme: "file", Path: filepath.ToSlash(absPath)}
	return fileURL.String(), cleanup, nil
}

// RenderToPDF composes htmlContent into a PDF at outputPath. It never
// returns audit findings — use RenderToPDFWithAudit for that — but always
// keeps this signature exactly as-is for API stability.
func RenderToPDF(htmlContent string, outputPath string, opts Options) error {
	_, err := RenderToPDFWithAudit(htmlContent, outputPath, opts)
	return err
}

// RenderToPDFWithAudit does exactly what RenderToPDF does, and additionally
// runs a best-effort visual/structural audit (see audit.go) of the
// composed document, returning its findings alongside any render error.
// The audit never turns a successful render into a failure: an audit
// finding is a warning about the *output*, not a reason to reject it.
//
// Keeps this signature exactly as-is for API stability — it roots the
// browser in context.Background() internally, so a caller that needs to
// cancel an in-flight render (e.g. on client disconnect or SIGINT) should
// use RenderToPDFWithAuditContext instead.
func RenderToPDFWithAudit(htmlContent string, outputPath string, opts Options) (*AuditReport, error) {
	return RenderToPDFWithAuditContext(context.Background(), htmlContent, outputPath, opts)
}

// RenderToPDFWithAuditContext is RenderToPDFWithAudit with the browser
// rooted in ctx instead of context.Background(): canceling ctx (client
// disconnect, SIGINT wired to context cancellation, etc.) now actually
// tears down the in-flight Chrome allocator/render instead of running to
// completion or to opts.Timeout regardless. opts.Timeout still applies as
// an upper bound layered on top of ctx via context.WithTimeout.
func RenderToPDFWithAuditContext(ctx context.Context, htmlContent string, outputPath string, opts Options) (*AuditReport, error) {
	allocCtx, allocCancel := NewBrowser(ctx, opts)
	defer allocCancel()

	browserCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	return renderToPDFWithAuditBrowser(browserCtx, htmlContent, outputPath, opts)
}

// RenderToPDFWithAuditBrowser renders htmlContent to outputPath reusing an
// existing Chrome browser context instead of booting a new Chrome process.
// The browser must have been created with NewBrowser (or chromedp.NewContext
// over a shared allocator). This is how callers rendering many documents
// (e.g. docsgen's per-theme PDFs) avoid paying the ~400ms Chrome startup
// cost once per document: boot once, render N times.
//
// The caller owns the browser lifetime: it must stay alive for the whole
// call and be closed (allocCancel) after all renders finish. opts.Timeout
// still bounds this single render. ctx cancels the in-flight render.
func RenderToPDFWithAuditBrowser(browserCtx context.Context, htmlContent string, outputPath string, opts Options) (*AuditReport, error) {
	renderCtx, cancel := context.WithTimeout(browserCtx, opts.Timeout)
	defer cancel()
	return renderToPDFWithAuditBrowser(renderCtx, htmlContent, outputPath, opts)
}

// renderToPDFWithAuditBrowser is the shared implementation behind
// RenderToPDFWithAuditContext and RenderToPDFWithAuditBrowser: it renders
// htmlContent into outputPath on the given browser context. The caller is
// responsible for wrapping browserCtx with any timeout.
func renderToPDFWithAuditBrowser(browserCtx context.Context, htmlContent string, outputPath string, opts Options) (*AuditReport, error) {
	// Resolved and validated up front, before spinning up a browser at
	// all, so a bad --cover-image path fails fast with a clear error
	// instead of after a full Chrome launch.
	var coverWidthIn, coverHeightIn float64
	if opts.CoverImagePath != "" {
		var err error
		coverWidthIn, coverHeightIn, err = coverImageDimensionsIn(opts.CoverImagePath)
		if err != nil {
			return nil, err
		}
	}

	navURL, cleanup, err := navigationURLFor(htmlContent)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// Chrome renders the header/footer template inside the top/bottom
	// margin strip it was given — a strip that's otherwise always blank
	// white, regardless of the page's own background. Painting that strip
	// with the theme's own --pdf-bg (and using --pdf-muted for the text
	// instead of a hardcoded gray) is what makes a dark theme's PDF look
	// like one continuous dark page instead of dark content floating in a
	// white border.
	//
	// The wrapper div must be given an explicit height equal to the
	// margin, in the same absolute unit (inches) passed to
	// WithMarginTop/Bottom — height:100% is unreliable here since Chrome
	// doesn't reliably give this template's ancestor chain a resolvable
	// height, and without *some* explicit height the div only grows as
	// tall as its one line of text, leaving the rest of the margin strip
	// (most of it) unpainted white.
	bg, mutedText := pageChrome(htmlContent)
	wrap := func(marginIn float64, alignItems string) string {
		return fmt.Sprintf(
			`width:100%%;height:%.3fin;box-sizing:border-box;margin:0;background:%s;`+
				`-webkit-print-color-adjust:exact;print-color-adjust:exact;`+
				`display:flex;align-items:%s;`,
			marginIn, bg, alignItems,
		)
	}
	headerWrap := wrap(opts.MarginTop, "flex-end")      // text sits just above the content
	footerWrap := wrap(opts.MarginBottom, "flex-start") // text sits just below the content
	textStyle := fmt.Sprintf(
		// 15mm left/right matches <body>'s own reading margin (see
		// theme/assets/base.css) so header/footer text lines up with the
		// content edge instead of the physical page edge.
		`width:100%%;box-sizing:border-box;font-size:8pt;font-family:system-ui,sans-serif;color:%s;padding:2pt 15mm;`,
		mutedText,
	)

	// Chrome renders this template HTML into its own little document with
	// the usual user-agent default (a few pixels of <body> margin) —
	// enough to leave a sliver of the true page-edge unpainted above the
	// header even though our own div's height matches the margin exactly.
	// Reset it explicitly rather than guess at an offset.
	resetStyle := `<style>html,body{margin:0;padding:0;}</style>`

	headerTpl := fmt.Sprintf(`%s<div style="%s"><div style="%s">&nbsp;</div></div>`, resetStyle, headerWrap, textStyle)
	if opts.ShowHeader && opts.HeaderTitle != "" {
		headerTpl = fmt.Sprintf(
			`%s<div style="%s"><div style="%s">%s</div></div>`,
			resetStyle, headerWrap, textStyle, opts.HeaderTitle,
		)
	}

	footerTpl := fmt.Sprintf(`%s<div style="%s"><div style="%s">&nbsp;</div></div>`, resetStyle, footerWrap, textStyle)
	if opts.PageNumbers {
		footerTpl = fmt.Sprintf(
			`%s<div style="%s"><div style="%s"><span class="title"></span><span style="float:right;">Page <span class="pageNumber"></span> of <span class="totalPages"></span></span></div></div>`,
			resetStyle, footerWrap, textStyle,
		)
	}

	var pdfBuf []byte
	var domIssues []Issue

	tasks := chromedp.Tasks{}
	if !opts.NetworkAccess {
		tasks = append(tasks,
			network.Enable(),
			network.SetBlockedURLs().WithURLPatterns([]*network.BlockPattern{
				{URLPattern: "*://*/*", Block: true},
			}),
		)
	}
	// Chrome reserves a small fixed inset (~0.2in, empirically measured —
	// independent of the margin size and not overridable via the template's
	// own CSS) at the very top/bottom of the page whenever header/footer
	// templates are displayed at all, even fully empty ones. There's no way
	// to make that sliver match the theme's background short of not paying
	// for the header/footer mechanism in the first place, so skip it
	// entirely when the caller wants neither a header nor page numbers —
	// pair with MarginTop/Bottom: 0 for a genuinely gap-free page.
	needsHeaderFooter := opts.ShowHeader || opts.PageNumbers

	var coverPDFBuf []byte
	if opts.CoverImagePath != "" {
		coverTasks, coverCleanup, err := coverPDFTasks(opts.CoverImagePath, coverWidthIn, coverHeightIn, &coverPDFBuf)
		if err != nil {
			return nil, err
		}
		defer coverCleanup()
		tasks = append(tasks, coverTasks...)
	}

	tasks = append(tasks,
		chromedp.Navigate(navURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Runs against the fully-loaded document, before PrintToPDF
			// hands it to Chrome's print engine — see audit.go for what it
			// checks and why it has to happen here rather than after.
			domIssues = runDOMAudit(ctx, needsHeaderFooter)
			return nil
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithDisplayHeaderFooter(needsHeaderFooter).
				WithHeaderTemplate(headerTpl).
				WithFooterTemplate(footerTpl).
				WithGenerateDocumentOutline(opts.GenerateDocumentOutline).
				WithGenerateTaggedPDF(opts.GenerateTaggedPDF).
				WithMarginTop(opts.MarginTop).
				WithMarginBottom(opts.MarginBottom).
				WithMarginLeft(opts.MarginLeft).
				WithMarginRight(opts.MarginRight).
				WithPaperWidth(opts.PaperWidth).
				WithPaperHeight(opts.PaperHeight).
				Do(ctx)
			return err
		}),
	)

	if err := chromedp.Run(browserCtx, tasks...); err != nil {
		return nil, fmt.Errorf("chromedp render: %w", err)
	}

	if opts.CoverImagePath != "" {
		if err := mergeCoverAndBody(coverPDFBuf, pdfBuf, outputPath); err != nil {
			return nil, err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return nil, fmt.Errorf("creating output directory: %w", err)
		}
		if err := os.WriteFile(outputPath, pdfBuf, 0644); err != nil {
			return nil, fmt.Errorf("writing PDF: %w", err)
		}
	}

	report := &AuditReport{Issues: append(domIssues, auditPDFBytes(pdfBuf)...)}
	return report, nil
}

// styleBlockRe extracts the content of the first <style>...</style> element
// in a composed document, so pageChrome only ever looks at actual CSS.
var styleBlockRe = regexp.MustCompile(`(?is)<style[^>]*>(.*?)</style>`)

// pageChrome pulls the page background and muted-text colors out of the
// composed document's own <style> block, so the native header/footer strip
// can be painted to match whatever theme is active instead of hardcoded
// white/gray. Falls back to sensible light-theme defaults if a theme
// doesn't declare one (shouldn't happen for any builtin theme — see
// theme.TestBuiltinThemesProduceSiteVars — but custom/raw CSS files are
// user-authored and may omit either).
//
// htmlContent is the *entire* composed document — cover, TOC, and every
// chapter's rendered HTML — not just its CSS, so vars are extracted from
// the <style> block specifically rather than passed to
// theme.ExtractCSSVars wholesale. That regex matches a `--pdf-x: value;`
// declaration wherever it textually appears, with no notion of HTML
// structure; running it over the full document would let a chapter that
// merely *mentions* that syntax (e.g. a code sample documenting theme
// variables) silently override the real theme's header/footer colors.
func pageChrome(htmlContent string) (bg, mutedText string) {
	css := htmlContent
	if m := styleBlockRe.FindStringSubmatch(htmlContent); m != nil {
		css = m[1]
	}

	vars := theme.ExtractCSSVars(css)
	bg = vars["bg"]
	if bg == "" {
		bg = "#ffffff"
	}
	mutedText = vars["muted"]
	if mutedText == "" {
		mutedText = "#666666"
	}
	return bg, mutedText
}

func CheckChromeAvailable() error {
	allocCtx, allocCancel := chromedp.NewExecAllocator(
		context.Background(),
		chromedp.NoSandbox,
		chromedp.Headless,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WSURLReadTimeout(15*time.Second),
	)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	return chromedp.Run(ctx)
}

// NewBrowser boots a headless Chrome allocator for rendering, tuned for
// minimal startup latency. The returned cancel tears down the browser; it
// must be deferred by the caller. The launch flags suppress the first-run
// experience, background networking, component updates, sync, and
// extensions — all of which Chrome would otherwise spin up on boot even
// though a PDF render uses none of them. These shave ~60-120ms off each
// cold start, and callers rendering many documents should create one
// browser and pass it to RenderToPDFWithAuditBrowser (or
// WithSharedBrowser on a PDF) to avoid the ~400ms startup cost entirely.
func NewBrowser(ctx context.Context, opts Options) (context.Context, context.CancelFunc) {
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Headless,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-component-update", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-features", "Translate,OptimizationHints,MediaRouter,CalculateNativeWinOcclusion"),
		// Chrome can take longer than chromedp's 20s default to print its
		// DevTools websocket URL on a cold/loaded CI runner (e.g. right
		// after a fresh install); give it more room to avoid a spurious
		// "websocket url timeout reached" before the browser even starts.
		chromedp.WSURLReadTimeout(45*time.Second),
	)
	if opts.ChromeExecPath != "" {
		allocOpts = append(allocOpts, chromedp.ExecPath(opts.ChromeExecPath))
	}
	return chromedp.NewExecAllocator(ctx, allocOpts...)
}

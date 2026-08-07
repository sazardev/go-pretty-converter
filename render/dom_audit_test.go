package render

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// runAuditOnHTML boots headless Chrome, navigates to htmlContent, and runs
// the real domAuditJS against it — the same code path RenderToPDFWithAudit
// uses. Skipped when no Chrome is available (mirrors requireChrome).
func runAuditOnHTML(t *testing.T, htmlContent string, needsHeaderFooter bool) []Issue {
	t.Helper()
	requireChrome(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.DisableGPU,
			chromedp.NoSandbox,
			chromedp.Headless,
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.WSURLReadTimeout(30*time.Second),
		)...,
	)
	defer allocCancel()

	browserCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	navURL := "data:text/html;charset=utf-8;base64," + base64.StdEncoding.EncodeToString([]byte(htmlContent))

	var issues []Issue
	if err := chromedp.Run(browserCtx,
		chromedp.Navigate(navURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			issues = runDOMAudit(ctx, needsHeaderFooter)
			return nil
		}),
	); err != nil {
		t.Fatalf("chromedp run failed: %v", err)
	}
	return issues
}

func checksOf(issues []Issue) map[string]bool {
	out := map[string]bool{}
	for _, i := range issues {
		out[i.Check] = true
	}
	return out
}

func TestDOMAuditOverflowX(t *testing.T) {
	issues := runAuditOnHTML(t, `<html><body><pre style="white-space:pre;width:200px;display:block">`+
		`this is a very long code line that overflows its narrow box by a lot of pixels for sure yes indeed</pre></body></html>`, false)
	if !checksOf(issues)["overflow-x"] {
		t.Errorf("expected overflow-x issue, got %v", issues)
	}
}

func TestDOMAuditBrokenAnchorAndDuplicateID(t *testing.T) {
	html := `<html><body>
    <section id="dup"><h1>First</h1></section>
    <section id="dup"><h1>Second</h1></section>
    <a href="#missing-fragment">dead link</a>
    <a href="#dup">dup link</a>
    <p>some real content that is definitely long enough to pass the empty check ok</p>
  </body></html>`
	issues := runAuditOnHTML(t, html, false)
	checks := checksOf(issues)
	if !checks["duplicate-id"] {
		t.Errorf("expected duplicate-id issue, got %v", issues)
	}
	if !checks["broken-anchor"] {
		t.Errorf("expected broken-anchor issue, got %v", issues)
	}
}

func TestDOMAuditLowContrastWCAGThreshold(t *testing.T) {
	// #999999 on white is ~2.8:1 — fails the 4.5:1 normal-text threshold,
	// passes the old 2.2 one. Guarantees the WCAG threshold is wired in.
	html := `<html><body><p style="color:#999999;background:#ffffff;font-size:14px">muted gray text that is hard to read</p></body></html>`
	issues := runAuditOnHTML(t, html, false)
	if !checksOf(issues)["low-contrast"] {
		t.Errorf("expected low-contrast (2.8:1 < 4.5:1), got %v", issues)
	}
}

func TestDOMAuditTOCMismatch(t *testing.T) {
	html := `<html><body>
    <div class="toc"><a href="#present">present</a><a href="#absent">absent</a></div>
    <section id="present"><h1>Present</h1></section>
    <section id="missing-entry"><h2>No TOC entry</h2></section>
    <p>enough content to pass the empty document check indeed</p>
  </body></html>`
	issues := runAuditOnHTML(t, html, false)
	if !checksOf(issues)["toc-mismatch"] {
		t.Errorf("expected toc-mismatch issue, got %v", issues)
	}
}

func TestDOMAuditLineBreakRisk(t *testing.T) {
	html := `<html><body><p style="orphans:1;widows:1">a short paragraph that sets orphans and widows to one which is a real typographic risk for print</p></body></html>`
	issues := runAuditOnHTML(t, html, false)
	if !checksOf(issues)["line-break-risk"] {
		t.Errorf("expected line-break-risk issue, got %v", issues)
	}
}

func TestDOMAuditCleanDocumentNoIssues(t *testing.T) {
	html := `<html><head><style>
      body { color:#111; background:#fff; font-size:14px; }
      h1 { orphans:3; widows:3; page-break-before: always; margin-top: 0.5in; }
      table, pre { page-break-inside: avoid; }
    </style></head><body>
      <h1>Clean</h1>
      <p>Plenty of readable body text that should not trigger any audit rule here whatsoever.</p>
      <p style="orphans:3;widows:3">Another paragraph with safe orphan and widow settings throughout.</p>
    </body></html>`
	issues := runAuditOnHTML(t, html, true)
	checks := checksOf(issues)
	// These checks should NOT fire on a clean document.
	for _, c := range []string{"overflow-x", "overflow-y", "broken-image", "empty-content", "low-contrast", "heading-clip-risk", "broken-anchor", "duplicate-id", "toc-mismatch", "line-break-risk"} {
		if checks[c] {
			t.Errorf("unexpected %s issue on clean document: %v", c, issues)
		}
	}
}

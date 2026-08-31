package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	prettyconverter "github.com/sazardev/go-pretty-converter"
	"github.com/sazardev/go-pretty-converter/mdx"
	"github.com/sazardev/go-pretty-converter/render"
	"github.com/sazardev/go-pretty-converter/theme"
)

// docsPDFDefault is the canonical, stable download URL (used in the
// sitemap and as the href before any client-side JS runs). It mirrors the
// site's own default theme (classic).
const docsPDFDefault = "go-pretty-converter-docs.pdf"

// docsPDFFilename returns the per-theme download artifact name. The site's
// theme switcher (site.js) rewrites the download button's href to match
// whichever of these the visitor currently has selected, so "download the
// docs" always matches what they're looking at.
func docsPDFFilename(themeID string) string {
	return "go-pretty-converter-docs-" + themeID + ".pdf"
}

var readmeBadgesRe = regexp.MustCompile(`(?m)^\[!\[.*\n?`)

// prepareDocsSource writes the three docs sources (README + CLI + changelog)
// into a fresh temp directory as frontmattered MDX, the exact same source
// a real `pretty-converter build` would consume, and returns that directory plus
// the parsed documents. PDF and EPUB builders share the same parsed
// content; PDFs re-derive their own copy through prettyconverter.New (the
// dogfooding path), while EPUBs use the documents directly.
//
// Badge images point at shields.io/pkg.go.dev and would just render as
// broken-image glyphs: WithNetworkAccess defaults to false, matching the
// CLI's own safe default for untrusted MDX sources, so they're stripped
// here.
func prepareDocsSource(readme, cli, changelog []byte) (srcDir string, docs []*mdx.Document, err error) {
	srcDir, err = os.MkdirTemp("", "go-pretty-converter-docs-src-*")
	if err != nil {
		return "", nil, err
	}

	cleanReadme := readmeBadgesRe.ReplaceAll(readme, nil)

	sources := []struct {
		file, id, title string
		body            []byte
	}{
		{"01-docs.mdx", "[1.0.0]", siteName, cleanReadme},
		{"02-cli.mdx", "[2.0.0]", "CLI Reference", cli},
		{"03-changelog.mdx", "[3.0.0]", "Changelog", changelog},
	}
	for _, d := range sources {
		content := fmt.Sprintf("---\nid: %q\ntitle: %q\n---\n\n%s", d.id, d.title, d.body)
		if werr := os.WriteFile(filepath.Join(srcDir, d.file), []byte(content), 0644); werr != nil {
			return "", nil, werr
		}
	}

	docs, err = parseDocsSource(srcDir)
	if err != nil {
		return "", nil, err
	}
	return srcDir, docs, nil
}

// generateDocsPDF renders README.md + docs/cli.md + CHANGELOG.md into one
// downloadable PDF per builtin theme, using the same code path a real
// user's `pretty-converter build` would take — dogfooding the actual public
// library API (mdx parser, theme package, chromedp render pipeline), not a
// raw HTML screenshot. Best-effort: like generateRasterAssets, it must not
// break `go run ./scripts/docsgen` for contributors without Chrome
// installed locally.
//
// The per-theme builds are independent, so they run concurrently — up to
// jobs Chrome processes at once (the dominant cost is Chrome launch +
// render, so this scales almost linearly until the machine saturates).
// Each result is reported individually so the summary shows exactly which
// themes succeeded and which were skipped.
func generateDocsPDF(outDir string, readme, cli, changelog []byte, log *buildLogger, jobs int, noOutline, noTagged bool) []renderResult {
	srcDir, _, err := prepareDocsSource(readme, cli, changelog)
	if err != nil {
		log.Warnf("skipping docs PDF, could not prepare source: %v", err)
		return nil
	}
	defer func() { _ = os.RemoveAll(srcDir) }()

	// Each theme PDF boots its own headless Chrome and renders in
	// parallel. Measured: sharing one Chrome across concurrent renders
	// *serializes* them (single-process contention), so per-PDF browsers
	// win at jobs>1; the flags in render.NewBrowser shave startup off each.
	themes := theme.List()
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]renderResult, 0, len(themes))

	for _, t := range themes {
		wg.Add(1)
		go func(t theme.Theme) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			outPath := filepath.Join(outDir, docsPDFFilename(t.Name))
			t0 := time.Now()
			err := buildOneDocsPDF(srcDir, outPath, t.Name, log, noOutline, noTagged)
			res := renderResult{name: t.Name, group: groupPDF, elapsed: time.Since(t0), ok: err == nil}
			if err != nil {
				res.err = err
			} else if info, serr := os.Stat(outPath); serr == nil {
				res.bytes = info.Size()
				res.note = formatBytes(int(info.Size()))
			}
			if res.ok && t.Name == theme.NameClassic {
				if cerr := copyFile(outPath, filepath.Join(outDir, docsPDFDefault)); cerr != nil {
					res.note = "built; could not stage default copy: " + cerr.Error()
				}
			}

			mu.Lock()
			results = append(results, res)
			mu.Unlock()

			if res.ok {
				log.Vf("  ✓ %-16s %-10s %s", t.Name, res.note, res.elapsed.Round(time.Millisecond))
			} else {
				log.Vf("  ✗ %-16s %v", t.Name, res.err)
			}
		}(t)
	}

	wg.Wait()
	return results
}

func buildOneDocsPDF(srcDir, outPath, themeID string, log *buildLogger, noOutline, noTagged bool) error {
	pdf, err := prettyconverter.New(
		prettyconverter.WithSourceDir(srcDir),
		prettyconverter.WithOutputFile(outPath),
		prettyconverter.WithTitle(siteName),
		prettyconverter.WithSubtitle("Write Markdown. Ship a book."),
		prettyconverter.WithAuthor("sazardev"),
		prettyconverter.WithHeaderTitle("go-pretty-converter — Documentation"),
		prettyconverter.WithThemeName(themeID, theme.Options{}),
		prettyconverter.WithTimeout(120*time.Second),
		prettyconverter.WithGenerateDocumentOutline(!noOutline),
		prettyconverter.WithGenerateTaggedPDF(!noTagged),
	)
	if err != nil {
		return fmt.Errorf("configuring PDF build: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := pdf.Build(ctx); err != nil {
		return fmt.Errorf("building PDF: %w", err)
	}

	// Report the audit findings for this theme in verbose mode: this is
	// dogfooding, so it should surface exactly what a user would see.
	if audit := pdf.LastAudit(); audit != nil && audit.HasIssues() {
		for _, issue := range audit.Issues {
			log.Vf("  ! [%s] %s: %s", themeID, issue.Check, issue.Message)
		}
	}
	if audit := pdf.LastAudit(); audit != nil && reportHasErrors(audit) {
		return fmt.Errorf("PDF audit reported errors (corrupt output)")
	}

	return nil
}

func reportHasErrors(report *render.AuditReport) bool {
	for _, i := range report.Issues {
		if i.HasError() {
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	return err
}

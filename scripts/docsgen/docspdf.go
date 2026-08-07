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

	prettypdf "github.com/sazardev/go-pretty-pdf"
	"github.com/sazardev/go-pretty-pdf/render"
	"github.com/sazardev/go-pretty-pdf/theme"
)

// docsPDFDefault is the canonical, stable download URL (used in the
// sitemap and as the href before any client-side JS runs). It mirrors the
// site's own default theme (classic).
const docsPDFDefault = "go-pretty-pdf-docs.pdf"

// docsPDFFilename returns the per-theme download artifact name. The site's
// theme switcher (site.js) rewrites the download button's href to match
// whichever of these the visitor currently has selected, so "download the
// docs" always matches what they're looking at.
func docsPDFFilename(themeID string) string {
	return "go-pretty-pdf-docs-" + themeID + ".pdf"
}

var readmeBadgesRe = regexp.MustCompile(`(?m)^\[!\[.*\n?`)

// generateDocsPDF renders README.md + docs/cli.md + CHANGELOG.md into one
// downloadable PDF per builtin theme, using the same code path a real
// user's `pretty-pdf build` would take — dogfooding the actual public
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
func generateDocsPDF(outDir string, readme, cli, changelog []byte, log *buildLogger, jobs int) []renderResult {
	srcDir, err := os.MkdirTemp("", "go-pretty-pdf-docs-src-*")
	if err != nil {
		log.Warnf("skipping docs PDF, could not create temp dir: %v", err)
		return nil
	}
	defer func() { _ = os.RemoveAll(srcDir) }()

	// Badge images point at shields.io/pkg.go.dev and would just render as
	// broken-image glyphs: WithNetworkAccess defaults to false, matching
	// the CLI's own safe default for untrusted MDX sources.
	cleanReadme := readmeBadgesRe.ReplaceAll(readme, nil)

	docs := []struct {
		file, id, title string
		body            []byte
	}{
		{"01-docs.mdx", "[1.0.0]", "go-pretty-pdf", cleanReadme},
		{"02-cli.mdx", "[2.0.0]", "CLI Reference", cli},
		{"03-changelog.mdx", "[3.0.0]", "Changelog", changelog},
	}
	for _, d := range docs {
		content := fmt.Sprintf("---\nid: %q\ntitle: %q\n---\n\n%s", d.id, d.title, d.body)
		if err := os.WriteFile(filepath.Join(srcDir, d.file), []byte(content), 0644); err != nil {
			log.Warnf("skipping docs PDF, could not write %s: %v", d.file, err)
			return nil
		}
	}

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
			err := buildOneDocsPDF(srcDir, outPath, t.Name, log)
			res := renderResult{name: t.Name, elapsed: time.Since(t0), ok: err == nil}
			if err != nil {
				res.err = err
			} else if info, serr := os.Stat(outPath); serr == nil {
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

func buildOneDocsPDF(srcDir, outPath, themeID string, log *buildLogger) error {
	pdf, err := prettypdf.New(
		prettypdf.WithSourceDir(srcDir),
		prettypdf.WithOutputFile(outPath),
		prettypdf.WithTitle("go-pretty-pdf"),
		prettypdf.WithSubtitle("Write Markdown. Ship a book."),
		prettypdf.WithAuthor("sazardev"),
		prettypdf.WithHeaderTitle("go-pretty-pdf — Documentation"),
		prettypdf.WithThemeName(themeID, theme.Options{}),
		prettypdf.WithTimeout(120*time.Second),
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

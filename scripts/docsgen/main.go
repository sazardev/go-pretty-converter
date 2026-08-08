package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkHtml "github.com/yuin/goldmark/renderer/html"

	"github.com/sazardev/go-pretty-pdf/theme"
)

//go:embed assets/site.css
var siteCSS string

//go:embed assets/site.js
var siteJS string

const (
	siteBaseURL     = "https://sazardev.github.io/go-pretty-pdf/"
	siteHost        = "sazardev.github.io"
	siteRepoURL     = "https://github.com/sazardev/go-pretty-pdf"
	siteTitle       = "go-pretty-pdf — Turn Markdown into Beautiful, Print-Ready PDFs (Go)"
	siteDescription = "go-pretty-pdf turns a folder of Markdown/MDX into a beautifully typeset, print-ready PDF via headless Chrome — as a Go library or CLI. No LaTeX, no design tools."
	siteKeywords    = "markdown to pdf, mdx to pdf, go pdf generator, golang pdf library, cli pdf generator, print-ready pdf, headless chrome pdf, markdown book generator, mdx renderer"
)

// siteName is the project display name, reused across metadata, EPUBs, and
// generated markdown to avoid repeated string literals.
const siteName = "go-pretty-pdf"

// heroSectionID is the landing hero section's stable id, referenced by the
// docs page assembly and the search index.
const heroSectionID = "hero"

type Section struct {
	ID      string
	Title   string
	Eyebrow string
	Content string
}

func main() {
	verbose := flag.Bool("verbose", false, "print every step (markdown sections, per-asset renders, per-theme PDFs)")
	jobs := flag.Int("jobs", 4, "maximum concurrent Chrome-based renders (screenshots + PDFs)")
	bench := flag.Bool("bench", false, "additionally render the theme PDFs sequentially (jobs=1) to measure real parallel speedup")
	noOutline := flag.Bool("no-outline", false, "skip PDF bookmarks/outline in theme PDFs (faster on large docs)")
	noTagged := flag.Bool("no-tagged-pdf", false, "skip PDF accessibility tagging in theme PDFs (faster on large docs)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [--verbose] [--jobs N] [--bench] [--no-outline] [--no-tagged-pdf]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Regenerates _site/ from README.md, docs/cli.md, and CHANGELOG.md.\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding repo root: %v\n", err)
		os.Exit(1)
	}

	log := newBuildLogger(*verbose)
	phases := newPhaseRecorder(log)
	start := time.Now()

	log.Vf("docsgen: root = %s, jobs = %d, verbose = %v", root, *jobs, *verbose)

	mdRenderer := goldmark.New(
		goldmark.WithExtensions(extension.GFM, meta.Meta),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(goldmarkHtml.WithHardWraps(), goldmarkHtml.WithUnsafe()),
	)

	tRead := time.Now()
	readme, _ := os.ReadFile(filepath.Join(root, "README.md"))
	cli, _ := os.ReadFile(filepath.Join(root, "docs", "cli.md"))
	changelog, _ := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	phases.logPhase("read sources", tRead)

	// goldmark Markdown is safe for concurrent use; render the three
	// sources in parallel. Sections must stay in document order, so the
	// final assembly appends in the deterministic order below.
	tMarkdown := time.Now()
	sections := renderSectionsParallel(readme, cli, changelog, mdRenderer)
	phases.logPhase("render markdown (parallel)", tMarkdown)

	tCompose := time.Now()
	landingHTML := buildLandingHTML()
	docsHTML := buildDocsHTML(sections)
	phases.logPhase("compose HTML", tCompose)

	tAssets := time.Now()
	outDir := filepath.Join(root, "_site")
	if mkErr := os.MkdirAll(outDir, 0755); mkErr != nil {
		fmt.Fprintf(os.Stderr, "error creating output directory: %v\n", mkErr)
		os.Exit(1)
	}
	writeTextAssets(outDir, map[string]string{
		"index.html":       landingHTML,
		"docs.html":        docsHTML,
		"robots.txt":       robotsTXT(),
		"sitemap.xml":      sitemapXML(),
		"site.webmanifest": webManifest(),
		llmsFileName():     llmsTXT(),
		"llms-full.txt":    llmsFullTXT(),
		"humans.txt":       humansTXT(),
		"favicon.svg":      faviconSVG(),
	})
	buildRawDocs(outDir, root, readme, cli, changelog, log)
	buildSearchIndex(outDir, sections, log)
	writeMetadata(outDir, log)
	writeSitemapTXT(outDir, log)
	phases.logPhase("write text + data assets", tAssets)

	tRaster := time.Now()
	rasterResults := generateRasterAssets(outDir, log, *jobs)
	phases.logPhase("raster assets (parallel)", tRaster)

	// PDFs and EPUBs share the same parsed docs; prepare once, then build
	// both in parallel — EPUBs need no Chrome, so they don't contend with
	// the PDF Chrome processes beyond the jobs semaphore.
	tFormats := time.Now()
	srcDir, docs, err := prepareDocsSource(readme, cli, changelog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error preparing docs source: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = os.RemoveAll(srcDir) }()

	var epubResults []renderResult
	var epubWG sync.WaitGroup
	epubWG.Add(1)
	go func() {
		defer epubWG.Done()
		epubResults = buildDocsEPUBs(outDir, docs, log, *jobs)
	}()
	pdfResults := generateDocsPDF(outDir, readme, cli, changelog, log, *jobs, *noOutline, *noTagged)
	epubWG.Wait()
	phases.logPhase("theme PDFs + EPUBs (parallel)", tFormats)

	// Optional benchmark: re-render the PDFs at jobs=1 to quantify the
	// parallelism speedup for this exact machine. Reuses the same source
	// dir but writes to a throwaway location so _site isn't disturbed.
	// The wall time for the *real* build is captured before the benchmark
	// runs, so the speedup compares like-for-like.
	buildTotal := time.Since(start)
	var benchElapsed time.Duration
	if *bench {
		log.Infof("")
		log.Infof("  Benchmarking parallel speedup (rendering 17 PDFs at jobs=1)...")
		benchDir, bdirErr := os.MkdirTemp("", "docsgen-bench-*")
		if bdirErr != nil {
			log.Warnf("benchmark: could not create temp dir: %v", bdirErr)
		} else {
			tBench := time.Now()
			benchResults := generateDocsPDF(benchDir, readme, cli, changelog, log, 1, *noOutline, *noTagged)
			benchElapsed = time.Since(tBench)
			for _, r := range benchResults {
				if !r.ok {
					log.Warnf("benchmark PDF %s failed: %v", r.name, r.err)
				}
			}
			_ = os.RemoveAll(benchDir)
			// Reported separately (not via phases) so the build timeline
			// stays comparable — the benchmark is extra work, not a build
			// phase, and would otherwise skew the phase-sum metric.
			log.Vf("  ✓ %-32s %s", "benchmark: PDFs at jobs=1", benchElapsed.Round(time.Millisecond))
		}
	}

	log.Infof("")
	log.Infof("  PDF quality checks ran on every theme PDF (see --verbose for the audit detail).")
	log.Infof("")
	for _, r := range rasterResults {
		if !r.ok {
			log.Warnf("raster asset %s failed: %v", r.name, r.err)
		}
	}
	for _, r := range pdfResults {
		if !r.ok {
			log.Warnf("theme PDF %s failed: %v", r.name, r.err)
		}
	}
	for _, r := range epubResults {
		if !r.ok {
			log.Warnf("theme EPUB %s failed: %v", r.name, r.err)
		}
	}

	allResults := append(append(append([]renderResult{}, rasterResults...), pdfResults...), epubResults...)

	// Persist the full machine-readable report before printing the human
	// summary, so _site/report.json always reflects this exact build.
	writeReportJSON(outDir, buildTotal, *jobs, *verbose, phases.allPhases(), allResults, log)

	printSummary(phases, buildTotal, allResults, *jobs, benchElapsed)
}

// printSummary renders the closing report: a phase timeline with ASCII
// bars, per-group artifact tables with full metrics, throughput numbers,
// and a grand total. benchElapsed (if non-zero) is the jobs=1 PDF time
// used to headline the parallel speedup.
func printSummary(phases *phaseRecorder, total time.Duration, results []renderResult, jobs int, benchElapsed time.Duration) {
	var b strings.Builder

	phaseSum, _ := phases.phaseStats()

	b.WriteString("\n")
	b.WriteString("  ╔══════════════════════════════════════════════════════════════╗\n")
	b.WriteString("  ║                 docsgen performance report                 ║\n")
	b.WriteString("  ╚══════════════════════════════════════════════════════════════╝\n")

	// big headline metrics
	okCount := 0
	var totalBytes int64
	for _, r := range results {
		if r.ok {
			okCount++
			totalBytes += r.bytes
		}
	}
	throughput := 0.0
	rate := 0.0
	if total.Seconds() > 0 {
		throughput = float64(okCount) / total.Seconds()
		rate = float64(totalBytes) / (1 << 20) / total.Seconds()
	}
	fmt.Fprintf(&b, "  wall time      %s\n", total.Round(time.Millisecond))
	fmt.Fprintf(&b, "  artifacts      %d ok / %d total\n", okCount, len(results))
	fmt.Fprintf(&b, "  total size     %s\n", formatBytes(int(totalBytes)))
	fmt.Fprintf(&b, "  throughput     %.1f artifacts/s  ·  %.2f MiB/s\n", throughput, rate)
	fmt.Fprintf(&b, "  phase sum      %s  (%.1fx of wall time → parallelism)\n",
		phaseSum.Round(time.Millisecond), ratioOf(phaseSum, total))
	if benchElapsed > 0 {
		speedup := ratioOf(benchElapsed, total)
		fmt.Fprintf(&b, "  PARALLELISM   jobs=%d · sequential %.2fs → parallel %.2fs = %.2fx speedup\n",
			jobs, benchElapsed.Seconds(), total.Seconds(), speedup)
	}

	// grouped artifact tables
	for _, group := range []string{groupRaster, groupPDF, groupEPUB} {
		var groupResults []renderResult
		for _, r := range results {
			if r.group == group {
				groupResults = append(groupResults, r)
			}
		}
		b.WriteString(renderResultsTable(groupResults, titleForGroup(group)))
	}

	// phase timeline with bars
	b.WriteString(phasesTable(phases.allPhases(), total))

	log := newBuildLogger(false)
	log.Infof("%s", strings.TrimRight(b.String(), "\n"))
}

func titleForGroup(group string) string {
	switch group {
	case groupRaster:
		return "Raster assets (PNG/OG card)"
	case groupPDF:
		return "Theme PDFs (headless Chrome)"
	case groupEPUB:
		return "Theme EPUBs (no Chrome)"
	}
	return group
}

// ratioOf returns d as a multiple of total (for parallelism headlines).
func ratioOf(d, total time.Duration) float64 {
	if total <= 0 {
		return 0
	}
	return float64(d) / float64(total)
}

// writeTextAssets writes every static text file to outDir, reporting each
// in verbose mode.
func writeTextAssets(outDir string, assets map[string]string) {
	names := make([]string, 0, len(assets))
	for name := range assets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(outDir, name), []byte(assets[name]), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", name, err)
			os.Exit(1)
		}
	}
}

// renderSectionsParallel renders the three markdown sources concurrently.
// goldmark.Markdown is documented as safe for concurrent use, so the three
// goroutines share one renderer. The returned slice is always in document
// order regardless of which goroutine finishes first.
func renderSectionsParallel(readme, cli, changelog []byte, md goldmark.Markdown) []Section {
	type res struct {
		sections []Section
	}
	ch := make(chan res, 3)

	go func() { ch <- res{readmeSections(readme, md)} }()
	go func() { ch <- res{cliSections(cli, md)} }()
	go func() { ch <- res{changelogSection(changelog, md)} }()

	var rm, cs, cl []Section
	for i := 0; i < 3; i++ {
		r := <-ch
		// Identify which one arrived; each source produces a distinct
		// first section id (cli sections are prefixed, changelog is
		// "changelog"), but ordering is guaranteed by assembling in the
		// fixed order below, so capture by channel position instead.
		switch i {
		case 0:
			rm = r.sections
		case 1:
			cs = r.sections
		case 2:
			cl = r.sections
		}
	}

	sections := make([]Section, 1, 1+len(rm)+len(cs)+len(cl))
	sections[0] = heroSection()
	sections = append(sections, rm...)
	sections = append(sections, cs...)
	sections = append(sections, cl...)
	return sections
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func heroSection() Section {
	ascii := ` ###   ###        ####  ####  ##### ##### ##### #   #       ####  ####  #####
#     #   #       #   # #   # #       #     #    # #        #   # #   # #
#  ## #   # ##### ####  ####  ####    #     #     #   ##### ####  #   # ####
#   # #   #       #     #  #  #       #     #     #         #     #   # #
 ###   ###        #     #   # #####   #     #     #         #     ####  #
`
	return Section{
		ID:      heroSectionID,
		Title:   siteName,
		Eyebrow: "MDX &rarr; PDF, via headless Chrome",
		Content: `<pre class="hero-ascii">` + ascii + `</pre>
<div class="hero-line"></div>
<p class="hero-tagline">Turn a folder of MDX into a beautifully typeset, print-ready PDF &mdash; no LaTeX, no design tools, no fuss.</p>
<div class="hero-meta">
  <span>Library + CLI</span>
  <span>Go 1.26+</span>
  <span>MIT</span>
</div>
<a class="download-pdf-btn" id="download-pdf-btn" href="` + docsPDFDefault + `" download>
  <span>Download these docs as a PDF</span>
  <span class="download-pdf-sub" id="download-pdf-sub">in the Classic theme &mdash; rendered by go-pretty-pdf itself</span>
</a>
<div class="hero-install">
  <div class="install-block">
    <span class="install-label">CLI</span>
    <pre class="install-cmd"><code>$ go install github.com/sazardev/go-pretty-pdf/cmd/pretty-pdf@latest</code></pre>
  </div>
  <div class="install-block">
    <span class="install-label">Library</span>
    <pre class="install-cmd"><code>$ go get github.com/sazardev/go-pretty-pdf</code></pre>
  </div>
</div>
<p class="hero-requirements">
  Requires Chrome or Chromium for PDF rendering.
  <a href="#quick-start">Get started</a> &middot;
  <a href="https://github.com/sazardev/go-pretty-pdf">GitHub</a> &middot;
  <a href="https://pkg.go.dev/github.com/sazardev/go-pretty-pdf">pkg.go.dev</a>
</p>`,
	}
}

func renderMarkdown(src []byte, md goldmark.Markdown) string {
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return fmt.Sprintf("<p>Error rendering markdown: %v</p>", err)
	}
	return buf.String()
}

func readmeSections(src []byte, md goldmark.Markdown) []Section {
	html := renderMarkdown(src, md)
	parts := splitByHeadings(html)

	sectionMap := map[string]string{
		"install":             "Installation",
		"quick-start":         "Quick Start",
		"how-it-works":        "How It Works",
		"mdx-format":          "MDX Format",
		"built-in-components": "Built-in Components",
		"configuration":       "Configuration",
		"library-api":         "Library API",
		"themes":              "Themes",
		"cli-reference":       "CLI Reference",
	}

	sections := make([]Section, 0, len(sectionMap))
	for _, part := range parts {
		id := anchorFromHeading(part.Heading)
		if title, ok := sectionMap[id]; ok {
			sections = append(sections, Section{ID: id, Title: title, Content: part.Body})
		}
	}
	return sections
}

func cliSections(src []byte, md goldmark.Markdown) []Section {
	html := renderMarkdown(src, md)
	parts := splitByHeadings(html)

	sectionMap := map[string]string{
		"overview":           "CLI Overview",
		"requirements":       "Requirements",
		"usage":              "Usage",
		"global-flags":       "Global Flags",
		"commands":           "Commands",
		"config-file":        "Config File",
		"themes":             "Themes",
		"template-variables": "Template Variables",
		"environment":        "Environment",
		"exit-codes":         "Exit Codes",
	}

	sections := make([]Section, 0, 17)
	for _, part := range parts {
		id := anchorFromHeading(part.Heading)
		if title, ok := sectionMap[id]; ok {
			sections = append(sections, Section{ID: "cli-" + id, Title: title, Content: part.Body})
		}
		if id == "commands" {
			cmdSubs := splitByH3(part.Body)
			cmdMap := map[string]string{
				"build":      "build",
				"check":      "check",
				"theme":      "theme",
				"init":       "init",
				"serve":      "serve",
				"watch":      "watch",
				"version":    "version",
				"completion": "completion",
			}
			for _, sub := range cmdSubs {
				subID := anchorFromHeading(sub.Heading)
				if cmdLabel, ok := cmdMap[subID]; ok {
					sections = append(sections, Section{
						ID:      "cmd-" + subID,
						Title:   "pretty-pdf " + cmdLabel,
						Content: sub.Body,
					})
				}
			}
		}
	}
	return sections
}

func changelogSection(src []byte, md goldmark.Markdown) []Section {
	// Drop the file's own leading "# Changelog" H1: the section already
	// renders "Changelog" as its own heading, and a page must have exactly
	// one <h1> (the hero) for a clean, crawlable document outline.
	body := regexp.MustCompile(`(?m)^#\s+Changelog\s*\n`).ReplaceAll(src, nil)
	return []Section{{
		ID:      "changelog",
		Title:   "Changelog",
		Content: renderMarkdown(body, md),
	}}
}

type headingPart struct {
	Heading string
	Level   int
	Body    string
}

func splitByH3(html string) []headingPart {
	h3Re := regexp.MustCompile(`<h3[^>]*>`)
	h3CloseRe := regexp.MustCompile(`</h3>`)
	headingTextRe := regexp.MustCompile(`<h3[^>]*>(.*?)</h3>`)

	openMatches := h3Re.FindAllStringSubmatchIndex(html, -1)
	if len(openMatches) == 0 {
		return []headingPart{{Heading: "", Level: 1, Body: html}}
	}

	parts := make([]headingPart, len(openMatches))
	for i, om := range openMatches {
		bodyStart := h3CloseRe.FindStringIndex(html[om[1]:])
		if bodyStart == nil {
			continue
		}
		contentStart := om[1] + bodyStart[1]
		contentEnd := len(html)
		if i+1 < len(openMatches) {
			contentEnd = openMatches[i+1][0]
		}
		headingMatch := headingTextRe.FindStringSubmatch(html[om[0]:contentStart])
		heading := ""
		if len(headingMatch) >= 2 {
			heading = headingMatch[1]
		}
		parts[i] = headingPart{Heading: heading, Level: 3, Body: strings.TrimSpace(html[contentStart:contentEnd])}
	}
	return parts
}

func splitByHeadings(html string) []headingPart {
	h2Re := regexp.MustCompile(`<h2[^>]*>`)
	h2CloseRe := regexp.MustCompile(`</h2>`)
	headingTextRe := regexp.MustCompile(`<h2[^>]*>(.*?)</h2>`)

	openMatches := h2Re.FindAllStringSubmatchIndex(html, -1)
	if len(openMatches) == 0 {
		return []headingPart{{Heading: "", Level: 1, Body: html}}
	}

	parts := make([]headingPart, len(openMatches))
	for i, om := range openMatches {
		bodyStart := h2CloseRe.FindStringIndex(html[om[1]:])
		if bodyStart == nil {
			continue
		}
		contentStart := om[1] + bodyStart[1]
		contentEnd := len(html)
		if i+1 < len(openMatches) {
			contentEnd = openMatches[i+1][0]
		}
		headingMatch := headingTextRe.FindStringSubmatch(html[om[0]:contentStart])
		heading := ""
		if len(headingMatch) >= 2 {
			heading = headingMatch[1]
		}
		parts[i] = headingPart{Heading: heading, Level: 2, Body: strings.TrimSpace(html[contentStart:contentEnd])}
	}
	return parts
}

func anchorFromHeading(html string) string {
	lower := strings.ToLower(stripHTMLTags(html))
	slug := regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.TrimSpace(lower), "-")
	return strings.Trim(slug, "-")
}

func stripHTMLTags(s string) string {
	return regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
}

// breadcrumbJSONLD is the BreadcrumbList structured data for docs.html,
// giving crawlers an explicit hierarchy instead of guessing at it from
// links. Both landing and docs reference the same path so breadcrumb
// continuity holds across the two pages.
func breadcrumbJSONLD() string {
	return `{
  "@context": "https://schema.org",
  "@type": "BreadcrumbList",
  "itemListElement": [
    {
      "@type": "ListItem",
      "position": 1,
      "name": "go-pretty-pdf",
      "item": "` + siteBaseURL + `"
    },
    {
      "@type": "ListItem",
      "position": 2,
      "name": "Documentation",
      "item": "` + siteBaseURL + `docs.html"
    }
  ]
}`
}

func buildDocsHTML(sections []Section) string {
	n := len(sections)
	navItems := make([]string, n)
	bodyParts := make([]string, n)

	for i, s := range sections {
		navItems[i] = fmt.Sprintf(`<a href="#%s">%s</a>`, s.ID, s.Title)
		cls := "section"
		eyebrow := ""
		headingTag := "h2"
		if s.ID == heroSectionID {
			cls = "section hero-section"
			eyebrow = fmt.Sprintf(`<p class="hero-eyebrow">%s</p>`, s.Eyebrow)
			// The hero is the page's single <h1>; every other section heading
			// is an <h2>, giving crawlers (and assistive tech) an unambiguous
			// document outline instead of a flat run of <h2>s.
			headingTag = "h1"
		}
		bodyParts[i] = fmt.Sprintf(
			`<section id="%s" class="%s">%s<%s class="section-title">%s</%s><div class="section-content">%s</div></section>`,
			s.ID, cls, eyebrow, headingTag, s.Title, headingTag, s.Content)
	}

	docsTitle := "go-pretty-pdf — Full Documentation (CLI Reference, MDX Format, Changelog)"
	docsURL := siteBaseURL + "docs.html"

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-site-theme="classic">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<meta name="description" content="%s">
<meta name="keywords" content="%s">
<meta name="author" content="sazardev">
<meta name="robots" content="index, follow, max-image-preview:large, max-snippet:-1, max-video-preview:-1">
<meta name="googlebot" content="index, follow, max-image-preview:large, max-snippet:-1, max-video-preview:-1">
<meta name="bingbot" content="index, follow, max-image-preview:large, max-snippet:-1, max-video-preview:-1">
<meta name="googlebot-news" content="noindex">
<link rel="canonical" href="%s">
<meta name="referrer" content="strict-origin-when-cross-origin">
<meta name="format-detection" content="telephone=no, email=no, address=no">

<link rel="icon" href="favicon.svg" type="image/svg+xml">
<link rel="icon" href="favicon-32.png" type="image/png" sizes="32x32">
<link rel="apple-touch-icon" href="apple-touch-icon.png" sizes="180x180">
<link rel="manifest" href="site.webmanifest">
<meta name="theme-color" content="#fffdf8" media="(prefers-color-scheme: light)">
<meta name="theme-color" content="#121212" media="(prefers-color-scheme: dark)">
<meta name="mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-status-bar-style" content="default">
<meta name="application-name" content="go-pretty-pdf">

<meta property="og:type" content="article">
<meta property="og:site_name" content="go-pretty-pdf">
<meta property="og:title" content="%s">
<meta property="og:description" content="%s">
<meta property="og:url" content="%s">
<meta property="og:locale" content="en_US">
<meta property="og:image" content="%sog-image.png">
<meta property="og:image:url" content="%sog-image.png">
<meta property="og:image:secure_url" content="%sog-image.png">
<meta property="og:image:type" content="image/png">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">
<meta property="og:image:alt" content="go-pretty-pdf — write Markdown, ship a book.">

<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="%s">
<meta name="twitter:description" content="%s">
<meta name="twitter:image" content="%sog-image.png">
<meta name="twitter:image:alt" content="go-pretty-pdf — write Markdown, ship a book.">

<meta name="twitter:site" content="@sazardev">
<meta name="twitter:creator" content="@sazardev">
<meta name="twitter:domain" content="%s">

<script type="application/ld+json">%s</script>
<script type="application/ld+json">%s</script>

<style>
%s
</style>
</head>
<body>
<nav class="sidebar">
  <div class="sidebar-brand">
    <a href="index.html" class="sidebar-home-link">&larr; pretty-pdf</a>
    <a href="#hero">go-pretty-pdf docs</a>
    <span class="sidebar-tagline">Write Markdown. Ship a book.</span>
    <button type="button" class="nav-toggle" id="nav-toggle" aria-expanded="false" aria-controls="sidebar-nav" aria-label="Toggle navigation">&#9776;</button>
  </div>
  <div class="sidebar-nav" id="sidebar-nav">
    %s
  </div>
  <div class="sidebar-footer">
    <button type="button" class="palette-trigger" id="palette-trigger" aria-label="Open command palette">
      <span>Search sections</span>
      <kbd id="palette-shortcut-hint">Ctrl K</kbd>
    </button>
    %s
  </div>
</nav>
<main class="main">
  %s
  <footer class="footer">
    <p>Generated from source &mdash; <a href="https://github.com/sazardev/go-pretty-pdf">GitHub</a> &middot; <a href="https://pkg.go.dev/github.com/sazardev/go-pretty-pdf">pkg.go.dev</a></p>
  </footer>
</main>
%s
<script>
%s
</script>
</body>
</html>`,
		docsTitle, siteDescription, siteKeywords, docsURL,
		docsTitle, siteDescription, docsURL, siteBaseURL,
		siteBaseURL, siteBaseURL,
		docsTitle, siteDescription, siteBaseURL,
		siteHost,
		jsonLD(), breadcrumbJSONLD(),
		siteCSS+"\n"+generatedThemeCSS(), strings.Join(navItems, "\n    "), themeSwitcherHTML(), strings.Join(bodyParts, "\n  "), commandPaletteHTML(), siteJS)
}

// jsonLD returns the page's structured data: a WebSite entry plus a
// SoftwareApplication entry describing the CLI/library, an Organization
// entry for the project itself, and (via faqJSONLD) rich results for the
// landing page's FAQ. This is what lets search engines and LLM crawlers
// identify go-pretty-pdf as a concrete, installable, open-source tool
// rather than just a prose page, and qualifies the landing page for FAQ
// rich results.
func jsonLD() string {
	return `{
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "Organization",
      "name": "go-pretty-pdf",
      "url": "` + siteBaseURL + `",
      "logo": "` + siteBaseURL + `favicon.svg",
      "description": "` + siteDescription + `",
      "foundingDate": "2026",
      "sameAs": [
        "` + siteRepoURL + `",
        "` + siteRepoURL + `/discussions",
        "` + siteRepoURL + `/issues"
      ]
    },
    {
      "@type": "WebSite",
      "name": "go-pretty-pdf",
      "url": "` + siteBaseURL + `",
      "description": "` + siteDescription + `",
      "inLanguage": "en",
      "publisher": {
        "@type": "Organization",
        "name": "sazardev",
        "url": "https://github.com/sazardev"
      }
    },
    {
      "@type": "SoftwareApplication",
      "name": "go-pretty-pdf",
      "description": "` + siteDescription + `",
      "url": "` + siteBaseURL + `",
      "applicationCategory": "DeveloperApplication",
      "applicationSubCategory": "Document Generation",
      "operatingSystem": "Linux, macOS, Windows",
      "programmingLanguage": "Go",
      "softwareVersion": "latest",
      "requirements": "Go 1.26+; Chrome or Chromium for PDF rendering (auto-downloaded)",
      "featureList": [
        "Markdown/MDX to PDF and EPUB",
        "17 built-in print themes",
        "Headless Chrome rendering",
        "Automatic table of contents and PDF bookmarks",
        "Syntax highlighting via Chroma",
        "Print-ready trim sizes (6x9in, A5, mm/in)",
        "Custom .theme.yml themes",
        "Automatic quality audit"
      ],
      "license": "` + siteRepoURL + `/blob/master/LICENSE",
      "codeRepository": "` + siteRepoURL + `",
      "downloadUrl": "` + siteRepoURL + `/releases",
      "installUrl": "` + siteRepoURL + `/releases/latest",
      "offers": {
        "@type": "Offer",
        "price": "0",
        "priceCurrency": "USD",
        "category": "Free open-source software"
      },
      "author": {
        "@type": "Person",
        "name": "sazardev",
        "url": "https://github.com/sazardev"
      }
    }
  ]
}`
}

// faqJSONLD is the FAQPage structured data for the landing page. It must
// stay in sync with the on-page FAQ section in landing.go — the questions
// here are the same ones a visitor actually reads, so the rich result
// Google shows never diverges from what's on screen.
func faqJSONLD() string {
	return `{
  "@context": "https://schema.org",
  "@type": "FAQPage",
  "mainEntity": [
    {
      "@type": "Question",
      "name": "Does go-pretty-pdf require LaTeX?",
      "acceptedAnswer": {
        "@type": "Answer",
        "text": "No. go-pretty-pdf renders PDFs from Markdown/MDX using headless Chrome — no LaTeX, no separate design tool, nothing to install but the binary."
      }
    },
    {
      "@type": "Question",
      "name": "Does go-pretty-pdf require a manual Chrome install?",
      "acceptedAnswer": {
        "@type": "Answer",
        "text": "Usually not. On first render chromemgr auto-downloads a headless Chrome build if no system Chrome or --chrome-path is found. Only on linux/arm64 you must provide Chrome via --chrome-path."
      }
    },
    {
      "@type": "Question",
      "name": "Can I build an EPUB without Chrome?",
      "acceptedAnswer": {
        "@type": "Answer",
        "text": "Yes. The 'epub' command builds EPUB 3 output from the same Markdown with no Chrome or Chromium required at all."
      }
    },
    {
      "@type": "Question",
      "name": "What input file formats are supported?",
      "acceptedAnswer": {
        "@type": "Answer",
        "text": "Markdown (.md), MDX (.mdx), and even bare .txt files. Documents are ordered by their [X.Y.Z] frontmatter id, not by filename; missing frontmatter gets an id/title generated from the filename."
      }
    },
    {
      "@type": "Question",
      "name": "How many built-in themes does go-pretty-pdf ship?",
      "acceptedAnswer": {
        "@type": "Answer",
        "text": "17 built-in themes ranging from minimal and modern to Gruvbox, LaTeX-style academic, corporate, and government letterhead, plus a custom .theme.yml theme system and per-theme color/font overrides."
      }
    },
    {
      "@type": "Question",
      "name": "Can I build both PDF and EPUB from one source?",
      "acceptedAnswer": {
        "@type": "Answer",
        "text": "Yes. A single command (--formats pdf,epub) builds both formats from the same Markdown in one pass — no separate pipeline to maintain."
      }
    },
    {
      "@type": "Question",
      "name": "Is go-pretty-pdf free?",
      "acceptedAnswer": {
        "@type": "Answer",
        "text": "Yes. go-pretty-pdf is MIT-licensed, open source, with no paid tier. Every feature ships in the core binary."
      }
    }
  ]
}`
}

func commandPaletteHTML() string {
	return `<div class="command-palette" id="command-palette" role="dialog" aria-modal="true" aria-label="Command palette" hidden>
  <div class="command-palette-backdrop" data-palette-close></div>
  <div class="command-palette-panel">
    <div class="command-palette-input-row">
      <span class="command-palette-prompt">&gt;</span>
      <input type="text" id="command-palette-input" class="command-palette-input" placeholder="Jump to a section&hellip;" autocomplete="off" autocapitalize="off" spellcheck="false">
      <kbd>ESC</kbd>
    </div>
    <ul class="command-palette-results" id="command-palette-results"></ul>
  </div>
</div>`
}

// themeSwitcherHTML renders one swatch per theme.List() entry — adding a
// builtin theme to theme/builtin.go is enough for it to show up here, with
// correct colors (swatchGradient reads the theme's own CSS) and no
// per-theme code to write.
func themeSwitcherHTML() string {
	var b strings.Builder
	b.WriteString(`<div class="theme-switcher">
    <span class="theme-switcher-label">Theme</span>
    <div class="theme-swatches">
`)
	for _, t := range theme.List() {
		pressed := "false"
		if t.Name == theme.NameClassic {
			pressed = "true"
		}
		name := displayName(t.Name)
		fmt.Fprintf(&b, `      <button type="button" class="theme-swatch" data-theme="%s" title="%s &mdash; %s" aria-pressed="%s">
        <span class="swatch-dot" style="background:%s"></span>
        <span class="theme-swatch-label">%s</span>
      </button>
`, t.Name, name, t.Description, pressed, swatchGradient(t), name)
	}
	b.WriteString(`    </div>
  </div>`)
	return b.String()
}

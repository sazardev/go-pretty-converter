package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
	siteRepoURL     = "https://github.com/sazardev/go-pretty-pdf"
	siteTitle       = "go-pretty-pdf — Turn Markdown into Beautiful, Print-Ready PDFs (Go)"
	siteDescription = "go-pretty-pdf turns a folder of Markdown/MDX into a beautifully typeset, print-ready PDF via headless Chrome — as a Go library or CLI. No LaTeX, no design tools."
	siteKeywords    = "markdown to pdf, mdx to pdf, go pdf generator, golang pdf library, cli pdf generator, print-ready pdf, headless chrome pdf, markdown book generator, mdx renderer"
)

type Section struct {
	ID      string
	Title   string
	Eyebrow string
	Content string
}

func main() {
	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding repo root: %v\n", err)
		os.Exit(1)
	}

	mdRenderer := goldmark.New(
		goldmark.WithExtensions(extension.GFM, meta.Meta),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(goldmarkHtml.WithHardWraps(), goldmarkHtml.WithUnsafe()),
	)

	readme, _ := os.ReadFile(filepath.Join(root, "README.md"))
	cli, _ := os.ReadFile(filepath.Join(root, "docs", "cli.md"))
	changelog, _ := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))

	sections := make([]Section, 1, 28)
	sections[0] = heroSection()
	sections = append(sections, readmeSections(readme, mdRenderer)...)
	sections = append(sections, cliSections(cli, mdRenderer)...)
	sections = append(sections, changelogSection(changelog, mdRenderer)...)

	landingHTML := buildLandingHTML()
	docsHTML := buildDocsHTML(sections)
	outDir := filepath.Join(root, "_site")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating output directory: %v\n", err)
		os.Exit(1)
	}
	textAssets := map[string]string{
		"index.html":       landingHTML,
		"docs.html":        docsHTML,
		"robots.txt":       robotsTXT(),
		"sitemap.xml":      sitemapXML(),
		"site.webmanifest": webManifest(),
		"llms.txt":         llmsTXT(),
		"favicon.svg":      faviconSVG(),
	}
	for name, content := range textAssets {
		if err := os.WriteFile(filepath.Join(outDir, name), []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", name, err)
			os.Exit(1)
		}
	}

	generateRasterAssets(outDir)
	generateDocsPDF(outDir, readme, cli, changelog)

	fmt.Println("Site generated at _site/index.html (landing) and _site/docs.html (full reference)")
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
		ID:      "hero",
		Title:   "go-pretty-pdf",
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

func buildDocsHTML(sections []Section) string {
	n := len(sections)
	navItems := make([]string, n)
	bodyParts := make([]string, n)

	for i, s := range sections {
		navItems[i] = fmt.Sprintf(`<a href="#%s">%s</a>`, s.ID, s.Title)
		cls := "section"
		eyebrow := ""
		headingTag := "h2"
		if s.ID == "hero" {
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
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<meta name="description" content="%s">
<meta name="keywords" content="%s">
<meta name="author" content="sazardev">
<meta name="robots" content="index, follow, max-image-preview:large, max-snippet:-1, max-video-preview:-1">
<meta name="googlebot" content="index, follow">
<link rel="canonical" href="%s">

<link rel="icon" href="favicon.svg" type="image/svg+xml">
<link rel="icon" href="favicon-32.png" type="image/png" sizes="32x32">
<link rel="apple-touch-icon" href="apple-touch-icon.png" sizes="180x180">
<link rel="manifest" href="site.webmanifest">
<meta name="theme-color" content="#fffdf8" media="(prefers-color-scheme: light)">
<meta name="theme-color" content="#121212" media="(prefers-color-scheme: dark)">

<meta property="og:type" content="website">
<meta property="og:site_name" content="go-pretty-pdf">
<meta property="og:title" content="%s">
<meta property="og:description" content="%s">
<meta property="og:url" content="%s">
<meta property="og:image" content="%sog-image.png">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">
<meta property="og:image:alt" content="go-pretty-pdf &mdash; write Markdown, ship a book.">
<meta property="og:locale" content="en_US">

<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="%s">
<meta name="twitter:description" content="%s">
<meta name="twitter:image" content="%sog-image.png">

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
		docsTitle, siteDescription, siteBaseURL,
		jsonLD(),
		siteCSS+"\n"+generatedThemeCSS(), strings.Join(navItems, "\n    "), themeSwitcherHTML(), strings.Join(bodyParts, "\n  "), commandPaletteHTML(), siteJS)
}

// jsonLD returns the page's structured data: a WebSite entry plus a
// SoftwareApplication entry describing the CLI/library, so search engines
// and LLM crawlers can identify go-pretty-pdf as a concrete, installable
// open-source tool rather than just a prose page.
func jsonLD() string {
	return `{
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "WebSite",
      "name": "go-pretty-pdf",
      "url": "` + siteBaseURL + `",
      "description": "` + siteDescription + `",
      "inLanguage": "en"
    },
    {
      "@type": "SoftwareApplication",
      "name": "go-pretty-pdf",
      "description": "` + siteDescription + `",
      "url": "` + siteBaseURL + `",
      "applicationCategory": "DeveloperApplication",
      "operatingSystem": "Linux, macOS, Windows",
      "programmingLanguage": "Go",
      "license": "` + siteRepoURL + `/blob/master/LICENSE",
      "codeRepository": "` + siteRepoURL + `",
      "downloadUrl": "` + siteRepoURL + `/releases",
      "offers": {
        "@type": "Offer",
        "price": "0",
        "priceCurrency": "USD"
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

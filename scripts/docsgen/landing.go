package main

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/sazardev/go-pretty-pdf/theme"
)

//go:embed assets/landing.css
var landingCSS string

//go:embed assets/landing.js
var landingJS string

// landingDefaultTheme is the theme the marketing page previews before a
// visitor touches the theme switcher — bold and dark on purpose, since this
// page's job is to be memorable, unlike docs.html's calmer "classic" default
// for long-form reading.
const landingDefaultTheme = theme.NameGruvbox

// landingThemeCSS builds one [data-theme="x"] { --bg: ...; } block per
// builtin theme, read straight from theme.List() — the exact same palette
// data go-pretty-pdf renders into a real PDF, never a hand-copied hex code.
//
// The default theme's block is deliberately NOT also written as a combined
// ":root, [data-theme=...]" selector: :root matches the <html> element
// unconditionally, so if that combined rule weren't first in source order,
// it would permanently win the cascade over every other theme's block at
// equal specificity, no matter which data-theme is actually selected. A
// plain, separate ":root" fallback up front (used only until JS/the
// server-rendered attribute takes over) avoids that trap.
func landingThemeCSS() string {
	var b strings.Builder
	b.WriteString("/* Generated from theme.List() by scripts/docsgen — do not hand-edit. */\n")
	fmt.Fprintf(&b, ":root { %s }\n", landingThemeDeclarations(mustTheme(landingDefaultTheme)))
	for _, t := range theme.List() {
		fmt.Fprintf(&b, "[data-theme=%q] {\n  %s\n}\n", t.Name, landingThemeDeclarations(t))
	}
	return b.String()
}

func mustTheme(name string) theme.Theme {
	t, ok := theme.Get(name)
	if !ok {
		panic("landing.go: unknown theme " + name)
	}
	return t
}

// landingThemeDeclarations returns the --bg/--surface/--ink/--accent AND
// --font-heading/--font-body/--font-code declarations for t (no selector,
// no braces) — callers wrap them. The fonts are the theme's real
// --pdf-font-* stacks, so picking "classic" genuinely swaps the landing
// page's headline into Georgia/Palatino and "gruvbox" into JetBrains Mono,
// not just its colors.
func landingThemeDeclarations(t theme.Theme) string {
	vars := extractThemeVars(t.CSS)
	var b strings.Builder
	if v, ok := vars[varBg]; ok {
		fmt.Fprintf(&b, "  --bg: %s;\n", v)
	}
	if v, ok := vars[varSurface]; ok {
		fmt.Fprintf(&b, "  --surface: %s;\n", v)
	}
	if v, ok := vars[varPrimary]; ok {
		fmt.Fprintf(&b, "  --ink: %s;\n", v)
	}
	if v, ok := vars[varAccent]; ok {
		fmt.Fprintf(&b, "  --accent: %s;\n", v)
	}
	if v, ok := vars[varFontHeading]; ok {
		fmt.Fprintf(&b, "  --font-heading: %s;\n", v)
	}
	if v, ok := vars[varFontBody]; ok {
		fmt.Fprintf(&b, "  --font-body: %s;\n", v)
	}
	if v, ok := vars[varFontCode]; ok {
		fmt.Fprintf(&b, "  --font-code: %s;\n", v)
	}
	return b.String()
}

// themeSwatchSpans renders the three bg/ink/accent color dots shared by
// both the Themes-section cards and the nav dropdown's compact options.
func themeSwatchSpans(t theme.Theme) string {
	vars := extractThemeVars(t.CSS)
	return fmt.Sprintf(`<span class="swatch" style="background:%s"></span><span class="swatch" style="background:%s"></span><span class="swatch" style="background:%s"></span>`,
		vars[varBg], vars[varPrimary], vars[varAccent])
}

// landingThemeCards renders one clickable swatch card per builtin theme for
// the "Themes" showcase section — clicking sets data-theme on <html>, and
// the CSS landingThemeCSS() generated above takes over instantly, so the
// whole page re-skins itself using the theme's real colors and fonts.
func landingThemeCards() string {
	var b strings.Builder
	for _, t := range theme.List() {
		active := ""
		if t.Name == landingDefaultTheme {
			active = " active"
		}
		fmt.Fprintf(&b, `<div class="theme-card%s" data-name="%s" tabindex="0" role="button" aria-label="Preview %s theme">
  <div class="swatches">%s</div>
  <div class="theme-name">%s</div>
  <div class="theme-cat">%s</div>
</div>
`, active, t.Name, t.Name, themeSwatchSpans(t), t.Name, t.Category)
	}
	return b.String()
}

// landingThemeDropdownOptions renders the compact theme list shown inside
// the nav bar's "Theme: X" dropdown — the same 17 themes, same click
// behavior (applyTheme in landing.js matches on .theme-card AND
// .theme-option), just without the category line so 17 of them fit in a
// narrow panel.
func landingThemeDropdownOptions() string {
	var b strings.Builder
	for _, t := range theme.List() {
		active := ""
		if t.Name == landingDefaultTheme {
			active = " active"
		}
		fmt.Fprintf(&b, `<button type="button" class="theme-option%s" data-name="%s">
  <span class="swatches">%s</span>
  <span class="theme-name">%s</span>
</button>
`, active, t.Name, themeSwatchSpans(t), t.Name)
	}
	return b.String()
}

// buildLandingHTML assembles index.html: a product-marketing homepage
// (hero, how-it-works, features, live theme showcase, comparison, CTA)
// rather than a documentation dump — full reference docs live at docs.html.
func buildLandingHTML() string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="%s">
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
<meta name="theme-color" content="#282828">

<meta property="og:type" content="website">
<meta property="og:site_name" content="go-pretty-pdf">
<meta property="og:title" content="%s">
<meta property="og:description" content="%s">
<meta property="og:url" content="%s">
<meta property="og:image" content="%sog-image.png">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">
<meta property="og:image:alt" content="go-pretty-pdf &mdash; write Markdown, ship a beautiful PDF.">
<meta property="og:locale" content="en_US">

<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="%s">
<meta name="twitter:description" content="%s">
<meta name="twitter:image" content="%sog-image.png">

<script type="application/ld+json">%s</script>

<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Fraunces:ital,opsz,wght@0,9..144,400;0,9..144,500;0,9..144,600;1,9..144,500&family=Space+Mono:wght@400;700&display=swap" rel="stylesheet">

<style>
%s
%s
</style>
</head>
<body>

<header class="nav">
  <div class="nav-inner">
    <a href="index.html" class="brand">pretty<span class="dim">-pdf</span></a>
    <nav class="links">
      <a href="#how">How it works</a>
      <a href="#features">Features</a>
      <a href="#themes">Themes</a>
      <a href="docs.html">Docs</a>
      <a href="docs.html#changelog">Changelog</a>
    </nav>
    <div class="nav-cta">
      <div class="theme-dropdown">
        <button type="button" class="theme-dropdown-btn" id="themeDropdownBtn" aria-haspopup="true" aria-expanded="false">
          <span class="swatches" id="navThemeSwatches">%s</span>
          <span>Theme: <b id="navThemeLabel">%s</b></span>
          <svg class="chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M6 9l6 6 6-6"/></svg>
        </button>
        <div class="theme-dropdown-panel" id="themeDropdownPanel" hidden>
%s        </div>
      </div>
      <a class="gh-stars" href="%s" target="_blank" rel="noopener">
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 0a8 8 0 00-2.53 15.59c.4.07.55-.17.55-.38l-.01-1.49c-2.01.44-2.44-.97-2.44-.97-.33-.84-.81-1.06-.81-1.06-.66-.45.05-.44.05-.44.73.05 1.12.75 1.12.75.65 1.11 1.7.79 2.12.6.07-.48.26-.79.46-.97-1.6-.18-3.29-.8-3.29-3.56 0-.79.28-1.43.75-1.93-.08-.18-.32-.92.07-1.92 0 0 .61-.2 2 .73a6.9 6.9 0 013.64 0c1.39-.94 2-.73 2-.73.39 1 .15 1.74.07 1.92.47.5.75 1.14.75 1.93 0 2.77-1.69 3.38-3.3 3.56.27.23.51.68.51 1.37l-.01 2.03c0 .2.14.44.55.37A8 8 0 008 0z"/></svg>
        GitHub
      </a>
    </div>
  </div>
</header>

<section class="hero">
  <div class="container">
    <p class="eyebrow"><b>go</b> &middot; CLI + library</p>
    <h1 class="headline">Write Markdown.<br>Ship a beautiful <em>PDF.</em></h1>
    <p class="sub">
      <code>pretty-pdf</code> turns a folder of Markdown into a typeset, print-ready PDF &mdash; or EPUB &mdash;
      via headless Chrome. No LaTeX, no design tools, nothing to install but a binary.
    </p>
    <div class="cta-row">
      <div class="install-line">
        <span class="prompt">$</span>
        <code>go install github.com/sazardev/go-pretty-pdf/cmd/pretty-pdf@latest</code>
        <button class="copy-btn" id="copyInstall" aria-label="Copy install command">%s</button>
      </div>
      <a href="docs.html" class="text-cta">Read the docs &rarr;</a>
    </div>

    <div class="terminal">
      <div class="terminal-bar">
        <span class="tdot"></span><span class="tdot"></span><span class="tdot"></span>
        <span class="terminal-title">book/ &mdash; pretty-pdf build</span>
      </div>
      <div class="terminal-body" id="termBody"></div>
    </div>
  </div>
</section>

<section id="how">
  <div class="container">
    <div class="section-head center reveal">
      <p class="kicker">How it works</p>
      <h2>Three steps between a folder of notes and a finished book.</h2>
    </div>
    <div class="steps reveal">
      <div class="step">
        <p class="step-num">01 &mdash; Write</p>
        <h3>Plain Markdown, plain frontmatter</h3>
        <p><code>.md</code>, <code>.mdx</code>, or even bare <code>.txt</code> &mdash; missing frontmatter gets an id/title generated from the filename automatically.</p>
        <pre><span class="c">---</span>
<span class="k">id:</span> <span class="s">"[1.0.0]"</span>
<span class="k">title:</span> <span class="s">Getting Started</span>
<span class="c">---</span>

# Getting Started</pre>
      </div>
      <div class="step">
        <p class="step-num">02 &mdash; Configure</p>
        <h3>Pick a theme, a trim size, a cover</h3>
        <p>17 built-in themes, custom paper dimensions for print-on-demand, and a cover image &mdash; all from one config file.</p>
        <pre><span class="k">theme:</span> <span class="s">gruvbox</span>
<span class="k">paper:</span> <span class="s">6x9in</span>
<span class="k">cover_image:</span> <span class="s">cover.svg</span></pre>
      </div>
      <div class="step">
        <p class="step-num">03 &mdash; Ship</p>
        <h3>One command, two formats</h3>
        <p>A quality audit checks for overflow, broken images, and clipped headings before you ever open the file.</p>
        <pre><span class="c">$</span> pretty-pdf build \
    --formats pdf,epub</pre>
      </div>
    </div>
  </div>
</section>

<section id="features">
  <div class="container">
    <div class="section-head center reveal">
      <p class="kicker">Everything a real book needs</p>
      <h2>Built for print, not just for screens.</h2>
      <p>Every feature below ships in the core binary &mdash; no plugins, no paid tier.</p>
    </div>
    <div class="feature-grid reveal">
      <div class="feature"><span class="fnum">01</span><h3>17 built-in themes</h3><p>From clean &amp; minimal to Gruvbox, LaTeX-style academic papers, and government letterhead.</p></div>
      <div class="feature"><span class="fnum">02</span><h3>PDF + EPUB, one source</h3><p>Build both formats from the same Markdown in a single pass &mdash; no separate pipeline to maintain.</p></div>
      <div class="feature"><span class="fnum">03</span><h3>Print-ready trim sizes</h3><p>6&times;9in, A5, or exact mm/in dimensions &mdash; the sizes real print-on-demand services expect.</p></div>
      <div class="feature"><span class="fnum">04</span><h3>Syntax highlighting</h3><p>Fenced code blocks are highlighted via Chroma, with a palette matched to your theme.</p></div>
      <div class="feature"><span class="fnum">05</span><h3>Auto TOC &amp; bookmarks</h3><p>Table of contents and PDF bookmarks are generated straight from your headings.</p></div>
      <div class="feature"><span class="fnum">06</span><h3>Automatic quality audit</h3><p>Catches overflow, broken images, low-contrast text, and clipped headings before you do.</p></div>
      <div class="feature"><span class="fnum">07</span><h3>Custom components</h3><p><code>&lt;DeepDive&gt;</code>, <code>&lt;Warning&gt;</code>, <code>&lt;Axiom&gt;</code> &mdash; or register your own in Go.</p></div>
      <div class="feature"><span class="fnum">08</span><h3>Live reload</h3><p><code>watch</code> and <code>serve</code> rebuild on save, with the browser refreshing instantly.</p></div>
      <div class="feature"><span class="fnum">09</span><h3>Write however you want</h3><p>Even a stray <code>.txt</code> file gets folded in &mdash; auto-numbered, auto-titled, zero setup.</p></div>
    </div>
  </div>
</section>

<section id="themes">
  <div class="container">
    <div class="section-head center reveal">
      <p class="kicker">Themes &mdash; <b>try one</b></p>
      <h2>17 palettes. Same Markdown.</h2>
      <p>This page is skinned with the exact same tokens as the real themes &mdash; click one, the whole site follows.</p>
    </div>
    <div class="themes-scroll reveal" id="themeGrid">
%s    </div>
    <p class="themes-hint">Currently previewing <b id="activeThemeLabel">%s</b> &mdash; this is exactly what your book's cover, headings, and code blocks would use. <a href="%s" id="themePdfLink" class="text-cta">Download this page's docs in this theme &rarr;</a></p>
  </div>
</section>

<section id="compare">
  <div class="container">
    <div class="section-head center reveal">
      <p class="kicker">Why not LaTeX?</p>
      <h2>Typesetting without the tradeoffs.</h2>
    </div>
    <div class="reveal" style="overflow-x:auto">
      <table class="compare">
        <thead>
          <tr><th>&nbsp;</th><th>LaTeX</th><th>Design app</th><th class="hl">pretty-pdf</th></tr>
        </thead>
        <tbody>
          <tr><td class="row-label">Plain text, git-diffable</td><td class="yes">Yes</td><td class="no">&mdash;</td><td class="hl">Yes</td></tr>
          <tr><td class="row-label">Runs headless in CI</td><td class="yes">Yes</td><td class="no">&mdash;</td><td class="hl">Yes</td></tr>
          <tr><td class="row-label">Learning curve</td><td class="no">Steep</td><td class="no">Software-specific</td><td class="hl">Just Markdown</td></tr>
          <tr><td class="row-label">Print-ready trim sizes</td><td class="yes">Manual</td><td class="yes">Manual</td><td class="hl">Built in</td></tr>
          <tr><td class="row-label">Free &amp; open source</td><td class="yes">Yes</td><td class="no">Usually not</td><td class="hl">MIT licensed</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</section>

<section class="footer-cta">
  <div class="container">
    <div class="reveal" style="display:flex;flex-direction:column;align-items:center">
      <p class="kicker">Get started</p>
      <h2>Ship your first PDF in the next five minutes.</h2>
      <p class="sub">Install the binary, scaffold a book, and build &mdash; that's the whole workflow.</p>
      <div class="cta-row" style="margin-bottom:0">
        <div class="install-line">
          <span class="prompt">$</span>
          <code>pretty-pdf init my-book</code>
          <button class="copy-btn" id="copyInit" aria-label="Copy init command">%s</button>
        </div>
        <a href="%s" class="text-cta" target="_blank" rel="noopener">View on GitHub &rarr;</a>
      </div>
    </div>
  </div>
</section>

<footer class="site-footer">
  <div class="container">
    <div class="footer-grid">
      <div class="footer-col footer-brand">
        <a href="index.html" class="brand">pretty<span class="dim">-pdf</span></a>
        <p>A Go library and CLI that turns Markdown into print-ready PDFs and EPUBs. MIT licensed.</p>
      </div>
      <div class="footer-col"><h4>Product</h4><a href="#themes">Themes</a><a href="#features">Features</a><a href="docs.html#changelog">Changelog</a></div>
      <div class="footer-col"><h4>Docs</h4><a href="docs.html#cli-reference">CLI reference</a><a href="docs.html#mdx-format">MDX format</a><a href="%s/blob/master/SECURITY.md">Security</a></div>
      <div class="footer-col"><h4>Community</h4><a href="%s">GitHub</a><a href="%s/issues">Issues</a><a href="%s/discussions">Discussions</a></div>
    </div>
    <div class="footer-bottom">
      <span>&copy; 2026 go-pretty-pdf &mdash; MIT License</span>
      <span>Built with Go, headless Chrome, and too many themes</span>
    </div>
  </div>
</footer>

<script>
%s
</script>
</body>
</html>`,
		landingDefaultTheme,
		siteTitle, siteDescription, siteKeywords, siteBaseURL,
		siteTitle, siteDescription, siteBaseURL, siteBaseURL,
		siteTitle, siteDescription, siteBaseURL,
		jsonLD(),
		landingCSS, landingThemeCSS(),
		themeSwatchSpans(mustTheme(landingDefaultTheme)), landingDefaultTheme, landingThemeDropdownOptions(),
		siteRepoURL,
		copyIconSVG(),
		landingThemeCards(), landingDefaultTheme, docsPDFFilename(landingDefaultTheme),
		copyIconSVG(),
		siteRepoURL,
		siteRepoURL,
		siteRepoURL, siteRepoURL, siteRepoURL,
		landingJS,
	)
}

func copyIconSVG() string {
	return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>`
}

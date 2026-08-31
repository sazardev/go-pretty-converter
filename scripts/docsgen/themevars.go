package main

import (
	"fmt"
	"strings"

	"github.com/sazardev/go-pretty-converter/theme"
)

// Names of the --pdf-*/--site-* custom properties this file reads and
// mirrors (only the prefix differs between the two).
const (
	varPrimary     = "primary"
	varAccent      = "accent"
	varText        = "text"
	varMuted       = "muted"
	varBg          = "bg"
	varSurface     = "surface"
	varSurface2    = "surface-2"
	varBorder      = "border"
	varInfo        = "info"
	varWarn        = "warn"
	varSuccess     = "success"
	varFontHeading = "font-heading"
	varFontBody    = "font-body"
	varFontCode    = "font-code"
	varLineHeight  = "line-height"
)

// siteThemeVarNames lists every --pdf-* custom property the docs site
// mirrors as --site-*. The names match 1:1 (only the prefix changes), so
// no theme-specific translation table is needed — new properties just need
// adding here once.
var siteThemeVarNames = []string{
	varPrimary, varAccent, varText, varMuted, varBg,
	varSurface, varSurface2, varBorder,
	varInfo, varWarn, varSuccess,
	varFontHeading, varFontBody, varFontCode,
}

// extractThemeVars reads every --pdf-* declaration out of a builtin
// theme's raw CSS (which is always exactly what ships in the actual PDF
// output). This is how the docs site derives its color/font palette
// instead of hand-copying hex codes that can drift from what the CLI
// actually renders — see the "azul incoherente" bug this replaced.
// (Shared with render.RenderToPDF, which uses the same parser to color
// the native PDF header/footer to match the page.)
func extractThemeVars(css string) map[string]string {
	return theme.ExtractCSSVars(css)
}

// themeCSSBlock renders the [data-site-theme="X"] { --site-*: ...; } block
// for one builtin theme, generated entirely from its real CSS so the site
// can never show a palette the actual PDF doesn't produce.
func themeCSSBlock(t theme.Theme) string {
	vars := extractThemeVars(t.CSS)

	var b strings.Builder
	// default is the site's theme (matches the server-rendered
	// data-site-theme="default" on <html>) AND the first theme in
	// theme.List(), so its combined ":root," selector is safe: the :root
	// fallback only matters if the attribute is ever missing, and every
	// other theme's [data-site-theme] block comes later in source order
	// and wins when selected. Earlier themes used classic here, whose block
	// sits mid-list — its unconditional :root rule then overrode the
	// default/minimal/modern palettes that precede it.
	if t.Name == theme.NameDefault {
		b.WriteString(":root,\n")
	}
	fmt.Fprintf(&b, "[data-site-theme=%q] {\n", t.Name)
	for _, name := range siteThemeVarNames {
		if v, ok := vars[name]; ok {
			fmt.Fprintf(&b, "  --site-%s: %s;\n", name, v)
		}
	}
	if lh, ok := vars[varLineHeight]; ok {
		fmt.Fprintf(&b, "  --site-line-height: %s;\n", lh)
	}

	flourish := "var(--site-text)"
	if t.Accented {
		flourish = "var(--site-accent)"
	}
	fmt.Fprintf(&b, "  --site-flourish: %s;\n", flourish)
	b.WriteString("}\n")
	return b.String()
}

// generatedThemeCSS builds the CSS variable blocks for every builtin theme
// in theme.List() order — the docs site's entire theme switcher palette,
// with zero hand-duplicated color/font data. Adding a new builtin theme to
// theme/builtin.go makes it show up here automatically on the next build.
func generatedThemeCSS() string {
	var b strings.Builder
	b.WriteString("/* Generated from theme.List() by scripts/docsgen — do not hand-edit.\n")
	b.WriteString(" * Each builtin theme's actual CSS is the single source of truth. */\n")
	for _, t := range theme.List() {
		b.WriteString(themeCSSBlock(t))
	}
	return b.String()
}

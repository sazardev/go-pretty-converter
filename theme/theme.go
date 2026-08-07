// Package theme implements go-pretty-pdf's theme engine: a set of
// professional built-in themes plus a customization layer (colors, fonts,
// section toggles, density) that composes on top of them via CSS custom
// properties, and a YAML format for user-defined custom themes.
package theme

import (
	"fmt"
	"regexp"
	"strings"
)

// cssDeclarationUnsafeRe matches characters that would let a color/font
// value (user-supplied via Options or a custom theme's YAML) escape its
// CSS declaration or the surrounding <style> block — e.g. a value like
// "red;} .cover{display:block" reopening a new rule after the :root block.
// Parens/quotes are left alone since legitimate values need them (rgb(...),
// "Georgia", serif).
var cssDeclarationUnsafeRe = regexp.MustCompile(`[;{}<>\\\r\n]`)

// cssURLUnsafeRe is used for values embedded inside a CSS url('...')
// (the Google Fonts @import), so — unlike cssDeclarationUnsafeRe — it also
// strips quotes and parens that would otherwise close the url() early.
var cssURLUnsafeRe = regexp.MustCompile(`[;{}<>'"()\\\r\n]`)

func sanitizeCSSDeclarationValue(s string) string {
	return cssDeclarationUnsafeRe.ReplaceAllString(s, "")
}

func sanitizeCSSURLValue(s string) string {
	return cssURLUnsafeRe.ReplaceAllString(s, "")
}

// Theme is a built-in (or synthetic) theme: a name plus the CSS that
// implements its palette/typography and any structural deltas on top of
// the shared base stylesheet.
type Theme struct {
	Name        string
	Description string
	Category    string
	CSS         string
	// Sections holds this theme's own section defaults (all true unless a
	// theme has a good reason to ship differently).
	Sections ResolvedSections
	// Accented marks themes that use their accent color as a bold,
	// structural design element (a border on the cover/heading, an
	// accent-colored blockquote, etc.) rather than reserving it for links
	// only. Consumers that want to echo a theme's visual identity outside
	// the PDF itself (e.g. the docs website's theme switcher) can read this
	// instead of guessing from the CSS or hardcoding a list.
	Accented bool
}

// Colors overrides the CSS custom properties a theme's palette is built
// from. Empty fields fall back to the theme's own defaults.
type Colors struct {
	Primary    string `yaml:"primary"`
	Accent     string `yaml:"accent"`
	Text       string `yaml:"text"`
	Muted      string `yaml:"muted"`
	Background string `yaml:"background"`
}

// Fonts overrides the font-family custom properties a theme uses. Empty
// fields fall back to the theme's own defaults. GoogleImports is only
// honored when Options.AllowNetworkFonts is true.
type Fonts struct {
	Heading       string   `yaml:"heading"`
	Body          string   `yaml:"body"`
	Code          string   `yaml:"code"`
	GoogleImports []string `yaml:"google_fonts"`
}

// Sections toggles document sections on or off. A nil pointer means "use
// the theme's default"; a non-nil pointer always wins.
type Sections struct {
	Cover       *bool `yaml:"cover"`
	TOC         *bool `yaml:"toc"`
	PageNumbers *bool `yaml:"page_numbers"`
	Header      *bool `yaml:"header"`
}

// ResolvedSections is Sections after defaults have been applied — every
// field has a concrete value.
type ResolvedSections struct {
	Cover       bool
	TOC         bool
	PageNumbers bool
	Header      bool
}

// Density adjusts overall spacing/line-height. The empty string means
// "normal" (the theme's own defaults, no adjustment).
type Density string

const (
	DensityCompact Density = "compact"
	DensityNormal  Density = "normal"
	DensityRelaxed Density = "relaxed"
)

// Options customizes a Theme at resolve time: colors, fonts, section
// toggles, density, and whether network-fetched (Google) fonts are allowed.
type Options struct {
	Colors            Colors
	Fonts             Fonts
	Sections          Sections
	Density           Density
	AllowNetworkFonts bool
}

// BoolPtr is a small helper for constructing Sections literals, e.g.
// theme.Sections{Cover: theme.BoolPtr(false)}.
func BoolPtr(b bool) *bool { return &b }

// Resolve builds the final CSS for theme t customized by opts, and returns
// the section toggles after defaults have been applied. The CSS is
// assembled as: optional Google Fonts @import, the shared base stylesheet,
// the theme's own CSS, a :root override block for any customized
// colors/fonts/density, and CSS for any disabled sections (cover/TOC).
// Page numbers and the running header are not CSS-controlled; callers wire
// ResolvedSections.PageNumbers/Header into render.Options.
func Resolve(t Theme, opts Options) (string, ResolvedSections, error) {
	sections := mergeSections(t.Sections, opts.Sections)

	var root []string
	appendVar := func(name, value string) {
		if value != "" {
			root = append(root, fmt.Sprintf("--pdf-%s: %s;", name, sanitizeCSSDeclarationValue(value)))
		}
	}
	appendVar("primary", opts.Colors.Primary)
	appendVar("accent", opts.Colors.Accent)
	appendVar("text", opts.Colors.Text)
	appendVar("muted", opts.Colors.Muted)
	appendVar("bg", opts.Colors.Background)
	appendVar("font-heading", opts.Fonts.Heading)
	appendVar("font-body", opts.Fonts.Body)
	appendVar("font-code", opts.Fonts.Code)

	switch opts.Density {
	case DensityCompact:
		root = append(root, "--pdf-line-height: 1.35;", "--pdf-space-scale: 0.7;")
	case DensityRelaxed:
		root = append(root, "--pdf-line-height: 1.85;", "--pdf-space-scale: 1.35;")
	case DensityNormal, "":
		// no override — base/theme defaults apply
	default:
		return "", ResolvedSections{}, fmt.Errorf("unknown density %q (expected compact, normal, or relaxed)", opts.Density)
	}

	var b strings.Builder
	if opts.AllowNetworkFonts {
		if imp := googleFontsImport(opts.Fonts.GoogleImports); imp != "" {
			b.WriteString(imp)
			b.WriteString("\n")
		}
	}
	b.WriteString(baseCSS)
	b.WriteString("\n")
	b.WriteString(t.CSS)
	b.WriteString("\n")
	b.WriteString(chromaCSSFor(t.Name, t.CSS))
	b.WriteString("\n")
	if len(root) > 0 {
		b.WriteString(":root{" + strings.Join(root, "") + "}\n")
	}
	if !sections.Cover {
		b.WriteString(".cover{display:none !important;}\n")
	}
	if !sections.TOC {
		b.WriteString(".toc{display:none !important;}\n")
	}

	return b.String(), sections, nil
}

func mergeSections(base ResolvedSections, override Sections) ResolvedSections {
	r := base
	if override.Cover != nil {
		r.Cover = *override.Cover
	}
	if override.TOC != nil {
		r.TOC = *override.TOC
	}
	if override.PageNumbers != nil {
		r.PageNumbers = *override.PageNumbers
	}
	if override.Header != nil {
		r.Header = *override.Header
	}
	return r
}

func googleFontsImport(families []string) string {
	if len(families) == 0 {
		return ""
	}
	cleaned := make([]string, 0, len(families))
	for _, f := range families {
		f = strings.TrimSpace(sanitizeCSSURLValue(f))
		if f == "" {
			continue
		}
		cleaned = append(cleaned, strings.ReplaceAll(f, " ", "+"))
	}
	if len(cleaned) == 0 {
		return ""
	}
	return fmt.Sprintf("@import url('https://fonts.googleapis.com/css?family=%s&display=swap');", strings.Join(cleaned, "|"))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

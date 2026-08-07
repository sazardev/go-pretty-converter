package theme

import (
	"math"
	"strconv"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
)

// ChromaClassPrefix is the CSS class prefix the mdx parser's syntax
// highlighter must use for code-block token spans, so the class names it
// emits line up with the stylesheet chromaCSSFor generates here.
const ChromaClassPrefix = "chroma-"

// chromaStyleByTheme pairs each dark builtin theme with a Chroma style of
// similar tone, so code blocks don't render as dark-on-dark or otherwise
// clash. Every other (light) theme shares "github", a clean neutral light
// style. This is a curated pairing, not a computed color match — it only
// needs to keep code blocks legible against each theme's background, not
// replicate the theme's exact palette.
var chromaStyleByTheme = map[string]string{
	NameGruvbox:   "gruvbox",
	NameDark:      "dracula",
	NameTerminal:  "monokai",
	NameBlueprint: "nord",
}

var chromaFormatter = chromahtml.New(
	chromahtml.WithClasses(true),
	chromahtml.ClassPrefix(ChromaClassPrefix),
)

// chromaCSSFor returns the stylesheet mapping code-block token classes to
// colors for the given theme, so fenced code blocks are syntax-highlighted
// consistently with the theme's overall tone.
//
// themeName picks a curated dark style for the builtin dark themes (see
// chromaStyleByTheme); anything else starts from the neutral light "github"
// style. For themes that don't declare a name in that map — most notably a
// raw --theme foo.css file, whose Name is just its path — the theme's own
// CSS is inspected: if it declares a dark --pdf-bg, a dark Chroma style is
// used instead of "github", so code blocks stay legible on a dark
// background rather than rendering as dark-on-dark.
func chromaCSSFor(themeName string, themeCSS string) string {
	styleName, ok := chromaStyleByTheme[themeName]
	if !ok {
		styleName = "github"
		if isDarkBackground(themeCSS) {
			styleName = "dracula"
		}
	}
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}

	var b strings.Builder
	// chromaFormatter.WithClasses(true), so WriteCSS never touches an
	// io.Writer that can fail; the error is always nil.
	_ = chromaFormatter.WriteCSS(&b, style)
	return b.String()
}

// isDarkBackground reports whether css declares a dark --pdf-bg page
// background (by relative luminance), so a theme's syntax-highlighting
// style can be picked to match instead of clashing.
func isDarkBackground(css string) bool {
	bg := ExtractCSSVars(css)["bg"]
	return isDarkColor(bg)
}

// isDarkColor reports whether a #rgb/#rrggbb color is dark enough that
// light-colored code tokens would be unreadable on it. Non-hex values
// (rgb(), names, empty) report false and leave the caller at its default.
func isDarkColor(s string) bool {
	s = strings.TrimSpace(strings.TrimPrefix(s, "#"))
	if len(s) == 3 {
		s = strings.Repeat(s[:1], 2) + strings.Repeat(s[1:2], 2) + strings.Repeat(s[2:3], 2)
	}
	if len(s) != 6 {
		return false
	}
	r, e1 := strconv.ParseUint(s[0:2], 16, 8)
	g, e2 := strconv.ParseUint(s[2:4], 16, 8)
	b, e3 := strconv.ParseUint(s[4:6], 16, 8)
	if e1 != nil || e2 != nil || e3 != nil {
		return false
	}
	lin := func(c uint64) float64 {
		x := float64(c) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	lum := 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
	return lum < 0.35
}

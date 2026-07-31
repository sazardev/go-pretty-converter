package theme

import (
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
// colors for the given builtin theme name, so fenced code blocks are
// syntax-highlighted consistently with the theme's overall tone.
func chromaCSSFor(themeName string) string {
	styleName, ok := chromaStyleByTheme[themeName]
	if !ok {
		styleName = "github"
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

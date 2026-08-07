package theme

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
)

//go:embed assets/epub-base.css
var epubBaseCSS string

// ResolveForEPUB builds the final CSS for theme t customized by opts,
// using the EPUB structural skeleton (relative units, no print-only rules)
// instead of the PDF base stylesheet. The CSS is assembled as: optional
// Google Fonts @import, the EPUB base stylesheet, the theme's own CSS,
// and a :root override block for any customized colors/fonts/density.
//
// Unlike Resolve, this does not return ResolvedSections — EPUB has its own
// cover-image mechanism and nav.xhtml for navigation, so the PDF section
// toggles (cover, TOC, page numbers, header) do not apply.
func ResolveForEPUB(t Theme, opts Options) (string, error) {
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
	default:
		return "", fmt.Errorf("unknown density %q (expected compact, normal, or relaxed)", opts.Density)
	}

	var b strings.Builder
	if opts.AllowNetworkFonts {
		if imp := googleFontsImport(opts.Fonts.GoogleImports); imp != "" {
			b.WriteString(imp)
			b.WriteString("\n")
		}
	}
	b.WriteString(epubBaseCSS)
	b.WriteString("\n")
	b.WriteString(t.CSS)
	b.WriteString("\n")
	b.WriteString(chromaCSSFor(t.Name, t.CSS))
	b.WriteString("\n")
	if len(root) > 0 {
		b.WriteString(":root{" + strings.Join(root, "") + "}\n")
	}

	return b.String(), nil
}

// ResolveByNameForEPUB resolves a theme by name (same dispatch as
// ResolveByName) and returns its EPUB-ready CSS.
func ResolveByNameForEPUB(name string, opts Options, cwd string) (string, error) {
	if name == "" {
		name = NameDefault
	}

	switch {
	case strings.HasSuffix(name, ".css"):
		return resolveRawCSSFileForEPUB(name, opts)
	case strings.HasSuffix(name, ThemeFileSuffix), strings.HasSuffix(name, ".theme.yaml"):
		ct, err := LoadCustomTheme(name)
		if err != nil {
			return "", err
		}
		return ct.ResolveForEPUB(opts)
	}

	if t, ok := Get(name); ok {
		return ResolveForEPUB(t, opts)
	}

	if path, ok := FindCustom(cwd, name); ok {
		ct, err := LoadCustomTheme(path)
		if err != nil {
			return "", err
		}
		return ct.ResolveForEPUB(opts)
	}

	return "", fmt.Errorf("unknown theme %q (not a builtin, custom theme, or file path)", name)
}

func resolveRawCSSFileForEPUB(path string, opts Options) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading theme CSS file %s: %w", path, err)
	}
	t := Theme{Name: path, CSS: string(data)}
	return ResolveForEPUB(t, opts)
}

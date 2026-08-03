package theme

import (
	"regexp"
	"strings"
	"testing"
)

func stripCSSComments(css string) string {
	return regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
}

func TestResolveForEPUBIncludesEpubBaseAndThemeCSS(t *testing.T) {
	th, ok := Get("corporate")
	if !ok {
		t.Fatal("expected corporate theme to be registered")
	}

	css, err := ResolveForEPUB(th, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(css, "EPUB output format") {
		t.Error("expected resolved CSS to include the EPUB base stylesheet")
	}
	if !strings.Contains(css, "--pdf-primary") {
		t.Error("expected resolved CSS to include the theme's own CSS variables")
	}
}

func TestResolveForEPUBOmitsPrintOnlyRules(t *testing.T) {
	th, _ := Get(NameDefault)

	css, err := ResolveForEPUB(th, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stripped := stripCSSComments(css)
	if strings.Contains(stripped, "@page") {
		t.Error("EPUB CSS should not contain @page rules")
	}
	if strings.Contains(stripped, "page-break") {
		t.Error("EPUB CSS should not contain page-break rules")
	}
	if strings.Contains(stripped, "break-before") {
		t.Error("EPUB CSS should not contain break-before rules")
	}
	if strings.Contains(stripped, "break-after") {
		t.Error("EPUB CSS should not contain break-after rules")
	}
	if strings.Contains(stripped, "orphans") {
		t.Error("EPUB CSS should not contain orphans rules")
	}
	if strings.Contains(stripped, "widows") {
		t.Error("EPUB CSS should not contain widows rules")
	}
}

func TestResolveForEPUBOmitsPDFOnlySections(t *testing.T) {
	th, _ := Get(NameDefault)

	css, err := ResolveForEPUB(th, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stripped := stripCSSComments(css)
	if strings.Contains(stripped, ".cover") {
		t.Error("EPUB CSS should not contain .cover rules")
	}
	if strings.Contains(stripped, ".toc") {
		t.Error("EPUB CSS should not contain .toc rules")
	}
	if strings.Contains(stripped, ".metadata") {
		t.Error("EPUB CSS should not contain .metadata rules")
	}
}

func TestResolveForEPUBUsesRelativeUnits(t *testing.T) {
	if !strings.Contains(epubBaseCSS, "em") {
		t.Error("epub-base.css should use em units for reflowable layout")
	}
	if strings.Contains(epubBaseCSS, "pt") {
		t.Error("epub-base.css should not use pt units")
	}
	if strings.Contains(epubBaseCSS, "mm") {
		t.Error("epub-base.css should not use mm units")
	}
}

func TestResolveForEPUBColorAndFontOverrides(t *testing.T) {
	th, _ := Get(NameDefault)

	css, err := ResolveForEPUB(th, Options{
		Colors: Colors{Primary: "#ff0000", Accent: "#00ff00"},
		Fonts:  Fonts{Heading: "Comic Sans MS"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(css, "--pdf-primary: #ff0000;") {
		t.Error("expected override root block to set --pdf-primary")
	}
	if !strings.Contains(css, "--pdf-accent: #00ff00;") {
		t.Error("expected override root block to set --pdf-accent")
	}
	if !strings.Contains(css, "--pdf-font-heading: Comic Sans MS;") {
		t.Error("expected override root block to set --pdf-font-heading")
	}
}

func TestResolveForEPUBDensity(t *testing.T) {
	th, _ := Get(NameDefault)

	css, err := ResolveForEPUB(th, Options{Density: DensityCompact})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(css, "--pdf-line-height: 1.35;") {
		t.Error("expected compact density to set --pdf-line-height")
	}
	if !strings.Contains(css, "--pdf-space-scale: 0.7;") {
		t.Error("expected compact density to set --pdf-space-scale")
	}

	css, err = ResolveForEPUB(th, Options{Density: DensityRelaxed})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(css, "--pdf-line-height: 1.85;") {
		t.Error("expected relaxed density to set --pdf-line-height")
	}

	if _, err := ResolveForEPUB(th, Options{Density: "extreme"}); err == nil {
		t.Error("expected an error for an unknown density value")
	}
}

func TestResolveForEPUBSanitizesCSSInjection(t *testing.T) {
	th, _ := Get(NameDefault)

	css, err := ResolveForEPUB(th, Options{
		Colors: Colors{Primary: "red;} .cover{display:block !important;} .x{color:red"},
		Fonts:  Fonts{Heading: "</style><script>alert(1)</script>"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(css, ".cover{display:block") || strings.Contains(css, "<script>") || strings.Contains(css, "</style>") {
		t.Errorf("expected CSS-breaking characters to be stripped from overrides, got: %s", css)
	}
}

func TestResolveByNameForEPUB(t *testing.T) {
	css, err := ResolveByNameForEPUB("default", Options{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(css, "EPUB output format") {
		t.Error("expected EPUB base CSS in resolved output")
	}

	css, err = ResolveByNameForEPUB("", Options{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(css, "EPUB output format") {
		t.Error("expected empty name to default to 'default' theme")
	}

	if _, err := ResolveByNameForEPUB("nonexistent-theme", Options{}, ""); err == nil {
		t.Error("expected error for unknown theme name")
	}
}

func TestResolveByNameForEPUBWithAllBuiltinThemes(t *testing.T) {
	for _, name := range order {
		t.Run(name, func(t *testing.T) {
			css, err := ResolveByNameForEPUB(name, Options{}, "")
			if err != nil {
				t.Fatalf("unexpected error for theme %q: %v", name, err)
			}
			if css == "" {
				t.Errorf("expected non-empty CSS for theme %q", name)
			}
			if !strings.Contains(css, "EPUB output format") {
				t.Errorf("expected EPUB base CSS for theme %q", name)
			}
		})
	}
}

func TestResolveForEPUBGoogleFontsImport(t *testing.T) {
	th, _ := Get(NameDefault)

	css, err := ResolveForEPUB(th, Options{
		Fonts:             Fonts{GoogleImports: []string{testGoogleFont}},
		AllowNetworkFonts: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(css, "fonts.googleapis.com") {
		t.Error("did not expect a Google Fonts @import without AllowNetworkFonts")
	}

	css, err = ResolveForEPUB(th, Options{
		Fonts:             Fonts{GoogleImports: []string{testGoogleFont}},
		AllowNetworkFonts: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(css, "@import url('https://fonts.googleapis.com/css?family=Inter:400,600&display=swap');") {
		t.Errorf("expected a Google Fonts @import, got: %.200s", css)
	}
}

func TestResolveForEPUBIncludesContentStyling(t *testing.T) {
	th, _ := Get(NameDefault)

	css, err := ResolveForEPUB(th, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requiredSelectors := []string{
		"component-deep-dive",
		"component-warning",
		"component-axiom",
		"blockquote",
		"table",
		"pre",
		"code",
	}
	for _, sel := range requiredSelectors {
		if !strings.Contains(css, sel) {
			t.Errorf("expected EPUB CSS to contain %q styling", sel)
		}
	}
}

package theme

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// testGoogleFont is a sample Options.Fonts.GoogleImports value shared by
// tests across this package's test files.
const testGoogleFont = "Inter:400,600"

func TestGetAndList(t *testing.T) {
	list := List()
	if len(list) != len(order) {
		t.Fatalf("expected %d builtin themes, got %d", len(order), len(list))
	}
	for i, name := range order {
		if list[i].Name != name {
			t.Errorf("List()[%d].Name = %q, want %q", i, list[i].Name, name)
		}
		if list[i].CSS == "" {
			t.Errorf("theme %q has empty CSS", name)
		}
	}

	if _, ok := Get("does-not-exist"); ok {
		t.Error("expected Get to report false for an unknown theme")
	}
	if t2, ok := Get(NameDark); !ok || t2.Name != NameDark {
		t.Errorf("expected Get(%q) to return the dark theme, got %+v, %v", NameDark, t2, ok)
	}
}

// TestBaseCSSPageRuleHasNoMargin guards a real regression: this Chromium
// version honors an @page { margin: ... } rule — even "margin: 0" — over
// whatever render.RenderToPDF's Page.printToPDF marginTop/Bottom/Left
// /Right requested, silently making custom margins (render.Options or
// go-pretty-pdf.yml's render.margin_*) have no effect at all. The
// imperative printToPDF margins must stay the single source of truth for
// page margins, which only holds if @page never declares one.
func TestBaseCSSPageRuleHasNoMargin(t *testing.T) {
	pageRule := regexpMustFind(t, `@page\s*{[^}]*}`, baseCSS)
	withoutComments := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(pageRule, "")
	if strings.Contains(withoutComments, "margin") {
		t.Errorf("base.css's @page rule must not declare margin (silently overrides Page.printToPDF's margin parameters): %s", pageRule)
	}
}

// TestBaseCSSH1HasTopMarginBuffer guards a real regression: chrome-headless-shell
// clips the first ~0.3in of any element flush against a forced page break
// (margin-top: 0) whenever header/footer templates are displayed — the
// glyphs render partly inside the unreliable margin/header strip and get
// sliced off, while the same heading renders perfectly with
// --no-header --no-page-numbers, or when it's not the first thing on a
// fresh page. h1 (every chapter title and ".toc h1") must keep a top
// margin comfortably larger than that dead zone so its text always clears
// it, on every theme (none of which override h1's margin-top).
func TestBaseCSSH1HasTopMarginBuffer(t *testing.T) {
	h1Rule := regexpMustFind(t, `(?:^|\n)h1\s*{[^}]*}`, baseCSS)
	m := regexp.MustCompile(`margin-top:\s*([\d.]+)(in|pt|mm)`).FindStringSubmatch(h1Rule)
	if m == nil {
		t.Fatalf("base.css's h1 rule must declare an explicit margin-top buffer, got: %s", h1Rule)
	}
	val, unit := m[1], m[2]
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		t.Fatalf("could not parse h1 margin-top value %q: %v", val, err)
	}
	var inches float64
	switch unit {
	case "in":
		inches = f
	case "pt":
		inches = f / 72
	case "mm":
		inches = f / 25.4
	}
	const minBufferIn = 0.3 // empirically measured dead-zone size; keep a safety margin above it
	if inches < minBufferIn {
		t.Errorf("h1's margin-top (%s%s = %.3fin) is smaller than the empirically measured header dead zone (%.2fin) — chapter titles will render clipped when a header/page-numbers are shown", val, unit, inches, minBufferIn)
	}
}

func regexpMustFind(t *testing.T, pattern, s string) string {
	t.Helper()
	re := regexp.MustCompile("(?s)" + pattern)
	m := re.FindString(s)
	if m == "" {
		t.Fatalf("pattern %q not found in base.css", pattern)
	}
	return m
}

func TestResolveIncludesBaseAndThemeCSS(t *testing.T) {
	th, ok := Get("corporate")
	if !ok {
		t.Fatal("expected corporate theme to be registered")
	}

	css, sections, err := Resolve(th, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(css, "Variable contract") {
		t.Error("expected resolved CSS to include the shared base stylesheet")
	}
	if !strings.Contains(css, "corporate") {
		t.Error("expected resolved CSS to include the theme's own CSS")
	}
	if !sections.Cover || !sections.TOC || !sections.PageNumbers || !sections.Header {
		t.Errorf("expected all sections on by default, got %+v", sections)
	}
}

func TestResolveColorAndFontOverrides(t *testing.T) {
	th, _ := Get(NameDefault)

	css, _, err := Resolve(th, Options{
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

func TestResolveSanitizesCSSInjectionInColorAndFontOverrides(t *testing.T) {
	th, _ := Get(NameDefault)

	css, _, err := Resolve(th, Options{
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

func TestGoogleFontsImportSanitizesInjection(t *testing.T) {
	imp := googleFontsImport([]string{`Evil');}</style><script>alert(1)</script`})
	if strings.Contains(imp, "');}") || strings.Contains(imp, "<script>") || strings.Contains(imp, "</style>") {
		t.Errorf("expected google fonts import to strip URL-breaking characters, got: %s", imp)
	}
}

func TestResolveSectionToggles(t *testing.T) {
	th, _ := Get(NameDefault)

	css, sections, err := Resolve(th, Options{
		Sections: Sections{Cover: BoolPtr(false), Header: BoolPtr(false)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sections.Cover {
		t.Error("expected Cover to be disabled")
	}
	if sections.TOC != true {
		t.Error("expected TOC to remain at its default (true)")
	}
	if sections.Header {
		t.Error("expected Header to be disabled")
	}
	if !strings.Contains(css, `.cover{display:none !important;}`) {
		t.Error("expected CSS to hide the cover when disabled")
	}
	if strings.Contains(css, `.toc{display:none !important;}`) {
		t.Error("did not expect CSS to hide the TOC")
	}
}

func TestResolveInvalidDensity(t *testing.T) {
	th, _ := Get(NameDefault)
	if _, _, err := Resolve(th, Options{Density: "extreme"}); err == nil {
		t.Error("expected an error for an unknown density value")
	}
}

func TestResolveGoogleFontsImportOnlyWhenAllowed(t *testing.T) {
	th, _ := Get(NameDefault)

	css, _, err := Resolve(th, Options{
		Fonts:             Fonts{GoogleImports: []string{testGoogleFont}},
		AllowNetworkFonts: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(css, "fonts.googleapis.com") {
		t.Error("did not expect a Google Fonts @import without AllowNetworkFonts")
	}

	css, _, err = Resolve(th, Options{
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

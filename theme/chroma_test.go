package theme

import (
	"strings"
	"testing"
)

func TestChromaCSSForKnownThemes(t *testing.T) {
	for _, name := range order {
		css := chromaCSSFor(name, "")
		if !strings.Contains(css, ".chroma-chroma") {
			t.Errorf("theme %q: expected chroma CSS to contain a .chroma-chroma rule, got: %s", name, css)
		}
	}
}

func TestChromaCSSForUnknownThemeFallsBackToGithub(t *testing.T) {
	got := chromaCSSFor("some-custom-theme-name", "")
	want := chromaCSSFor(NameDefault, "") // default isn't in chromaStyleByTheme, so it also uses the "github" fallback
	if got != want {
		t.Errorf("expected unknown theme name to fall back to the same CSS as an unmapped builtin theme")
	}
}

func TestChromaCSSForDarkRawCSSUsesDarkStyle(t *testing.T) {
	dark := chromaCSSFor("path/to/custom.css", "/* custom */\n:root{--pdf-bg: #1e1e1e;}\n")
	light := chromaCSSFor("path/to/custom.css", "/* custom */\n:root{--pdf-bg: #ffffff;}\n")
	if dark == light {
		t.Error("expected a dark --pdf-bg to select a different (dark) Chroma style than a light one")
	}
	// The light fallback must equal the default "github" output.
	if light != chromaCSSFor(NameDefault, "") {
		t.Error("expected a light --pdf-bg to use the same CSS as the default github fallback")
	}
}

func TestIsDarkColor(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"#ffffff", false},
		{"#000000", true},
		{"#1e1e1e", true},
		{"#f0f0f0", false},
		{"#111", true}, // 3-digit form is expanded then evaluated
		{"notacolor", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isDarkColor(tt.in); got != tt.want {
			t.Errorf("isDarkColor(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestResolveIncludesChromaCSS(t *testing.T) {
	dark, _ := Get(NameDark)
	css, _, err := Resolve(dark, Options{})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if !strings.Contains(css, ".chroma-chroma") {
		t.Error("expected Resolve's CSS to include the Chroma stylesheet")
	}
}

func TestResolveForEPUBIncludesChromaCSS(t *testing.T) {
	gruvbox, _ := Get(NameGruvbox)
	css, err := ResolveForEPUB(gruvbox, Options{})
	if err != nil {
		t.Fatalf("ResolveForEPUB returned error: %v", err)
	}
	if !strings.Contains(css, ".chroma-chroma") {
		t.Error("expected ResolveForEPUB's CSS to include the Chroma stylesheet")
	}
}

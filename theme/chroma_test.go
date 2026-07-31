package theme

import (
	"strings"
	"testing"
)

func TestChromaCSSForKnownThemes(t *testing.T) {
	for _, name := range order {
		css := chromaCSSFor(name)
		if !strings.Contains(css, ".chroma-chroma") {
			t.Errorf("theme %q: expected chroma CSS to contain a .chroma-chroma rule, got: %s", name, css)
		}
	}
}

func TestChromaCSSForUnknownThemeFallsBackToGithub(t *testing.T) {
	got := chromaCSSFor("some-custom-theme-name")
	want := chromaCSSFor(NameDefault) // default isn't in chromaStyleByTheme, so it also uses the "github" fallback
	if got != want {
		t.Errorf("expected unknown theme name to fall back to the same CSS as an unmapped builtin theme")
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

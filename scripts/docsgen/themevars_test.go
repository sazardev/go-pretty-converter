package main

import (
	"strings"
	"testing"

	"github.com/sazardev/go-pretty-converter/theme"
)

func TestExtractThemeVars(t *testing.T) {
	css := `:root {
  --pdf-primary: #1c1c1c;
  --pdf-accent: #7a4a2b;
  --pdf-font-heading: 'Georgia', 'Iowan Old Style', serif;
}
.cover h1 { border-bottom: 2px solid var(--pdf-accent, #7a4a2b); }
`
	got := extractThemeVars(css)

	want := map[string]string{
		varPrimary:     "#1c1c1c",
		varAccent:      "#7a4a2b",
		varFontHeading: "'Georgia', 'Iowan Old Style', serif",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("extractThemeVars()[%q] = %q, want %q", k, got[k], v)
		}
	}
	// A var(--pdf-accent, ...) *usage* must never be mistaken for a
	// declaration — it has no trailing "-usage" key and must not silently
	// override the real "accent" declaration above.
	if len(got) != len(want) {
		t.Errorf("extractThemeVars() found %d vars, want exactly %d: %v", len(got), len(want), got)
	}
}

// TestBuiltinThemesProduceSiteVars is the regression test for the exact
// bug this file's approach replaced: every builtin theme's CSS must yield
// a usable --site-primary/--site-accent/--site-bg/--site-font-body, or the
// docs site would silently fall back to no color/font at all for it.
func TestBuiltinThemesProduceSiteVars(t *testing.T) {
	required := []string{varPrimary, varAccent, varText, varMuted, varBg, varFontHeading, varFontBody, varFontCode}

	for _, th := range theme.List() {
		vars := extractThemeVars(th.CSS)
		for _, name := range required {
			if vars[name] == "" {
				t.Errorf("theme %q: missing --pdf-%s (site would render with no fallback)", th.Name, name)
			}
		}
	}
}

func TestThemeCSSBlock(t *testing.T) {
	def, ok := theme.Get(theme.NameDefault)
	if !ok {
		t.Fatal("theme.Get(default) not found")
	}
	defBlock := themeCSSBlock(def)

	// default is both the site's initial theme and the first entry in
	// theme.List(), so its combined :root rule must be emitted FIRST —
	// otherwise the unconditional :root selector (same specificity as
	// [data-site-theme="default"]) would override the default palette.
	if !strings.HasPrefix(defBlock, ":root,\n") {
		t.Error("default block should double as the :root fallback (and be first in list order)")
	}
	if !strings.Contains(defBlock, "--site-flourish: var(--site-text);") {
		t.Error("default is not Accented, flourish should use the neutral text color")
	}

	classic, _ := theme.Get(theme.NameClassic)
	classicBlock := themeCSSBlock(classic)
	if strings.HasPrefix(classicBlock, ":root") {
		t.Error("only default should carry the :root fallback; classic must not, or its mid-list block overrides default/minimal/modern")
	}
	if !strings.Contains(classicBlock, `--site-accent: #7a4a2b;`) {
		t.Errorf("classic block missing expected accent value: %s", classicBlock)
	}
	if !strings.Contains(classicBlock, "--site-flourish: var(--site-accent);") {
		t.Error("classic is Accented, flourish should use the accent color")
	}
}

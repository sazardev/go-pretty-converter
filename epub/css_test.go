package epub

import (
	"strings"
	"testing"
)

func TestSanitizeEPUBCSSResolvesVarsFromRoot(t *testing.T) {
	css := `:root{--pdf-space-scale: 1.35;--pdf-line-height: 1.85;}
p {
  margin: calc(0.7em * var(--pdf-space-scale, 1)) 0;
  line-height: var(--pdf-line-height, 1.6);
  font-family: var(--pdf-font-body, 'Inter', sans-serif);
}`
	got := sanitizeEPUBCSS(css)
	for _, want := range []string{"margin: 0.945em 0;", "line-height: 1.85;", "font-family: 'Inter', sans-serif;"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected sanitized CSS to contain %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "var(") || strings.Contains(got, "calc(") {
		t.Errorf("expected no var()/calc() left in sanitized CSS, got:\n%s", got)
	}
}

func TestSanitizeEPUBCSSUsesFallbackWithoutRoot(t *testing.T) {
	css := `p { margin: calc(0.7em * var(--pdf-space-scale, 1)) 0; color: var(--pdf-text, #1a1a1a); }`
	got := sanitizeEPUBCSS(css)
	for _, want := range []string{"margin: 0.7em 0;", "color: #1a1a1a;"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected fallback to be applied without a :root definition, got:\n%s", got)
		}
	}
}

func TestSanitizeEPUBCSSStripsFlexGrid(t *testing.T) {
	css := `.chroma-chroma .chroma-line { display: flex; }
.foo { display: inline-grid; }
.bar { display: block; }
.baz { display: none; }`
	got := sanitizeEPUBCSS(css)
	if strings.Contains(got, "display: flex") || strings.Contains(got, "display: inline-grid") {
		t.Errorf("expected flex/grid declarations removed, got:\n%s", got)
	}
	for _, want := range []string{"display: block;", "display: none;"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q to survive sanitization, got:\n%s", want, got)
		}
	}
}

func TestSanitizeEPUBCSSKeepsUnrelatedCalc(t *testing.T) {
	css := `.exotic { width: calc(100% - 2em); }`
	got := sanitizeEPUBCSS(css)
	if !strings.Contains(got, "calc(100% - 2em)") {
		t.Errorf("expected unrelated calc() to be left untouched, got:\n%s", got)
	}
}

func TestSanitizeEPUBCSSMultReverseOrder(t *testing.T) {
	css := `.x { margin: calc(0.75 * 2em); }`
	got := sanitizeEPUBCSS(css)
	if !strings.Contains(got, "1.5em") {
		t.Errorf("expected reversed <number> * <length> calc to compute, got:\n%s", got)
	}
}

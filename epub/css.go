package epub

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// sanitizeEPUBCSS rewrites a stylesheet so it survives conversion to
// formats whose renderers are far less capable than Chrome (EPUB readers,
// and especially Calibre's MOBI/AZW3 pipeline and old Kindle firmware):
//
//  1. CSS custom properties (var(--pdf-*)) are resolved to their concrete
//     values. Kindle's KF8 renderer does not support var() at all, so a
//     rule relying on it is silently dropped — and when that happens to
//     margins, padding, or line-height, adjacent text runs collide and
//     Kindle's reflow splices sentences mid-word. Values come from any
//     :root{...} block in the sheet (the theme system appends one), else
//     from each var() call's own fallback.
//
//  2. calc() expressions of the form "<length> * <number>" (the only form
//     the EPUB base stylesheet uses, always driven by the space-scale
//     multiplier) are computed to a plain length — calc() itself predates
//     most e-ink readers and is equally unreliable there.
//
//  3. display: flex / grid declarations are removed. flex is used by the
//     Chroma line rule (`.chroma-line { display: flex; }`) so every code
//     line background spans its container in the PDF; on Kindle-class
//     renderers unsupported display values collapse the code block into
//     garbled, overlapping text. EPUB output is intentionally flex-free.
func sanitizeEPUBCSS(css string) string {
	vars := extractCSSVars(css)
	css = resolveCSSVarRefs(css, vars)
	css = resolveCalcMult(css)
	css = stripFlexGridDeclarations(css)
	return css
}

var cssVarDefRe = regexp.MustCompile(`--pdf-[a-z0-9-]+\s*:\s*[^;]+;`)

// extractCSSVars collects every `--pdf-*: value;` declaration from
// :root blocks anywhere in the sheet. Plain-string matching (no full CSS
// parser) is fine here: the declaration surface is the theme system's
// own `:root{...}` line plus the base stylesheets, all generated with
// exactly this syntax.
func extractCSSVars(css string) map[string]string {
	out := make(map[string]string)
	for _, m := range cssVarDefRe.FindAllString(css, -1) {
		colon := strings.IndexByte(m, ':')
		name := strings.TrimSpace(m[:colon])
		value := strings.TrimSpace(m[colon+1 : len(m)-1])
		if strings.HasPrefix(name, "--pdf-") && value != "" {
			out[name] = value
		}
	}
	return out
}

var cssVarRefRe = regexp.MustCompile(`var\(\s*--pdf-[a-z0-9-]+\s*(?:,[^)]*)?\)`)

// var() resolves against the definition map, falling back to the
// call's own fallback value when the property isn't defined anywhere.
// Fallbacks are returned verbatim (quotes included): var() substitution
// is raw text substitution, so `font-family: 'Inter', sans-serif;` is
// already valid CSS and quote-stripping would only corrupt it.
func resolveCSSVarRefs(css string, vars map[string]string) string {
	return cssVarRefRe.ReplaceAllStringFunc(css, func(ref string) string {
		inner := strings.TrimSuffix(strings.TrimPrefix(ref, "var("), ")")
		parts := strings.SplitN(inner, ",", 2)
		name := strings.TrimSpace(parts[0])
		if v, ok := vars[name]; ok {
			return v
		}
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
		return ref
	})
}

// calcLenNumRe matches the two multiplication shapes the EPUB base
// stylesheets emit — "<length> * <number>" and "<number> * <length>".
// It is deliberately anchored on calc( ... ) with no ^ or $ at the ends,
// because these expressions appear mid-declaration:
// `margin: calc(0.7em * 1.35) 0;`. The strict grammar (length unit
// adjacent to the multiplier, no operators other than `*`) is what keeps
// unrelated calc() expressions — `calc(100% - 2em)` — from matching.
var calcLenNumRe = regexp.MustCompile(
	`calc\(\s*(?:` +
		// group 1 length, group 2 unit, group 3 number  (0.7em * 1.35)
		`([0-9]*\.?[0-9]+)(em|rem|px|pt|ex|ch|%|cm|mm|in)\s*\*\s*([0-9]*\.?[0-9]+)(?:\s*)\)` +
		`|` +
		// group 4 number, group 5 length, group 6 unit (0.75 * 2em)
		`([0-9]*\.?[0-9]+)\s*\*\s*([0-9]*\.?[0-9]+)(em|rem|px|pt|ex|ch|%|cm|mm|in)(?:\s*)\)` +
		`)`,
)

// resolveCalcMult computes the only calc() shapes the EPUB base
// stylesheets emit — "<length> * <number>" and "<number> * <length>"
// (the latter driven by the space-scale multiplier in either order) —
// leaving any other calc() expression untouched: either it isn't one of
// ours or a custom theme reached for something rarer, and in both cases
// guessing is worse than leaving it for the reader to attempt.
func resolveCalcMult(css string) string {
	return calcLenNumRe.ReplaceAllStringFunc(css, func(expr string) string {
		m := calcLenNumRe.FindStringSubmatch(expr)
		var left, right, unit string
		switch {
		case m[1] != "": // length * number
			left, unit, right = m[1], m[2], m[3]
		case m[4] != "": // number * length
			left, right, unit = m[4], m[5], m[6]
		default:
			return expr
		}
		l, err1 := strconv.ParseFloat(left, 64)
		r, err2 := strconv.ParseFloat(right, 64)
		if err1 != nil || err2 != nil || unit == "" {
			return expr
		}
		// Round to 4 decimals first (0.7 * 1.35 = 0.94499... in binary
		// floating point) and format with -1 precision so the result is
		// clean CSS: "0.945em", "1.2em", never "0.9450000000000001em".
		product := math.Round(l*r*10000) / 10000
		return strconv.FormatFloat(product, 'f', -1, 64) + unit
	})
}

var flexGridRe = regexp.MustCompile(`(?i)\bdisplay\s*:\s*(?:inline-)?(?:flex|grid)\s*;`)

func stripFlexGridDeclarations(css string) string {
	return flexGridRe.ReplaceAllString(css, "")
}

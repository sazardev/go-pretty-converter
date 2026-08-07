package config

import (
	"regexp"
	"strconv"
	"strings"
)

var cssUnitRe = regexp.MustCompile(`^(-?[0-9]*\.?[0-9]+)\s*([a-zA-Z]+)$`)

var customPaperRe = regexp.MustCompile(`^(-?[0-9]*\.?[0-9]+)\s*([a-zA-Z]*)\s*x\s*(-?[0-9]*\.?[0-9]+)\s*([a-zA-Z]*)$`)

// PaperLetter is the config value for US Letter paper size.
const PaperLetter = "letter"

// ParsePaperSize maps a named paper size to its width/height in inches.
// Named sizes: "letter", "legal", "a4" (case-insensitive).
// Custom dimensions: "6x9", "6x9in", "6in x 9in", "152.4mm x 228.6mm".
// Bare numbers default to inches. ok is false for unrecognized input.
func ParsePaperSize(name string) (width, height float64, ok bool) {
	switch strings.ToLower(name) {
	case PaperLetter:
		return 8.5, 11, true
	case "legal":
		return 8.5, 14, true
	case "a4":
		return 8.27, 11.69, true
	}

	m := customPaperRe.FindStringSubmatch(name)
	if m == nil {
		return 0, 0, false
	}

	wStr, wUnit := m[1], m[2]
	hStr, hUnit := m[3], m[4]

	if wUnit == "" {
		wUnit = "in"
	}
	if hUnit == "" {
		hUnit = "in"
	}

	w := ParseCSSUnit(wStr + wUnit)
	h := ParseCSSUnit(hStr + hUnit)
	if w <= 0 || h <= 0 {
		return 0, 0, false
	}

	return w, h, true
}

// ParseCSSUnit converts a CSS length string (e.g. "20mm", "0.8in") to
// inches. It returns 0 for empty or unrecognized input.
func ParseCSSUnit(s string) float64 {
	v, _ := ParseCSSUnitStrict(s)
	return v
}

// ParseCSSUnitStrict is ParseCSSUnit with a separate ok result so callers
// can distinguish "0in", a legitimate value, from an unrecognized/empty
// input that ParseCSSUnit reports as 0 too.
func ParseCSSUnitStrict(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}

	m := cssUnitRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}

	switch strings.ToLower(m[2]) {
	case "in":
		return value, true
	case "mm":
		return value / 25.4, true
	case "cm":
		return value / 2.54, true
	case "pt":
		return value / 72.0, true
	case "px":
		return value / 96.0, true
	default:
		return 0, false
	}
}

package config

import "testing"

func TestParsePaperSize(t *testing.T) {
	tests := []struct {
		name   string
		wantW  float64
		wantH  float64
		wantOK bool
	}{
		{PaperLetter, 8.5, 11, true},
		{"Legal", 8.5, 14, true},
		{"A4", 8.27, 11.69, true},
		{"6x9", 6, 9, true},
		{"6x9in", 6, 9, true},
		{"6in x 9in", 6, 9, true},
		{"6.0x9.0", 6, 9, true},
		{"5.5x8.5", 5.5, 8.5, true},
		{"152.4mm x 228.6mm", 6, 9, true},
		{"tabloid", 0, 0, false},
		{"", 0, 0, false},
		{"6x", 0, 0, false},
		{"6x9x12", 0, 0, false},
		{"abc", 0, 0, false},
	}
	for _, tt := range tests {
		w, h, ok := ParsePaperSize(tt.name)
		if ok != tt.wantOK {
			t.Errorf("ParsePaperSize(%q) ok = %v, want %v", tt.name, ok, tt.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if diff := w - tt.wantW; diff < -0.01 || diff > 0.01 {
			t.Errorf("ParsePaperSize(%q) width = %v, want %v", tt.name, w, tt.wantW)
		}
		if diff := h - tt.wantH; diff < -0.01 || diff > 0.01 {
			t.Errorf("ParsePaperSize(%q) height = %v, want %v", tt.name, h, tt.wantH)
		}
	}
}

func TestParseCSSUnit(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"", 0},
		{"1in", 1},
		{"25.4mm", 1},
		{"2.54cm", 1},
		{"72pt", 1},
		{"96px", 1},
		{"10bogus", 0},
	}
	for _, tt := range tests {
		got := ParseCSSUnit(tt.in)
		diff := got - tt.want
		if diff < -0.001 || diff > 0.001 {
			t.Errorf("ParseCSSUnit(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseCSSUnitStrict(t *testing.T) {
	tests := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"", 0, false},
		{"1in", 1, true},
		{"25.4mm", 1, true},
		{"0mm", 0, true}, // explicit zero is a valid, meaningful choice
		{"96px", 1, true},
		{"10bogus", 0, false},
		{"10", 0, false},   // bare number, no unit
		{"10em", 0, false}, // em isn't an absolute print unit
		{"not a length", 0, false},
	}
	for _, tt := range tests {
		got, ok := ParseCSSUnitStrict(tt.in)
		if ok != tt.ok {
			t.Errorf("ParseCSSUnitStrict(%q) ok = %v, want %v", tt.in, ok, tt.ok)
			continue
		}
		diff := got - tt.want
		if diff < -0.001 || diff > 0.001 {
			t.Errorf("ParseCSSUnitStrict(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

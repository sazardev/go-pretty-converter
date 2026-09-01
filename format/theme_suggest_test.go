package format

import "testing"

func TestSuggestThemeResume(t *testing.T) {
	raw := "Jane Doe\n\njane.doe@example.com\n\nSkills\n\nGo, Python, Leadership.\n\nExperience\n\nSenior Engineer at Acme, 2020-2026."
	report, err := Convert([]string{writeTempTxt(t, raw)}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if report.SuggestedCategory != categoryResume {
		t.Errorf("expected category %q, got %q (theme %q)", categoryResume, report.SuggestedCategory, report.SuggestedTheme)
	}
}

func TestSuggestThemeTechnical(t *testing.T) {
	body := "Overview\n\nSome intro text.\n\n"
	for i := 0; i < 6; i++ {
		body += "    func example() {\n        return 1\n    }\n\n"
	}
	report, err := Convert([]string{writeTempTxt(t, body)}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if report.SuggestedCategory != categoryTechnical {
		t.Errorf("expected category %q, got %q", categoryTechnical, report.SuggestedCategory)
	}
}

func TestSuggestThemeAcademic(t *testing.T) {
	raw := "Study Title\n\nAbstract\n\nThis paper examines something important."
	report, err := Convert([]string{writeTempTxt(t, raw)}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if report.SuggestedCategory != categoryAcademic {
		t.Errorf("expected category %q, got %q", categoryAcademic, report.SuggestedCategory)
	}
}

func TestSuggestThemeNoSignal(t *testing.T) {
	raw := "Notes\n\nJust a short, ordinary note with nothing distinctive about it."
	report, err := Convert([]string{writeTempTxt(t, raw)}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if report.SuggestedTheme != "" || report.SuggestedCategory != "" {
		t.Errorf("expected no suggestion, got theme %q category %q", report.SuggestedTheme, report.SuggestedCategory)
	}
}

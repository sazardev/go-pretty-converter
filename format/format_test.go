package format

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Shared fixture literals, reused across this package's *_test.go files.
const (
	titleIntroduction = "Introduction"
	titleOverview     = "Overview"
	id1               = "[1.0.0]"
)

// writeTempTxt writes raw to a new .txt file in a fresh t.TempDir() and
// returns its path — the shared fixture helper for Convert-level tests
// across this package's test files.
func writeTempTxt(t *testing.T, raw string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConvertSingleChapter(t *testing.T) {
	raw := "Introduction\n\nThis is the body of the chapter."
	report, err := Convert([]string{writeTempTxt(t, raw)}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(report.Documents))
	}
	doc := report.Documents[0]
	if doc.ID != id1 {
		t.Errorf("expected id %s, got %q", id1, doc.ID)
	}
	if doc.Title != titleIntroduction {
		t.Errorf("expected title %q, got %q", titleIntroduction, doc.Title)
	}
	if doc.Filename != "01-introduction.mdx" {
		t.Errorf("expected filename %q, got %q", "01-introduction.mdx", doc.Filename)
	}
	if !strings.Contains(doc.Content, id1) {
		t.Errorf("expected id in frontmatter, got:\n%s", doc.Content)
	}
}

func TestConvertMultipleChaptersSequentialIDs(t *testing.T) {
	raw := "Introduction\n\nFirst.\n\nGetting Started\n\nSecond."
	report, err := Convert([]string{writeTempTxt(t, raw)}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Documents) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(report.Documents))
	}
	if report.Documents[0].ID != id1 || report.Documents[1].ID != "[2.0.0]" {
		t.Errorf("expected sequential ids, got %q, %q", report.Documents[0].ID, report.Documents[1].ID)
	}
}

func TestConvertColdStartUsesFilenameTitle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-notes.txt")
	if err := os.WriteFile(path, []byte("Just a paragraph with no heading."), 0644); err != nil {
		t.Fatal(err)
	}
	report, err := Convert([]string{path}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(report.Documents))
	}
	if report.Documents[0].Title != "My Notes" {
		t.Errorf("expected filename-derived title %q, got %q", "My Notes", report.Documents[0].Title)
	}
}

func TestConvertDuplicateTitlesGetUniqueSlugs(t *testing.T) {
	raw := "Overview\n\nFirst.\n\nOverview\n\nSecond."
	report, err := Convert([]string{writeTempTxt(t, raw)}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Documents) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(report.Documents))
	}
	if report.Documents[0].Filename == report.Documents[1].Filename {
		t.Errorf("expected unique filenames for duplicate titles, both were %q", report.Documents[0].Filename)
	}
}

func TestConvertStartID(t *testing.T) {
	report, err := Convert([]string{writeTempTxt(t, "Introduction\n\nBody.")}, Options{StartID: 5})
	if err != nil {
		t.Fatal(err)
	}
	if report.Documents[0].ID != "[5.0.0]" {
		t.Errorf("expected id [5.0.0], got %q", report.Documents[0].ID)
	}
}

func TestSlugify(t *testing.T) {
	tests := map[string]string{
		"Getting Started!":  "getting-started",
		"":                  defaultSlug,
		"---":               defaultSlug,
		"Multiple   Spaces": "multiple-spaces",
	}
	for in, want := range tests {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

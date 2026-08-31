package analyze

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sazardev/go-pretty-pdf/mdx"
)

func mustParseDoc(t *testing.T, dir, filename, id, title, body string) *mdx.Document {
	t.Helper()
	content := "---\nid: \"" + id + "\"\ntitle: " + title + "\n---\n\n" + body + "\n"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := mdx.NewParser().ParseFile(path)
	if err != nil {
		t.Fatalf("parsing fixture %s: %v", filename, err)
	}
	return doc
}

func writePNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func findIssue(issues []Issue, check string) *Issue {
	for i := range issues {
		if issues[i].Check == check {
			return &issues[i]
		}
	}
	return nil
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.MaxTableColumns != 6 {
		t.Errorf("MaxTableColumns = %d, want 6", opts.MaxTableColumns)
	}
	if opts.MaxCodeLineLength != 100 {
		t.Errorf("MaxCodeLineLength = %d, want 100", opts.MaxCodeLineLength)
	}
	if opts.MaxListDepth != 3 {
		t.Errorf("MaxListDepth = %d, want 3", opts.MaxListDepth)
	}
	if opts.LongChapterWords != 3000 {
		t.Errorf("LongChapterWords = %d, want 3000", opts.LongChapterWords)
	}
	if opts.OversizedImageWidth != 3000 {
		t.Errorf("OversizedImageWidth = %d, want 3000", opts.OversizedImageWidth)
	}
}

func TestHeadingLevelSkip(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "# One\n\n### Three\n")

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	if findIssue(issues, "heading-level-skip") == nil {
		t.Fatalf("expected heading-level-skip, got %+v", issues)
	}
}

func TestNoHeadingLevelSkipForCorrectOrder(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "# One\n\n## Two\n\n### Three\n")

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	if findIssue(issues, "heading-level-skip") != nil {
		t.Fatalf("did not expect heading-level-skip, got %+v", issues)
	}
}

func TestMultipleH1(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "# One\n\nBody text.\n\n# Two\n\nBody text.\n")

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	if findIssue(issues, "multiple-h1") == nil {
		t.Fatalf("expected multiple-h1, got %+v", issues)
	}
}

func TestNoHeadings(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "Just a paragraph with some words in it, no headings at all.\n")

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	iss := findIssue(issues, "no-headings")
	if iss == nil {
		t.Fatalf("expected no-headings, got %+v", issues)
	}
	if iss.Severity != SeverityInfo {
		t.Errorf("no-headings severity = %q, want info", iss.Severity)
	}
}

func TestNoHeadingsSkippedForEmptyDoc(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "")

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	if findIssue(issues, "no-headings") != nil {
		t.Fatalf("did not expect no-headings for an empty document, got %+v", issues)
	}
}

func TestBrokenInternalAnchor(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "[broken](#missing-id)\n\nSome text.\n")

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	iss := findIssue(issues, "broken-internal-anchor")
	if iss == nil {
		t.Fatalf("expected broken-internal-anchor, got %+v", issues)
	}
	if iss.Severity != SeverityError {
		t.Errorf("broken-internal-anchor severity = %q, want error", iss.Severity)
	}
}

func TestValidInternalAnchorNotFlagged(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "# Target\n\n[link](#target)\n")

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	if findIssue(issues, "broken-internal-anchor") != nil {
		t.Fatalf("did not expect broken-internal-anchor, got %+v", issues)
	}
}

func TestDuplicateElementID(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "<div id=\"dup\">A</div>\n\n<div id=\"dup\">B</div>\n")

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	iss := findIssue(issues, "duplicate-element-id")
	if iss == nil {
		t.Fatalf("expected duplicate-element-id, got %+v", issues)
	}
	if iss.Severity != SeverityError {
		t.Errorf("duplicate-element-id severity = %q, want error", iss.Severity)
	}
}

func TestWideTable(t *testing.T) {
	dir := t.TempDir()
	table := "| a | b | c | d | e | f | g |\n" +
		"|---|---|---|---|---|---|---|\n" +
		"| 1 | 2 | 3 | 4 | 5 | 6 | 7 |\n"
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", table)

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	iss := findIssue(issues, "wide-table")
	if iss == nil {
		t.Fatalf("expected wide-table, got %+v", issues)
	}
	if iss.Severity != SeverityWarning {
		t.Errorf("wide-table severity = %q, want warning", iss.Severity)
	}
}

func TestNarrowTableNotFlagged(t *testing.T) {
	dir := t.TempDir()
	table := "| a | b |\n|---|---|\n| 1 | 2 |\n"
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", table)

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	if findIssue(issues, "wide-table") != nil {
		t.Fatalf("did not expect wide-table, got %+v", issues)
	}
}

func TestImageMissingAlt(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "pic.png"), 10, 10)
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "![](pic.png)\n")

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	iss := findIssue(issues, "image-missing-alt")
	if iss == nil {
		t.Fatalf("expected image-missing-alt, got %+v", issues)
	}
	if iss.Severity != SeverityWarning {
		t.Errorf("image-missing-alt severity = %q, want warning", iss.Severity)
	}
}

func TestImageWithAltNotFlagged(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "pic.png"), 10, 10)
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "![a photo](pic.png)\n")

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	if findIssue(issues, "image-missing-alt") != nil {
		t.Fatalf("did not expect image-missing-alt, got %+v", issues)
	}
}

func TestImageExternalURL(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "![a photo](https://example.com/pic.png)\n")

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	iss := findIssue(issues, "image-external-url")
	if iss == nil {
		t.Fatalf("expected image-external-url, got %+v", issues)
	}
	if iss.Severity != SeverityInfo {
		t.Errorf("image-external-url severity = %q, want info", iss.Severity)
	}
}

func TestImageFileNotFound(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "![a photo](does-not-exist.png)\n")

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	iss := findIssue(issues, "image-file-not-found")
	if iss == nil {
		t.Fatalf("expected image-file-not-found, got %+v", issues)
	}
	if iss.Severity != SeverityError {
		t.Errorf("image-file-not-found severity = %q, want error", iss.Severity)
	}
}

func TestImageOversized(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "big.png"), 40, 10)
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "![a photo](big.png)\n")

	issues := Analyze([]*mdx.Document{doc}, Options{OversizedImageWidth: 20})
	iss := findIssue(issues, "image-oversized")
	if iss == nil {
		t.Fatalf("expected image-oversized, got %+v", issues)
	}
	if iss.Severity != SeverityInfo {
		t.Errorf("image-oversized severity = %q, want info", iss.Severity)
	}
}

func TestImageRightSizedNotFlagged(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "small.png"), 10, 10)
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "![a photo](small.png)\n")

	issues := Analyze([]*mdx.Document{doc}, Options{OversizedImageWidth: 20})
	if findIssue(issues, "image-oversized") != nil {
		t.Fatalf("did not expect image-oversized, got %+v", issues)
	}
}

func TestLongCodeLine(t *testing.T) {
	dir := t.TempDir()
	code := "```\n" + strings.Repeat("x", 150) + "\n```\n"
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", code)

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	iss := findIssue(issues, "long-code-line")
	if iss == nil {
		t.Fatalf("expected long-code-line, got %+v", issues)
	}
	if iss.Severity != SeverityWarning {
		t.Errorf("long-code-line severity = %q, want warning", iss.Severity)
	}
}

func TestShortCodeLineNotFlagged(t *testing.T) {
	dir := t.TempDir()
	code := "```\nshort line\n```\n"
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", code)

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	if findIssue(issues, "long-code-line") != nil {
		t.Fatalf("did not expect long-code-line, got %+v", issues)
	}
}

func TestDeepListNesting(t *testing.T) {
	dir := t.TempDir()
	nested := "<ul><li>a<ul><li>b<ul><li>c<ul><li>d</li></ul></li></ul></li></ul></li></ul>\n"
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", nested)

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	iss := findIssue(issues, "deep-list-nesting")
	if iss == nil {
		t.Fatalf("expected deep-list-nesting, got %+v", issues)
	}
	if iss.Severity != SeverityWarning {
		t.Errorf("deep-list-nesting severity = %q, want warning", iss.Severity)
	}
}

func TestShallowListNotFlagged(t *testing.T) {
	dir := t.TempDir()
	nested := "<ul><li>a<ul><li>b</li></ul></li></ul>\n"
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", nested)

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	if findIssue(issues, "deep-list-nesting") != nil {
		t.Fatalf("did not expect deep-list-nesting, got %+v", issues)
	}
}

func TestLongChapterNoSubheadings(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "# Chapter\n\nThis paragraph has more than five words in it.\n")

	issues := Analyze([]*mdx.Document{doc}, Options{LongChapterWords: 5})
	iss := findIssue(issues, "long-chapter-no-subheadings")
	if iss == nil {
		t.Fatalf("expected long-chapter-no-subheadings, got %+v", issues)
	}
	if iss.Severity != SeverityWarning {
		t.Errorf("long-chapter-no-subheadings severity = %q, want warning", iss.Severity)
	}
}

func TestLongChapterWithSubheadingsNotFlagged(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "# Chapter\n\n## Section\n\nThis paragraph has more than five words in it.\n")

	issues := Analyze([]*mdx.Document{doc}, Options{LongChapterWords: 5})
	if findIssue(issues, "long-chapter-no-subheadings") != nil {
		t.Fatalf("did not expect long-chapter-no-subheadings, got %+v", issues)
	}
}

func TestNoTags(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Doc", "Some content.\n")

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	iss := findIssue(issues, "no-tags")
	if iss == nil {
		t.Fatalf("expected no-tags, got %+v", issues)
	}
	if iss.Severity != SeverityInfo {
		t.Errorf("no-tags severity = %q, want info", iss.Severity)
	}
}

func TestTaggedDocNotFlagged(t *testing.T) {
	dir := t.TempDir()
	content := "---\nid: \"[1.0.0]\"\ntitle: Doc\ntags: [\"intro\"]\n---\n\nSome content.\n"
	path := filepath.Join(dir, "a.mdx")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := mdx.NewParser().ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	if findIssue(issues, "no-tags") != nil {
		t.Fatalf("did not expect no-tags, got %+v", issues)
	}
}

func TestIssueCarriesDocMetadata(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "My Chapter", "[broken](#missing)\n")

	issues := Analyze([]*mdx.Document{doc}, DefaultOptions())
	iss := findIssue(issues, "broken-internal-anchor")
	if iss == nil {
		t.Fatal("expected broken-internal-anchor")
	}
	if iss.File != doc.Path {
		t.Errorf("File = %q, want %q", iss.File, doc.Path)
	}
	if iss.DocTitle != "My Chapter" {
		t.Errorf("DocTitle = %q, want %q", iss.DocTitle, "My Chapter")
	}
}

func TestAnalyzeGroupsByDocumentOrder(t *testing.T) {
	dir := t.TempDir()
	doc1 := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "First", "[broken](#missing)\n")
	doc2 := mustParseDoc(t, dir, "b.mdx", "[2.0.0]", "Second", "[also broken](#missing-too)\n")

	issues := Analyze([]*mdx.Document{doc1, doc2}, DefaultOptions())
	if len(issues) < 2 {
		t.Fatalf("expected at least 2 issues, got %d", len(issues))
	}
	if issues[0].File != doc1.Path {
		t.Errorf("first issue File = %q, want doc1's path %q", issues[0].File, doc1.Path)
	}
	if issues[len(issues)-1].File != doc2.Path {
		t.Errorf("last issue File = %q, want doc2's path %q", issues[len(issues)-1].File, doc2.Path)
	}
}

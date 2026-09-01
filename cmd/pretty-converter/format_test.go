package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFormatFixture(t *testing.T, dir, name, raw string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveFormatInputsSingleFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFormatFixture(t, dir, "notes.txt", "Just a paragraph.")

	got, err := resolveFormatInputs(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != path {
		t.Errorf("resolveFormatInputs(%q) = %v, want [%q]", path, got, path)
	}
}

func TestResolveFormatInputsRejectsNonTxtFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFormatFixture(t, dir, "notes.md", "Already Markdown.")

	if _, err := resolveFormatInputs(path); err == nil {
		t.Error("expected an error for a non-.txt file")
	}
}

func TestResolveFormatInputsDirectorySortedRecursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	writeFormatFixture(t, dir, "b.txt", "B.")
	writeFormatFixture(t, sub, "a.txt", "A.")
	writeFormatFixture(t, dir, "ignored.md", "Not a .txt file.")

	got, err := resolveFormatInputs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 .txt files, got %d: %v", len(got), got)
	}
	// filepath.WalkDir + sort.Strings visits "b.txt" before "sub/a.txt"
	// lexically, since "b" < "sub".
	if filepath.Base(got[0]) != "b.txt" || filepath.Base(got[1]) != "a.txt" {
		t.Errorf("unexpected order: %v", got)
	}
}

func TestResolveFormatInputsNoTxtFilesInDir(t *testing.T) {
	dir := t.TempDir()
	writeFormatFixture(t, dir, "notes.md", "Not a .txt file.")

	if _, err := resolveFormatInputs(dir); err == nil {
		t.Error("expected an error when the directory has no .txt files")
	}
}

func TestResolveFormatInputsMissingPath(t *testing.T) {
	if _, err := resolveFormatInputs(filepath.Join(t.TempDir(), "does-not-exist.txt")); err == nil {
		t.Error("expected an error for a missing input path")
	}
}

// TestRunFormatEndToEnd exercises the format command exactly as a user
// would invoke it — through rootCmd, not by calling runFormat directly —
// then checks the generated output on disk: a scaffolded
// go-pretty-converter.yml plus a valid .mdx file under book/.
func TestRunFormatEndToEnd(t *testing.T) {
	dir := t.TempDir()
	raw := "1. Getting Started\n\n" +
		"This is the introductory paragraph.\n\n" +
		"Installation\n------------\n\n" +
		"Run the installer to get started.\n"
	inPath := writeFormatFixture(t, dir, "notes.txt", raw)
	outDir := filepath.Join(dir, "formatted")

	rootCmd.SetArgs([]string{"format", inPath, "--out", outDir, "--title", "Test Book", "--quiet"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute() = %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "go-pretty-converter.yml")); err != nil {
		t.Errorf("expected a scaffolded go-pretty-converter.yml: %v", err)
	}

	mdxPath := filepath.Join(outDir, "book", "01-1-getting-started.mdx")
	content, err := os.ReadFile(mdxPath)
	if err != nil {
		t.Fatalf("expected generated .mdx at %s: %v", mdxPath, err)
	}
	if !strings.Contains(string(content), "[1.0.0]") {
		t.Errorf("expected [1.0.0] frontmatter id in generated .mdx, got:\n%s", content)
	}
}

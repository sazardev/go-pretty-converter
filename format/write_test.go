package format

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustConvert(t *testing.T, raw string) *Report {
	t.Helper()
	report, err := Convert([]string{writeTempTxt(t, raw)}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestWriteLayout(t *testing.T) {
	report := mustConvert(t, "Introduction\n\nSome body text.")
	outDir := filepath.Join(t.TempDir(), "out")

	if err := Write(report, outDir, YAMLOptions{Title: "My Book", Author: "Jane"}, false); err != nil {
		t.Fatalf("Write: %v", err)
	}

	mdxPath := filepath.Join(outDir, "book", "01-introduction.mdx")
	if _, err := os.Stat(mdxPath); err != nil {
		t.Errorf("expected %s to exist: %v", mdxPath, err)
	}

	cfgPath := filepath.Join(outDir, "go-pretty-converter.yml")
	cfgBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", cfgPath, err)
	}
	cfg := string(cfgBytes)
	if !strings.Contains(cfg, "My Book") || !strings.Contains(cfg, "Jane") {
		t.Errorf("expected scaffold config to contain title/author, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "source: book") {
		t.Errorf("expected scaffold config to set source: book, got:\n%s", cfg)
	}
}

func TestWriteRefusesToClobberWithoutForce(t *testing.T) {
	report := mustConvert(t, "Introduction\n\nSome body text.")
	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "existing.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Write(report, outDir, YAMLOptions{}, false)
	if err == nil {
		t.Fatal("expected an error writing into a non-empty directory without --force")
	}

	if _, statErr := os.Stat(filepath.Join(outDir, "existing.txt")); statErr != nil {
		t.Error("expected the pre-existing directory to be left untouched")
	}
}

func TestWriteForceOverwrites(t *testing.T) {
	report := mustConvert(t, "Introduction\n\nSome body text.")
	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "existing.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Write(report, outDir, YAMLOptions{}, true); err != nil {
		t.Fatalf("Write with force: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "existing.txt")); !os.IsNotExist(err) {
		t.Error("expected --force to replace the old directory contents")
	}
	if _, err := os.Stat(filepath.Join(outDir, "book", "01-introduction.mdx")); err != nil {
		t.Errorf("expected the new content to be present: %v", err)
	}
}

func TestWriteEmptyExistingDirSucceedsWithoutForce(t *testing.T) {
	report := mustConvert(t, "Introduction\n\nSome body text.")
	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := Write(report, outDir, YAMLOptions{}, false); err != nil {
		t.Fatalf("expected writing into an empty existing directory to succeed: %v", err)
	}
}

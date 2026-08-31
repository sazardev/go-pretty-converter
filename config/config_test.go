package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Source != defaultSource {
		t.Errorf("expected source 'book', got %q", cfg.Source)
	}
	if cfg.Output != defaultOutput {
		t.Errorf("expected output 'out.pdf', got %q", cfg.Output)
	}
	if cfg.Title != "Document" {
		t.Errorf("expected title 'Document', got %q", cfg.Title)
	}
	if len(cfg.Lint.RequireFrontmatter) != 2 {
		t.Errorf("expected 2 required frontmatter fields, got %d", len(cfg.Lint.RequireFrontmatter))
	}
	if !cfg.Lint.NoDuplicateIDs {
		t.Error("expected NoDuplicateIDs to be true")
	}
}

func TestLoadThemeOptions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go-pretty-converter.yml")
	content := `theme: corporate
theme_options:
  colors:
    primary: "#1a56db"
    accent: "#0ea5e9"
  fonts:
    heading: "Georgia, serif"
    google_fonts: ["Inter:400,600"]
  sections:
    cover: false
    page_numbers: true
  density: compact
  allow_network_fonts: true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Theme != "corporate" {
		t.Errorf("expected theme 'corporate', got %q", cfg.Theme)
	}
	if cfg.ThemeOptions.Colors.Primary != "#1a56db" {
		t.Errorf("expected primary color '#1a56db', got %q", cfg.ThemeOptions.Colors.Primary)
	}
	if cfg.ThemeOptions.Fonts.Heading != "Georgia, serif" {
		t.Errorf("expected heading font 'Georgia, serif', got %q", cfg.ThemeOptions.Fonts.Heading)
	}
	if len(cfg.ThemeOptions.Fonts.GoogleFonts) != 1 || cfg.ThemeOptions.Fonts.GoogleFonts[0] != "Inter:400,600" {
		t.Errorf("expected google_fonts ['Inter:400,600'], got %v", cfg.ThemeOptions.Fonts.GoogleFonts)
	}
	if cfg.ThemeOptions.Sections.Cover == nil || *cfg.ThemeOptions.Sections.Cover != false {
		t.Error("expected sections.cover to be explicitly false")
	}
	if cfg.ThemeOptions.Sections.PageNumbers == nil || *cfg.ThemeOptions.Sections.PageNumbers != true {
		t.Error("expected sections.page_numbers to be explicitly true")
	}
	if cfg.ThemeOptions.Sections.TOC != nil {
		t.Error("expected sections.toc to be unset (nil)")
	}
	if cfg.ThemeOptions.Density != "compact" {
		t.Errorf("expected density 'compact', got %q", cfg.ThemeOptions.Density)
	}
	if !cfg.ThemeOptions.AllowNetworkFonts {
		t.Error("expected allow_network_fonts to be true")
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go-pretty-converter.yml")
	content := `title: "My Book"
subtitle: "A Guide"
author: "Jane Doe"
source: docs
output: mybook.pdf
theme: minimal
vars:
  api_version: "v2.1"
  company: "Acme Corp"
lint:
  require_frontmatter: [id, title, subtitle]
  require_id_format: "[X.Y.Z]"
  no_duplicate_ids: true
  max_heading_depth: 2
render:
  timeout: 30s
  paper: letter
  margin_top: 10mm
  margin_bottom: 10mm
  header_title: "{{title}}"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Title != "My Book" {
		t.Errorf("expected title 'My Book', got %q", cfg.Title)
	}
	if cfg.Subtitle != "A Guide" {
		t.Errorf("expected subtitle 'A Guide', got %q", cfg.Subtitle)
	}
	if cfg.Author != "Jane Doe" {
		t.Errorf("expected author 'Jane Doe', got %q", cfg.Author)
	}
	if cfg.Source != "docs" {
		t.Errorf("expected source 'docs', got %q", cfg.Source)
	}
	if cfg.Output != "mybook.pdf" {
		t.Errorf("expected output 'mybook.pdf', got %q", cfg.Output)
	}
	if cfg.Theme != "minimal" {
		t.Errorf("expected theme 'minimal', got %q", cfg.Theme)
	}
	if cfg.Vars["api_version"] != "v2.1" {
		t.Errorf("expected var api_version=v2.1, got %q", cfg.Vars["api_version"])
	}
	if cfg.Vars["company"] != "Acme Corp" {
		t.Errorf("expected var company='Acme Corp', got %q", cfg.Vars["company"])
	}
	if len(cfg.Lint.RequireFrontmatter) != 3 {
		t.Errorf("expected 3 required frontmatter fields, got %d", len(cfg.Lint.RequireFrontmatter))
	}
	if cfg.Lint.MaxHeadingDepth != 2 {
		t.Errorf("expected max heading depth 2, got %d", cfg.Lint.MaxHeadingDepth)
	}
	if cfg.Render.Timeout != "30s" {
		t.Errorf("expected timeout '30s', got %q", cfg.Render.Timeout)
	}
	if cfg.Render.Paper != PaperLetter {
		t.Errorf("expected paper 'letter', got %q", cfg.Render.Paper)
	}
	if cfg.Render.MarginTop != "10mm" {
		t.Errorf("expected margin_top '10mm', got %q", cfg.Render.MarginTop)
	}
}

func TestLoadCoverImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go-pretty-converter.yml")
	content := `title: "My Book"
render:
  cover_image: assets/cover.png
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Render.CoverImage != "assets/cover.png" {
		t.Errorf("expected cover_image 'assets/cover.png', got %q", cfg.Render.CoverImage)
	}
}

func TestLoadCustomPaperSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go-pretty-converter.yml")
	content := `title: "Trade Paperback"
render:
  paper: "6x9in"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Render.Paper != "6x9in" {
		t.Errorf("expected paper '6x9in', got %q", cfg.Render.Paper)
	}

	w, h, ok := ParsePaperSize(cfg.Render.Paper)
	if !ok {
		t.Fatal("expected ParsePaperSize to succeed for '6x9in'")
	}
	if w != 6 || h != 9 {
		t.Errorf("expected 6x9 inches, got %vx%v", w, h)
	}
}

func TestLoadDefaultsOnMissingKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go-pretty-converter.yml")
	content := `title: "Just Title"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Title != "Just Title" {
		t.Errorf("expected title 'Just Title', got %q", cfg.Title)
	}
	if cfg.Source != defaultSource {
		t.Errorf("expected default source 'book', got %q", cfg.Source)
	}
	if cfg.Output != defaultOutput {
		t.Errorf("expected default output 'out.pdf', got %q", cfg.Output)
	}
}

func TestFindConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go-pretty-converter.yml")
	if err := os.WriteFile(path, []byte("title: test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	oldDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatal(err)
		}
	}()

	found, err := FindConfig()
	if err != nil {
		t.Fatal(err)
	}
	if found == "" {
		t.Error("expected to find config file")
	}
}

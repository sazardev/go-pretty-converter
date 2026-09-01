package format

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/sazardev/go-pretty-converter/config"
)

// YAMLOptions carries the book-level metadata for the scaffolded
// go-pretty-converter.yml Write produces alongside the generated .mdx
// files.
type YAMLOptions struct {
	Title  string
	Author string
}

// Write persists report's Documents plus a scaffolded go-pretty-converter.yml
// into outDir/, crash-safely: everything is assembled in a sibling temp
// directory first, and only swapped into outDir via one atomic rename once
// every file has been written successfully — a failure partway through (a
// bad path, a full disk) never leaves outDir half-populated, and a
// pre-existing outDir is never touched at all unless the whole batch
// succeeds. This is a directory-scoped adaptation of the same
// temp-then-rename principle epub.Write/kindle.Write use for a single
// file: writing many small files individually-safely would still risk a
// partially-populated outDir if file N of M failed.
//
// If outDir already exists and is non-empty, Write fails unless force is
// true (in which case the existing directory is replaced, not merged,
// once the new content is fully assembled).
func Write(report *Report, outDir string, y YAMLOptions, force bool) error {
	if info, err := os.Stat(outDir); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s exists and is not a directory", outDir)
		}
		entries, readErr := os.ReadDir(outDir)
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", outDir, readErr)
		}
		if len(entries) > 0 && !force {
			return fmt.Errorf("%s already exists and is not empty — use --force to overwrite", outDir)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking %s: %w", outDir, err)
	}

	parent := filepath.Dir(outDir)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", parent, err)
	}

	tmpDir, err := os.MkdirTemp(parent, filepath.Base(outDir)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp output directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	bookDir := filepath.Join(tmpDir, "book")
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", bookDir, err)
	}

	for _, doc := range report.Documents {
		path := filepath.Join(bookDir, doc.Filename)
		if err := os.WriteFile(path, []byte(doc.Content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}

	if err := writeScaffoldConfig(tmpDir, report, y); err != nil {
		return err
	}

	if _, err := os.Stat(outDir); err == nil {
		if err := os.RemoveAll(outDir); err != nil {
			return fmt.Errorf("removing existing %s: %w", outDir, err)
		}
	}
	if err := os.Rename(tmpDir, outDir); err != nil {
		return fmt.Errorf("finalizing output directory: %w", err)
	}

	return nil
}

// writeScaffoldConfig marshals a real config.Config — the exact struct
// config.Load deserializes into, so the scaffold can never drift out of
// sync with what the CLI actually understands — so `cd <out> &&
// pretty-converter build` works immediately with zero extra flags, the
// same guarantee `pretty-converter init` gives.
func writeScaffoldConfig(tmpDir string, report *Report, y YAMLOptions) error {
	themeName := report.SuggestedTheme
	if themeName == "" {
		themeName = "default"
	}
	cfg := config.Config{
		Title:  y.Title,
		Author: y.Author,
		Source: "book",
		Output: "out.pdf",
		Theme:  themeName,
		Lint: config.LintConfig{
			RequireFrontmatter: []string{"id", "title"},
			NoDuplicateIDs:     true,
			MaxHeadingDepth:    5,
		},
	}
	cfgBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", config.DefaultConfigFile, err)
	}
	path := filepath.Join(tmpDir, config.DefaultConfigFile)
	if err := os.WriteFile(path, cfgBytes, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

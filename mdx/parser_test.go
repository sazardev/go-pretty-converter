package mdx

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const (
	testFileA    = "a.mdx"
	testSecondID = "[2.0.0]"
)

func TestParserParseDirNoMDXFiles(t *testing.T) {
	dir := t.TempDir()

	p := NewParser()
	_, err := p.ParseDir(dir)
	if err == nil {
		t.Fatal("expected error for directory with no .md, .mdx, or .txt files")
	}
	if !contains(err.Error(), "no .md, .mdx, or .txt files found") {
		t.Errorf("expected 'no .md, .mdx, or .txt files found' error, got: %v", err)
	}
}

func TestParserParseFileHighlightsFencedCode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "code.mdx")
	content := "---\nid: \"[1.0.0]\"\ntitle: Code\n---\n\n```go\nfunc main() {}\n```\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewParser()
	doc, err := p.ParseFile(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !contains(doc.HTML, "chroma-kd") {
		t.Errorf("expected syntax-highlighted output with chroma- prefixed classes, got: %s", doc.HTML)
	}
}

func TestParserParseDirAcceptsPlainMD(t *testing.T) {
	dir := t.TempDir()

	valid := `---
id: "[1.0.0]"
title: Valid
---

# Valid
`
	if err := os.WriteFile(filepath.Join(dir, "valid.md"), []byte(valid), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewParser()
	docs, err := p.ParseDir(dir)
	if err != nil {
		t.Fatalf("expected no error parsing .md file, got: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 parsed doc, got %d", len(docs))
	}
}

func TestParserParseDirTxtAutoFrontmatter(t *testing.T) {
	dir := t.TempDir()

	mdx := `---
id: "[1.0.0]"
title: Intro
---

# Intro
`
	if err := os.WriteFile(filepath.Join(dir, "01-intro.mdx"), []byte(mdx), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "03-notes-from-field.txt"), []byte("Line one.\nLine two.\n\nSecond paragraph."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "random-thoughts.txt"), []byte("<script>alert(1)</script>"), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewParser()
	docs, err := p.ParseDir(dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected 3 parsed docs, got %d", len(docs))
	}

	byID := make(map[string]*Document, len(docs))
	for _, d := range docs {
		byID[d.ID()] = d
	}

	notes, ok := byID["[3.0.0]"]
	if !ok {
		t.Fatalf("expected numbered .txt to get id [3.0.0], got ids: %v", keysOf(byID))
	}
	if notes.Title() != "Notes From Field" {
		t.Errorf("expected title 'Notes From Field', got %q", notes.Title())
	}
	if !contains(notes.HTML, "<br>") || !contains(notes.HTML, "<p>") {
		t.Errorf("expected paragraph/line-break HTML, got: %q", notes.HTML)
	}

	unnumbered, ok := byID["[4.0.0]"]
	if !ok {
		t.Fatalf("expected unprefixed .txt to get next free id [4.0.0] (max used is 3), got ids: %v", keysOf(byID))
	}
	if unnumbered.Title() != "Random Thoughts" {
		t.Errorf("expected title 'Random Thoughts', got %q", unnumbered.Title())
	}
	if contains(unnumbered.HTML, "<script>") {
		t.Errorf("expected .txt content to be HTML-escaped, got raw script tag: %q", unnumbered.HTML)
	}
}

func TestParserParseDirAutoFrontmatterForBareMDX(t *testing.T) {
	dir := t.TempDir()

	mdx := `---
id: "[1.0.0]"
title: Intro
---

# Intro
`
	if err := os.WriteFile(filepath.Join(dir, "01-intro.mdx"), []byte(mdx), 0644); err != nil {
		t.Fatal(err)
	}
	// No frontmatter block at all: gets an auto id/title from its
	// filename, same convention as .txt, instead of failing.
	bare := "# Getting Started\n\nSome **markdown** content, no frontmatter needed.\n"
	if err := os.WriteFile(filepath.Join(dir, "02-getting-started.mdx"), []byte(bare), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewParser()
	docs, err := p.ParseDir(dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 parsed docs, got %d", len(docs))
	}

	byID := make(map[string]*Document, len(docs))
	for _, d := range docs {
		byID[d.ID()] = d
	}

	bareDoc, ok := byID["[2.0.0]"]
	if !ok {
		t.Fatalf("expected bare .mdx to get id [2.0.0] from its filename, got ids: %v", keysOf(byID))
	}
	if bareDoc.Title() != "Getting Started" {
		t.Errorf("expected title 'Getting Started', got %q", bareDoc.Title())
	}
	// Unlike .txt, a bare .mdx still gets full markdown rendering (bold,
	// headings, components, raw HTML), not literal escaped text.
	if !contains(bareDoc.HTML, "<strong>markdown</strong>") {
		t.Errorf("expected bare .mdx content to be rendered as real markdown, got: %q", bareDoc.HTML)
	}
}

func keysOf(m map[string]*Document) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestParserParseFileMissingFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-frontmatter.mdx")
	if err := os.WriteFile(path, []byte("# Just a heading\n\nNo frontmatter here.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewParser()
	_, err := p.ParseFile(path)
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
	if !contains(err.Error(), "missing frontmatter") {
		t.Errorf("expected 'missing frontmatter' error, got: %v", err)
	}
}

func TestParserParseDirPartialFailure(t *testing.T) {
	dir := t.TempDir()

	valid := `---
id: "[1.0.0]"
title: Valid
---

# Valid
`
	// A --- block that's present but not valid YAML must still be a hard
	// error — unlike a file with no --- block at all, which now gets an
	// auto-generated id/title instead of failing (see
	// TestParserParseDirAutoFrontmatterForBareMDX).
	invalid := "---\nid: \"[2.0.0\ntitle: [Broken\n---\n\n# Invalid\n"

	if err := os.WriteFile(filepath.Join(dir, "valid.mdx"), []byte(valid), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "invalid.mdx"), []byte(invalid), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewParser()
	docs, err := p.ParseDir(dir)

	if len(docs) != 1 {
		t.Fatalf("expected 1 successfully parsed doc, got %d", len(docs))
	}
	if err == nil {
		t.Fatal("expected a non-nil error describing the partial failure")
	}
	var parseErrs ParseErrors
	if !errors.As(err, &parseErrs) {
		t.Fatalf("expected error to be a ParseErrors, got %T", err)
	}
	if len(parseErrs) != 1 {
		t.Fatalf("expected 1 parse error, got %d", len(parseErrs))
	}
}

func TestParseErrorsError(t *testing.T) {
	var empty ParseErrors
	if got := empty.Error(); got != "" {
		t.Errorf("expected empty string for 0 errors, got %q", got)
	}

	single := ParseErrors{{File: testFileA, Err: errors.New("boom")}}
	if got := single.Error(); !contains(got, testFileA) || !contains(got, "boom") {
		t.Errorf("expected single error message to mention file and cause, got %q", got)
	}

	multi := ParseErrors{
		{File: testFileA, Err: errors.New("boom")},
		{File: "b.mdx", Err: errors.New("bang")},
	}
	got := multi.Error()
	if !contains(got, "2 file(s) failed to parse") {
		t.Errorf("expected aggregate message for multiple errors, got %q", got)
	}
}

func TestParseFileErrorUnwrap(t *testing.T) {
	cause := errors.New("underlying failure")
	pfe := ParseFileError{File: "x.mdx", Err: cause}

	if !errors.Is(pfe, cause) {
		t.Error("expected errors.Is to find the wrapped cause via Unwrap")
	}
	if !contains(pfe.Error(), "x.mdx") {
		t.Errorf("expected error message to include file name, got %q", pfe.Error())
	}
}

func TestParserParseAll(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		testFileA: `---
id: "[2.0.0]"
title: Second
---

# Second
`,
		"b.mdx": `---
id: "[1.0.0]"
title: First
---

# First
`,
	}

	paths := make([]string, 0, len(files))
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}

	p := NewParser()
	docs, err := p.ParseAll(paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(docs))
	}
	if docs[0].ID() != defaultIDValue || docs[1].ID() != testSecondID {
		t.Errorf("expected docs sorted by ID, got %s then %s", docs[0].ID(), docs[1].ID())
	}
}

func TestParserParseAllPartialFailure(t *testing.T) {
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "good.mdx")
	if err := os.WriteFile(goodPath, []byte(`---
id: "[1.0.0]"
title: Good
---

# Good
`), 0644); err != nil {
		t.Fatal(err)
	}
	missingPath := filepath.Join(dir, "missing.mdx")

	p := NewParser()
	docs, err := p.ParseAll([]string{goodPath, missingPath})
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc from the successful path, got %d", len(docs))
	}
	if err == nil {
		t.Fatal("expected error for the missing file")
	}
}

// TestSubstituteVarsDoesNotChainPlaceholders guards against a bug where
// sequentially replacing one var at a time let one var's value (itself
// containing another var's {{placeholder}}) get expanded a second time —
// making the result depend on Go's randomized map iteration order instead
// of always doing exactly one substitution pass.
func TestSubstituteVarsDoesNotChainPlaceholders(t *testing.T) {
	p := NewParser(WithVars(map[string]string{
		"user":  "{{admin}}",
		"admin": "SECRET",
	}))

	for i := 0; i < 20; i++ {
		got := string(p.substituteVars([]byte("hello {{user}}")))
		if got != "hello {{admin}}" {
			t.Fatalf("substituteVars() = %q, want %q (no chaining into other placeholders)", got, "hello {{admin}}")
		}
	}
}

func TestSubstituteVarsReplacesAllOccurrences(t *testing.T) {
	p := NewParser(WithVars(map[string]string{"name": "Acme"}))
	got := string(p.substituteVars([]byte("{{name}} and {{name}} again")))
	if got != "Acme and Acme again" {
		t.Fatalf("substituteVars() = %q, want %q", got, "Acme and Acme again")
	}
}

package format

import (
	"path/filepath"
	"testing"

	"github.com/sazardev/go-pretty-converter/mdx"
)

// TestGeneratedMDXRoundTripsThroughMDXParser is the critical correctness
// check: format's output must be valid input to the *existing* mdx.Parser
// pipeline, not just plausible-looking text. A raw sample deliberately
// exercises every rule (a numbered chapter title, a Setext subheading, a
// bullet list, an indented code block with an internal blank line, and
// prose containing Markdown metacharacters that must survive escaped).
func TestGeneratedMDXRoundTripsThroughMDXParser(t *testing.T) {
	raw := `1. Getting Started

This chapter costs $5 * 2 and uses a variable named my_var. See [not a link].

Installation
------------

Run the installer, then:

    func main() {
        fmt.Println("a")
    }

    func other() {
        fmt.Println("b")
    }

Steps
-----

- First step
- Second step
- Third step
`
	report, err := Convert([]string{writeTempTxt(t, raw)}, DefaultOptions())
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if len(report.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(report.Documents))
	}

	outDir := filepath.Join(t.TempDir(), "out")
	if writeErr := Write(report, outDir, YAMLOptions{Title: "Test Book"}, false); writeErr != nil {
		t.Fatalf("Write: %v", writeErr)
	}

	docs, err := mdx.NewParser().ParseDir(filepath.Join(outDir, "book"))
	if err != nil {
		t.Fatalf("mdx.Parser.ParseDir failed to ingest generated output: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 parsed document, got %d", len(docs))
	}

	doc := docs[0]
	if doc.ID() != id1 {
		t.Errorf("expected id %s, got %q", id1, doc.ID())
	}
	// The detected heading text is kept verbatim as the title (including
	// its "1. " prefix) — format doesn't try to strip numbering from
	// author-written text, only to detect structure.
	if doc.Title() != "1. Getting Started" {
		t.Errorf("expected title %q, got %q", "1. Getting Started", doc.Title())
	}

	v := mdx.NewDefaultValidator()
	for _, e := range v.Validate(doc) {
		if e.Field != mdx.ContentField {
			t.Errorf("unexpected structural validation error: %v", e)
		}
	}
}

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	prettyconverter "github.com/sazardev/go-pretty-converter"
	"github.com/sazardev/go-pretty-converter/render"
)

const wantDocs = 8

// wantInPDF spans every page of the book: built-in components, all eight
// custom components, GFM extras, and the closing summary. If any of
// these disappear, something in the pipeline broke.
var wantInPDF = []string{
	"go-pretty-converter Showcase", // cover title
	"Go 1.26+",                     // project-facts table
	"Headless Chrome",              // project-facts table
	"Blocked by default",           // project-facts table
	"Heading Level 5",              // typography: h1-h5
	"Test coverage",                // custom Progress component
	"Security hardening",           // custom Timeline component
	"CALLOUT: INFO",                // custom Callout component
	"CALLOUT: SUCCESS",             // custom Callout component
	"CALLOUT: WARNING",             // custom Callout component
	"CALLOUT: DANGER",              // custom Callout component
	"Capabilities Summary",         // closing page
}

func TestShowcaseComposesVisibleData(t *testing.T) {
	pdf, err := prettyconverter.New(showcaseOptions()...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	docs, err := pdf.ParseDir()
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(docs) != wantDocs {
		t.Fatalf("expected %d documents, got %d", wantDocs, len(docs))
	}

	if errs := pdf.ValidateAll(docs); len(errs) != 0 {
		t.Errorf("expected no validation errors, got %d: %v", len(errs), errs)
	}

	html, err := pdf.ComposeHTML(docs)
	if err != nil {
		t.Fatalf("ComposeHTML: %v", err)
	}

	for _, want := range wantInPDF {
		if !strings.Contains(html, want) {
			t.Errorf("expected composed HTML to contain %q, it did not", want)
		}
	}
}

// outputDir is where the generated PDF is left on disk after the test so
// it can be opened and inspected manually. It mirrors the convention used
// by examples/main.go and is already covered by the repo's .gitignore
// (examples/output/, *.pdf).
const outputDir = "output"

func TestShowcaseBuildsValidPDF(t *testing.T) {
	if err := render.CheckChromeAvailable(); err != nil {
		t.Skipf("Chrome/Chromium not available: %v", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("creating output dir: %v", err)
	}
	outPath := filepath.Join(outputDir, "showcase.pdf")

	opts := append(showcaseOptions(), prettyconverter.WithOutputFile(outPath))
	pdf, err := prettyconverter.New(opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := pdf.Build(context.Background()); err != nil {
		t.Fatalf("Build: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("expected output PDF to exist: %v", err)
	}
	if len(data) < 4 || string(data[:4]) != "%PDF" {
		t.Error("expected output to start with a %PDF magic header")
	}
	const minPlausiblePDFSize = 20 * 1024 // an 8-page book with this much content won't be tiny
	if len(data) < minPlausiblePDFSize {
		t.Errorf("PDF looks too small to contain real content: %d bytes", len(data))
	}

	abs, err := filepath.Abs(outPath)
	if err != nil {
		abs = outPath
	}
	t.Logf("PDF generated: %s (%d bytes, %d documents)", abs, len(data), wantDocs)
}

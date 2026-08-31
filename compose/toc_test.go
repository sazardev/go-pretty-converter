package compose

import (
	"strings"
	"testing"

	"github.com/sazardev/go-pretty-converter/mdx"
)

func TestBuildTOCSkipsMalformedID(t *testing.T) {
	docs := []*mdx.Document{
		docWithID("invalid", "Untitled", "<p>x</p>"),
	}

	toc := buildTOC(docs)
	if strings.Contains(toc, "Untitled") {
		t.Error("expected malformed-ID doc to be skipped from the TOC")
	}
}

func TestBuildTOCLevels(t *testing.T) {
	docs := []*mdx.Document{
		docWithID("[1.0.0]", "One", "<h1>One</h1>"),
		docWithID("[1.1.0]", "OneOne", "<h2>OneOne</h2>"),
		docWithID("[1.1.1]", "OneOneOne", "<h3>OneOneOne</h3>"),
	}

	toc := buildTOC(docs)

	if !strings.Contains(toc, `class="toc-h1"`) {
		t.Error("expected a toc-h1 entry")
	}
	if !strings.Contains(toc, `class="toc-h2"`) {
		t.Error("expected a toc-h2 entry")
	}
	if !strings.Contains(toc, `class="toc-h3"`) {
		t.Error("expected a toc-h3 entry")
	}
	for _, title := range []string{"One", "OneOne", "OneOneOne"} {
		if !strings.Contains(toc, title) {
			t.Errorf("expected TOC to mention %q", title)
		}
	}
}

func TestBuildTOCDuplicateH1Fallback(t *testing.T) {
	docs := []*mdx.Document{
		docWithID("[1.0.0]", "First", "<h1>First</h1>"),
		docWithID("[1.0.0]", "Duplicate", "<h1>Duplicate</h1>"),
	}

	toc := buildTOC(docs)

	count := strings.Count(toc, `class="toc-h1"`)
	if count != 2 {
		t.Errorf("expected both entries (first + duplicate fallback) to render as toc-h1, got %d occurrences", count)
	}
	if !strings.Contains(toc, "First") || !strings.Contains(toc, "Duplicate") {
		t.Error("expected both duplicate-ID docs to appear in the TOC")
	}
}

package format

import "testing"

func TestIsHeadingCandidate(t *testing.T) {
	opts := DefaultOptions()
	tests := []struct {
		name string
		line string
		want bool
	}{
		{"short title", "Getting Started", true},
		{"numbered chapter title", "1. Introduction", true},
		{"numbered chapter title with paren", "2) Overview", true},
		{"ends in period", "This is a sentence.", false},
		{"ends in comma", "Wait, there is more,", false},
		{"ends in question mark", "Is this a heading?", false},
		{"too many words", "This line has way too many words to plausibly be a short section title", false},
		{"too many chars", "This line is deliberately constructed to exceed the seventy character heading length threshold by quite a lot", false},
		{"bullet marker excluded", "- Introduction", false},
		{"star bullet excluded", "* Introduction", false},
		{"divider line", "---", false},
		{"divider equals", "===", false},
		{"no letters", "12345", false},
		{"empty", "", false},
		{"whitespace only", "   ", false},
		{"accented title", "Última Actualización", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isHeadingCandidate(tt.line, opts); got != tt.want {
				t.Errorf("isHeadingCandidate(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestTier1Heading(t *testing.T) {
	opts := DefaultOptions()

	t.Run("single line", func(t *testing.T) {
		title, ok := tier1Heading(block{lines: []string{titleIntroduction}}, opts)
		if !ok || title != titleIntroduction {
			t.Errorf("got (%q, %v), want (\"Introduction\", true)", title, ok)
		}
	})

	t.Run("setext equals", func(t *testing.T) {
		title, ok := tier1Heading(block{lines: []string{titleIntroduction, "========"}}, opts)
		if !ok || title != titleIntroduction {
			t.Errorf("got (%q, %v), want (\"Introduction\", true)", title, ok)
		}
	})

	t.Run("setext dashes is not tier1", func(t *testing.T) {
		_, ok := tier1Heading(block{lines: []string{titleOverview, "--------"}}, opts)
		if ok {
			t.Error("expected setext dashes to not qualify as tier1")
		}
	})

	t.Run("ordinary paragraph", func(t *testing.T) {
		_, ok := tier1Heading(block{lines: []string{"This is just a normal sentence."}}, opts)
		if ok {
			t.Error("expected an ordinary sentence to not qualify as tier1")
		}
	})

	t.Run("two-line list is not tier1", func(t *testing.T) {
		_, ok := tier1Heading(block{lines: []string{"1. Step one", "2. Step two"}}, opts)
		if ok {
			t.Error("expected a two-line numbered list to not qualify as tier1")
		}
	})
}

func TestTier2Heading(t *testing.T) {
	opts := DefaultOptions()

	title, ok := tier2Heading(block{lines: []string{titleOverview, "--------"}}, opts)
	if !ok || title != titleOverview {
		t.Errorf("got (%q, %v), want (\"Overview\", true)", title, ok)
	}

	if _, ok := tier2Heading(block{lines: []string{titleOverview}}, opts); ok {
		t.Error("expected a lone short line (no --- underline) to not qualify as tier2")
	}
}

func TestSplitChaptersSingleHeading(t *testing.T) {
	raw := "Introduction\n\nThis is the body text of the chapter."
	chapters := splitChapters(raw, DefaultOptions())
	if len(chapters) != 1 {
		t.Fatalf("expected 1 chapter, got %d", len(chapters))
	}
	if chapters[0].Title != titleIntroduction {
		t.Errorf("expected title %q, got %q", titleIntroduction, chapters[0].Title)
	}
	if len(chapters[0].Blocks) != 1 {
		t.Fatalf("expected 1 body block, got %d", len(chapters[0].Blocks))
	}
}

func TestSplitChaptersMultiChapter(t *testing.T) {
	raw := "Introduction\n\nFirst chapter body.\n\nGetting Started\n\nSecond chapter body."
	chapters := splitChapters(raw, DefaultOptions())
	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chapters))
	}
	if chapters[0].Title != titleIntroduction || chapters[1].Title != "Getting Started" {
		t.Errorf("unexpected titles: %q, %q", chapters[0].Title, chapters[1].Title)
	}
}

func TestSplitChaptersColdStart(t *testing.T) {
	raw := "Just a paragraph with no heading at all in this short file."
	chapters := splitChapters(raw, DefaultOptions())
	if len(chapters) != 1 {
		t.Fatalf("expected 1 chapter, got %d", len(chapters))
	}
	if chapters[0].Title != "" {
		t.Errorf("expected empty title for a headingless file, got %q", chapters[0].Title)
	}
	if len(chapters[0].Blocks) != 1 {
		t.Fatalf("expected 1 body block, got %d", len(chapters[0].Blocks))
	}
}

func TestSplitChaptersSetextDashesStaysInChapter(t *testing.T) {
	raw := "Introduction\n\nSubsection\n----------\n\nBody text here."
	chapters := splitChapters(raw, DefaultOptions())
	if len(chapters) != 1 {
		t.Fatalf("expected 1 chapter (--- must not start a new one), got %d", len(chapters))
	}
	if len(chapters[0].Blocks) != 2 {
		t.Fatalf("expected 2 body blocks, got %d", len(chapters[0].Blocks))
	}
}

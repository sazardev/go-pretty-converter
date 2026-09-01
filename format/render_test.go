package format

import (
	"strings"
	"testing"
)

const listItemOne = "- one"

func TestIsListBlock(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  bool
	}{
		{"two bullet items", []string{listItemOne, "- two"}, true},
		{"two numbered items", []string{"1. Step one", "2. Step two"}, true},
		{"lone numbered line is not a list", []string{"1. Introduction"}, false},
		{"lone bullet line is not a list", []string{"- Introduction"}, false},
		{"mixed markers still count", []string{listItemOne, "2. two"}, true},
		{"deeply indented is not a top-level list", []string{"     - one", "     - two"}, false},
		{"not every line matches", []string{listItemOne, "just prose"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isListBlock(block{lines: tt.lines}); got != tt.want {
				t.Errorf("isListBlock(%v) = %v, want %v", tt.lines, got, tt.want)
			}
		})
	}
}

func TestRenderList(t *testing.T) {
	got := renderList(block{lines: []string{"* First item", "2) Second item"}})
	want := "- First item\n2. Second item"
	if got != want {
		t.Errorf("renderList() = %q, want %q", got, want)
	}
}

func TestIsIndentedCodeBlock(t *testing.T) {
	if !isIndentedCodeBlock(block{lines: []string{"    func a() {}", "    func b() {}"}}) {
		t.Error("expected 4-space-indented lines to be detected as code")
	}
	if !isIndentedCodeBlock(block{lines: []string{"\tfunc a() {}"}}) {
		t.Error("expected a tab-indented line to be detected as code")
	}
	if isIndentedCodeBlock(block{lines: []string{"  only two spaces"}}) {
		t.Error("did not expect a 2-space-indented line to be detected as code")
	}
	if isIndentedCodeBlock(block{lines: []string{}}) {
		t.Error("did not expect an empty block to be detected as code")
	}
}

func TestRenderCodeBlockDedents(t *testing.T) {
	got := renderCodeBlock(block{lines: []string{"    func a() {", "        return 1", "    }"}})
	want := "```\nfunc a() {\n    return 1\n}\n```"
	if got != want {
		t.Errorf("renderCodeBlock() =\n%s\nwant\n%s", got, want)
	}
}

func TestEscapeParagraphText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"emphasis characters", "Rate is 5% * 3_ [test]", `Rate is 5% \* 3\_ \[test\]`},
		{"backslash first", `a\b`, `a\\b`},
		{"line-initial hash", "# not a heading", `\# not a heading`},
		{"line-initial dash", "- not a list", `\- not a list`},
		{"line-initial blockquote", "> not a quote", `\> not a quote`},
		{"plain text unaffected", "Just a normal sentence", "Just a normal sentence"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeParagraphText(tt.in); got != tt.want {
				t.Errorf("escapeParagraphText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRenderChapterBodyMergesConsecutiveIndentedCode(t *testing.T) {
	blocks := splitBlankLineBlocks("    func a() {}\n\n    func b() {}")
	if len(blocks) != 2 {
		t.Fatalf("test setup: expected 2 blocks pre-merge, got %d", len(blocks))
	}
	body, _, _, code, _ := renderChapterBody(blocks, DefaultOptions())
	if code != 1 {
		t.Errorf("expected the merge pass to produce 1 code block, got %d", code)
	}
	if strings.Count(body, "```") != 2 {
		t.Errorf("expected exactly one fenced block (2 fence markers) in body, got:\n%s", body)
	}
	if !strings.Contains(body, "func a() {}") || !strings.Contains(body, "func b() {}") {
		t.Errorf("expected both functions to survive the merge, got:\n%s", body)
	}
}

func TestRenderChapterBodyCounts(t *testing.T) {
	blocks := splitBlankLineBlocks("A paragraph.\n\n- one\n- two\n\nOverview\n--------\n\nAnother paragraph.")
	body, headings, lists, code, paragraphs := renderChapterBody(blocks, DefaultOptions())
	if headings != 1 {
		t.Errorf("expected 1 heading, got %d", headings)
	}
	if lists != 1 {
		t.Errorf("expected 1 list, got %d", lists)
	}
	if code != 0 {
		t.Errorf("expected 0 code blocks, got %d", code)
	}
	if paragraphs != 2 {
		t.Errorf("expected 2 paragraphs, got %d", paragraphs)
	}
	if !strings.Contains(body, "## Overview") {
		t.Errorf("expected a rendered ## Overview heading, got:\n%s", body)
	}
}

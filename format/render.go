package format

import (
	"regexp"
	"strings"
)

var (
	bulletLineRe   = regexp.MustCompile(`^([-*•])(\s+)(\S.*)$`)
	numberedLineRe = regexp.MustCompile(`^(\d+)([.)])(\s+)(\S.*)$`)
)

// blockKind* label a block's per-chapter classification in
// renderChapterBody's merge pass; "fenced" isn't a separate constant since
// it's only ever compared once, right after being set.
const blockKindIndented = "indented"

// isIndentedCodeBlock reports whether every non-blank line in b starts with
// a tab or at least 4 spaces — the classic "indented code block" signal.
func isIndentedCodeBlock(b block) bool {
	saw := false
	for _, line := range b.lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, "    ") {
			return false
		}
		saw = true
	}
	return saw
}

// commonLeadingWhitespace returns the longest whitespace prefix shared by
// every non-blank line, for dedenting an indented code block. Compared
// byte-for-byte (whitespace is ASCII) rather than by column-width, so mixed
// tabs/spaces never produce an off-by-one slice.
func commonLeadingWhitespace(lines []string) string {
	var prefix string
	first := true
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lead := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if first {
			prefix = lead
			first = false
			continue
		}
		prefix = commonPrefix(prefix, lead)
	}
	return prefix
}

func commonPrefix(a, b string) string {
	n := min(len(a), len(b))
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return a[:i]
}

func renderCodeBlock(b block) string {
	prefix := commonLeadingWhitespace(b.lines)
	var out strings.Builder
	out.WriteString(fenceMarker + "\n")
	for _, line := range b.lines {
		out.WriteString(strings.TrimPrefix(line, prefix))
		out.WriteString("\n")
	}
	out.WriteString(fenceMarker)
	return out.String()
}

// isListBlock reports whether every non-blank line in b is a bullet
// (-, *, •) or numbered (N. / N)) item, with at most 3 leading spaces
// (CommonMark's own top-level-list-indent allowance), and there are at
// least 2 such lines. The >=2 threshold is what routes a lone "1.
// Introduction" line to heading detection instead of here — see
// isHeadingCandidate's doc comment.
func isListBlock(b block) bool {
	count := 0
	for _, line := range b.lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if leadingSpaces(line) > 3 {
			return false
		}
		trimmed := strings.TrimLeft(line, " ")
		if bulletLineRe.MatchString(trimmed) || numberedLineRe.MatchString(trimmed) {
			count++
			continue
		}
		return false
	}
	return count >= 2
}

func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// renderList normalizes a list block's markers (*/• -> -, ) -> .) and
// escapes each item's text content.
func renderList(b block) string {
	var out []string
	for _, line := range b.lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := strings.Repeat(" ", leadingSpaces(line))
		trimmed := strings.TrimLeft(line, " ")
		if m := bulletLineRe.FindStringSubmatch(trimmed); m != nil {
			out = append(out, indent+"- "+escapeParagraphText(m[3]))
			continue
		}
		if m := numberedLineRe.FindStringSubmatch(trimmed); m != nil {
			out = append(out, indent+m[1]+". "+escapeParagraphText(m[4]))
			continue
		}
	}
	return strings.Join(out, "\n")
}

var paragraphEscapeReplacer = strings.NewReplacer(
	`\`, `\\`,
	"`", "\\`",
	`*`, `\*`,
	`_`, `\_`,
	`[`, `\[`,
	`]`, `\]`,
)

// escapeParagraphText backslash-escapes Markdown metacharacters in raw
// prose so it survives being fed back through goldmark as literal text
// instead of being reinterpreted as emphasis/links/headings/blockquotes —
// the output here is real Markdown source, not pre-rendered HTML (unlike
// mdx/plaintext.go's renderPlainText), so it has to survive re-parsing.
func escapeParagraphText(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		escaped := paragraphEscapeReplacer.Replace(line)
		// A line-initial '#', '>', '-', '+' would be read as a block
		// marker (heading/blockquote/list) even after the inline
		// replacements above, since those only trigger mid-line.
		for _, prefix := range []string{"#", ">", "-", "+"} {
			if strings.HasPrefix(escaped, prefix) {
				escaped = `\` + escaped
				break
			}
		}
		lines[i] = escaped
	}
	return strings.Join(lines, "\n")
}

func renderParagraph(b block) string {
	return escapeParagraphText(b.text())
}

// renderChapterBody converts a rawChapter's blocks into final Markdown
// body text, applying (in precedence order) fenced passthrough, indented
// code detection + a merge pass, list normalization, Tier-2 Setext
// subheadings, and escaped-paragraph fallback. Returns the body plus
// counts for the report.
func renderChapterBody(blocks []block, opts Options) (body string, headings, lists, code, paragraphs int) {
	kinds := make([]string, len(blocks))
	for i, b := range blocks {
		switch {
		case isFencedBlock(b):
			kinds[i] = "fenced"
		case isIndentedCodeBlock(b):
			kinds[i] = blockKindIndented
		}
	}

	// Merge consecutive blockKindIndented blocks (separated only by the blank
	// line splitBlankLineBlocks already collapsed) into one, so a pasted
	// listing with an internal blank line between functions renders as one
	// continuous fenced block instead of several small ones.
	var merged []block
	var mergedKinds []string
	i := 0
	for i < len(blocks) {
		if kinds[i] == blockKindIndented {
			j := i
			var lines []string
			for j < len(blocks) && kinds[j] == blockKindIndented {
				if len(lines) > 0 {
					lines = append(lines, "")
				}
				lines = append(lines, blocks[j].lines...)
				j++
			}
			merged = append(merged, block{lines: lines})
			mergedKinds = append(mergedKinds, blockKindIndented)
			i = j
			continue
		}
		merged = append(merged, blocks[i])
		mergedKinds = append(mergedKinds, kinds[i])
		i++
	}

	var out strings.Builder
	for idx, b := range merged {
		if idx > 0 {
			out.WriteString("\n\n")
		}
		switch {
		case mergedKinds[idx] == "fenced":
			out.WriteString(b.text())
			code++
		case mergedKinds[idx] == blockKindIndented:
			out.WriteString(renderCodeBlock(b))
			code++
		case isListBlock(b):
			out.WriteString(renderList(b))
			lists++
		default:
			if title, ok := tier2Heading(b, opts); ok {
				out.WriteString("## " + escapeParagraphText(title))
				headings++
			} else {
				out.WriteString(renderParagraph(b))
				paragraphs++
			}
		}
	}
	return out.String(), headings, lists, code, paragraphs
}

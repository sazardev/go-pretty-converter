package format

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	dividerLineRe  = regexp.MustCompile(`^[=\-_*~]{3,}$`)
	setextEqualsRe = regexp.MustCompile(`^={3,}\s*$`)
	setextDashesRe = regexp.MustCompile(`^-{3,}\s*$`)
	bulletMarkerRe = regexp.MustCompile(`^[-*•]\s`)
)

// isHeadingCandidate reports whether line, on its own, plausibly reads as a
// short document/section title rather than a sentence — the signal used by
// splitChapters (isolated single-line blocks) and the Setext-underline
// Tier-2 rule.
//
// Deliberately conservative: a false negative here just means a real
// heading stays a plain paragraph (safe, low-cost); a false positive would
// misfile ordinary prose as a chapter break (high-cost, hard to notice).
// Every check below exists to suppress one specific class of false
// positive rather than to positively detect "heading-ness" — numbered
// markers ("1. Introduction") are deliberately NOT excluded here even
// though bullet markers are: a lone numbered line is a common raw-text
// chapter-title convention, not a list (lists need >=2 marker lines to
// qualify — see isListBlock), so excluding numbers here would misfile the
// single most common chapter-title shape in unstructured manuscripts.
func isHeadingCandidate(line string, opts Options) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	if utf8.RuneCountInString(line) > opts.MaxHeadingChars {
		return false
	}
	if len(strings.Fields(line)) > opts.MaxHeadingWords {
		return false
	}
	// A heading rarely ends the way a sentence does.
	r, _ := utf8.DecodeLastRuneInString(line)
	switch r {
	case '.', ',', ';', ':', '!', '?':
		return false
	}
	if bulletMarkerRe.MatchString(line) {
		return false
	}
	if dividerLineRe.MatchString(line) {
		return false
	}
	hasLetter := false
	for _, r := range line {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	return hasLetter
}

// tier1Heading reports whether block b is a chapter-title block: either a
// single isHeadingCandidate line, or two lines where the first is a
// candidate and the second is a Setext "===" underline.
func tier1Heading(b block, opts Options) (title string, ok bool) {
	switch len(b.lines) {
	case 1:
		if isHeadingCandidate(b.lines[0], opts) {
			return strings.TrimSpace(b.lines[0]), true
		}
	case 2:
		if isHeadingCandidate(b.lines[0], opts) && setextEqualsRe.MatchString(strings.TrimSpace(b.lines[1])) {
			return strings.TrimSpace(b.lines[0]), true
		}
	}
	return "", false
}

// tier2Heading reports whether block b is an in-chapter subheading: line 1
// a heading candidate, line 2 a Setext "---" underline. This is
// deliberately the *only* in-chapter heading signal — an isolated short
// line without the explicit "---" marker is indistinguishable from an
// ordinarily soft-wrapped paragraph's first line, so it is never promoted.
func tier2Heading(b block, opts Options) (title string, ok bool) {
	if len(b.lines) == 2 && isHeadingCandidate(b.lines[0], opts) && setextDashesRe.MatchString(strings.TrimSpace(b.lines[1])) {
		return strings.TrimSpace(b.lines[0]), true
	}
	return "", false
}

// rawChapter is one detected chapter before frontmatter/id assignment.
// Title is "" for a chapter that opens a file with no Tier-1 heading at
// all (a "cold start") — Convert falls back to a filename-derived title
// for those.
type rawChapter struct {
	Title  string
	Blocks []block
}

// splitChapters splits raw into chapters at each Tier-1 heading. cur starts
// nil, so both "the file opens with a heading" and "the file has no
// heading at all" fall out of the same loop with no special-casing: the
// first Tier-1 match (if any) closes the implicit cold-start chapter by
// simply never having added anything to it, or a truly headingless file
// leaves exactly one chapter with an empty Title.
func splitChapters(raw string, opts Options) []rawChapter {
	blocks := splitBlankLineBlocks(raw)

	var chapters []rawChapter
	var cur *rawChapter
	for _, b := range blocks {
		if title, ok := tier1Heading(b, opts); ok {
			chapters = append(chapters, rawChapter{Title: title})
			cur = &chapters[len(chapters)-1]
			continue
		}
		if cur == nil {
			chapters = append(chapters, rawChapter{})
			cur = &chapters[len(chapters)-1]
		}
		cur.Blocks = append(cur.Blocks, b)
	}
	return chapters
}

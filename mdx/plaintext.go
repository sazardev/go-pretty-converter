package mdx

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// txtPrefixRe extracts a leading numeric prefix from a .txt base filename
// (e.g. "01-notas" -> major 1, rest "notas"), mirroring the "NN-slug"
// convention the project already uses for numbering .mdx docs.
var txtPrefixRe = regexp.MustCompile(`^(\d+)[-_.\s]+(.+)$`)

// blankLineRe splits plain text into paragraphs on one or more blank lines.
var blankLineRe = regexp.MustCompile(`\n{2,}`)

func splitTxtName(base string) (major int, rest string, ok bool) {
	m := txtPrefixRe.FindStringSubmatch(base)
	if m == nil {
		return 0, "", false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, "", false
	}
	return n, m[2], true
}

// humanizeTitle turns a filename fragment like "getting-started" into
// "Getting Started" for use as an auto-generated document title.
func humanizeTitle(name string) string {
	name = strings.NewReplacer("-", " ", "_", " ").Replace(strings.TrimSpace(name))
	words := strings.Fields(name)
	if len(words) == 0 {
		return "Untitled"
	}
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// renderPlainText converts freeform .txt content into literal, escaped
// HTML: blank lines separate paragraphs and single line breaks become
// <br>. Unlike .md/.mdx, the text is always HTML-escaped rather than
// passed through raw — a .txt file has no way to opt into unsafe HTML, and
// its whole appeal is "write whatever you want" without needing to think
// about markup at all.
func renderPlainText(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	if raw == "" {
		return ""
	}
	var b strings.Builder
	for _, para := range blankLineRe.Split(raw, -1) {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		escaped := html.EscapeString(para)
		escaped = strings.ReplaceAll(escaped, "\n", "<br>\n")
		b.WriteString("<p>")
		b.WriteString(escaped)
		b.WriteString("</p>\n")
	}
	return b.String()
}

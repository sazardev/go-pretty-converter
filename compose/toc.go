package compose

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/sazardev/go-pretty-converter/mdx"
)

func buildTOC(docs []*mdx.Document) string {
	var buf bytes.Buffer
	buf.WriteString(`<div class="toc">` + "\n")
	buf.WriteString(`<h1>Table of Contents</h1>` + "\n")

	seenH1 := make(map[int]bool)

	for _, d := range docs {
		parts := mdx.SplitID(d.ID())
		if len(parts) < 3 {
			continue
		}

		h1Key := parts[0]
		isH1 := parts[1] == 0 && parts[2] == 0
		isH2 := parts[2] == 0 && !isH1
		isH3 := parts[2] != 0

		link := fmt.Sprintf(`<a href="#%s">%s %s</a>`,
			template.HTMLEscapeString(mdx.AnchorID(d.ID())),
			template.HTMLEscapeString(d.ID()),
			template.HTMLEscapeString(d.Title()))

		if isH1 && !seenH1[h1Key] {
			seenH1[h1Key] = true
			fmt.Fprintf(&buf, `<div class="toc-h1">%s</div>`+"\n", link)
		} else if isH2 {
			fmt.Fprintf(&buf, `<div class="toc-h2">%s</div>`+"\n", link)
		} else if isH3 {
			fmt.Fprintf(&buf, `<div class="toc-h3">%s</div>`+"\n", link)
		} else {
			fmt.Fprintf(&buf, `<div class="toc-h1">%s</div>`+"\n", link)
		}
	}

	buf.WriteString("</div>\n")
	return buf.String()
}

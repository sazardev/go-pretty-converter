package epub

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// flattenCodeBlocks strips syntax-highlighting token spans out of every
// <pre> block in an HTML fragment, leaving a single plain <pre><code>
// with the code's text — exactly what goldmark produces for a code fence
// without the highlighting extension. It is the difference between a
// handful of elements per code block and hundreds of nested spans (Chroma
// emits one span per token, including whitespace-only `chroma-w` spans):
// when Calibre converts the EPUB to MOBI/AZW3 it rewrites every text
// fragment with a KF8 `aid` anchor span, so span-dense code blocks come
// out thousands of aid-wrapped fragments that old Kindle renderers mangle
// into visible raw markup (`class="chroma-w" aid="4GULHE">`) and spliced
// words. Plain code keeps the blocks readable on any Kindle, at the cost
// of losing syntax colors in the Kindle output (the PDF and EPUB builds
// keep highlighting).
func flattenCodeBlocks(fragment string) (string, error) {
	context := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(fragment), context)
	if err != nil {
		return "", fmt.Errorf("parsing fragment to flatten code blocks: %w", err)
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "pre" {
			var text strings.Builder
			collectText(n, &text)
			n.FirstChild = nil
			n.LastChild = nil
			code := &html.Node{Type: html.ElementNode, Data: "code"}
			n.AppendChild(code)
			code.AppendChild(&html.Node{Type: html.TextNode, Data: text.String()})
			return // flattened content needs no further descent
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	var buf bytes.Buffer
	for _, n := range nodes {
		if err := html.Render(&buf, n); err != nil {
			return "", fmt.Errorf("rendering flattened fragment: %w", err)
		}
	}
	return buf.String(), nil
}

func collectText(n *html.Node, buf *strings.Builder) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			buf.WriteString(c.Data)
		} else {
			collectText(c, buf)
		}
	}
}

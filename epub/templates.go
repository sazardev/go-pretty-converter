package epub

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html/template"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sazardev/go-pretty-converter/mdx"
)

// navItem is one entry in the nested table of contents built from
// chapters' frontmatter IDs (see buildNavTree) — shared by both nav.xhtml
// (EPUB 3's required nav document) and toc.ncx (the EPUB 2 fallback,
// flattened — see renderNCX).
type navItem struct {
	Label    string
	File     string
	Children []*navItem
}

// buildNavTree groups chapters into a 3-level tree from their [X.Y.Z]
// frontmatter ID, mirroring compose.buildTOC's H1/H2/H3 classification
// (Y==0&&Z==0 is a top-level entry, Z==0 is its child, anything else is a
// grandchild) but as real nesting instead of a flat, indent-styled list —
// EPUB reading systems render nav.xhtml as a collapsible outline, so a
// flat list would lose the document's actual structure. A chapter whose
// ID doesn't match the pattern still gets a top-level entry rather than
// being dropped, unlike compose.buildTOC's TOC-page rendering: the nav
// document is the *only* way to jump to a chapter in an EPUB (there's no
// page-number fallback), so nothing can be silently unreachable from it.
func buildNavTree(chapters []chapterInfo) []*navItem {
	var tree []*navItem
	var lastH1, lastH2 *navItem

	for _, ch := range chapters {
		item := &navItem{Label: chapterLabel(ch.doc), File: ch.file}

		parts := mdx.SplitID(ch.doc.ID())
		isH1 := len(parts) < 3 || (parts[1] == 0 && parts[2] == 0)
		isH2 := len(parts) >= 3 && parts[2] == 0 && !isH1

		switch {
		case isH1:
			tree = append(tree, item)
			lastH1, lastH2 = item, nil
		case isH2:
			if lastH1 == nil {
				tree = append(tree, item)
				lastH1 = item
				continue
			}
			lastH1.Children = append(lastH1.Children, item)
			lastH2 = item
		default: // H3
			switch {
			case lastH2 != nil:
				lastH2.Children = append(lastH2.Children, item)
			case lastH1 != nil:
				lastH1.Children = append(lastH1.Children, item)
			default:
				tree = append(tree, item)
			}
		}
	}

	return tree
}

// flattenNavTree walks tree depth-first, the order toc.ncx's flat navMap
// (see renderNCX) presents chapters in.
func flattenNavTree(tree []*navItem) []*navItem {
	var flat []*navItem
	var walk func([]*navItem)
	walk = func(items []*navItem) {
		for _, it := range items {
			flat = append(flat, it)
			walk(it.Children)
		}
	}
	walk(tree)
	return flat
}

func chapterLabel(doc *mdx.Document) string {
	if id := doc.ID(); id != "" {
		return id + " " + doc.Title()
	}
	return doc.Title()
}

// renderNavList renders tree as nested <ol>/<li> markup, pre-escaping
// labels itself (via template.HTMLEscapeString) since it's built with
// plain string concatenation rather than html/template's own contextual
// escaping.
func renderNavList(items []*navItem) template.HTML {
	if len(items) == 0 {
		return ""
	}
	var buf bytes.Buffer
	buf.WriteString("<ol>")
	for _, it := range items {
		buf.WriteString("<li><a href=\"text/")
		buf.WriteString(template.HTMLEscapeString(it.File))
		buf.WriteString("\">")
		buf.WriteString(template.HTMLEscapeString(it.Label))
		buf.WriteString("</a>")
		buf.WriteString(string(renderNavList(it.Children)))
		buf.WriteString("</li>")
	}
	buf.WriteString("</ol>")
	return template.HTML(buf.String())
}

// xmlProlog is prepended to every XHTML file's rendered output rather
// than living inside the template source strings below: html/template's
// HTML tokenizer doesn't recognize "<?xml ... ?>" as a processing
// instruction (it's not valid HTML), and — surprisingly — ends up
// HTML-escaping its leading "<" as "&lt;" when it appears as static
// template text, corrupting the declaration. Keeping it out of the
// template and prepending it to the executed result sidesteps the bug
// entirely.
const xmlProlog = `<?xml version="1.0" encoding="utf-8"?>` + "\n"

const chapterTemplateSrc = `<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" lang="{{.Lang}}">
<head>
<meta charset="utf-8"/>
<title>{{.Title}}</title>
<link rel="stylesheet" type="text/css" href="../css/style.css"/>
</head>
<body>
<section id="{{.AnchorID}}">
{{.Body}}
</section>
</body>
</html>
`

var chapterTemplate = template.Must(template.New("chapter").Parse(chapterTemplateSrc))

// xhtmlifyFragment reparses an HTML fragment and re-serializes it through
// x/net/html so it's safe to embed in a document declared
// application/xhtml+xml. goldmark's output (rendered with WithUnsafe, so
// raw HTML an MDX author types by hand — <br>, &nbsp;, &mdash; — passes
// through untouched) is only guaranteed to be HTML5-lenient-parser-safe,
// not well-formed XML: void elements may be unclosed, and named entities
// like &nbsp; aren't defined without a DTD. x/net/html's parser resolves
// entities to literal Unicode text and its renderer always self-closes
// void elements and only emits the five XML-legal escapes, so the
// round-trip produces markup a strict EPUB reader can actually parse.
func xhtmlifyFragment(htmlFragment string) (string, error) {
	context := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(htmlFragment), context)
	if err != nil {
		return "", fmt.Errorf("parsing chapter HTML: %w", err)
	}
	var buf bytes.Buffer
	for _, n := range nodes {
		if err := html.Render(&buf, n); err != nil {
			return "", fmt.Errorf("rendering chapter XHTML: %w", err)
		}
	}
	return buf.String(), nil
}

func renderChapterXHTML(opts Options, ch chapterInfo) (string, error) {
	body := ch.doc.HTML
	if opts.PlainCode {
		flattened, err := flattenCodeBlocks(body)
		if err != nil {
			return "", fmt.Errorf("chapter %q: %w", ch.doc.ID(), err)
		}
		body = flattened
	}
	renderedBody, err := xhtmlifyFragment(body)
	if err != nil {
		return "", fmt.Errorf("chapter %q: %w", ch.doc.ID(), err)
	}

	var buf bytes.Buffer
	buf.WriteString(xmlProlog)
	err = chapterTemplate.Execute(&buf, struct {
		Lang     string
		Title    string
		AnchorID string
		Body     template.HTML
	}{
		Lang:     opts.Language,
		Title:    ch.doc.Title(),
		AnchorID: mdx.AnchorID(ch.doc.ID()),
		Body:     template.HTML(renderedBody),
	})
	return buf.String(), err
}

const coverTemplateSrc = `<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" lang="{{.Lang}}">
<head>
<meta charset="utf-8"/>
<title>{{.Title}}</title>
<style>html,body{margin:0;padding:0;}img{display:block;width:100%;height:100%;}</style>
</head>
<body><img src="../images/{{.CoverFile}}" alt="Cover"/></body>
</html>
`

var coverTemplate = template.Must(template.New("cover").Parse(coverTemplateSrc))

func renderCoverXHTML(opts Options, coverFile string) (string, error) {
	var buf bytes.Buffer
	buf.WriteString(xmlProlog)
	err := coverTemplate.Execute(&buf, struct {
		Lang      string
		Title     string
		CoverFile string
	}{
		Lang:      opts.Language,
		Title:     opts.Title,
		CoverFile: coverFile,
	})
	if err != nil {
		return "", fmt.Errorf("rendering cover: %w", err)
	}
	return buf.String(), nil
}

const navTemplateSrc = `<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="{{.Lang}}">
<head>
<meta charset="utf-8"/>
<title>{{.Title}}</title>
</head>
<body>
<nav epub:type="toc" id="toc">
<h1>Table of Contents</h1>
{{.List}}
</nav>
</body>
</html>
`

var navTemplate = template.Must(template.New("nav").Parse(navTemplateSrc))

func renderNavXHTML(opts Options, tree []*navItem) (string, error) {
	var buf bytes.Buffer
	buf.WriteString(xmlProlog)
	err := navTemplate.Execute(&buf, struct {
		Lang  string
		Title string
		List  template.HTML
	}{
		Lang:  opts.Language,
		Title: opts.Title,
		List:  renderNavList(tree),
	})
	if err != nil {
		return "", fmt.Errorf("rendering nav: %w", err)
	}
	return buf.String(), nil
}

// xmlEscape escapes s for use inside XML text or attribute content — used
// for content.opf and toc.ncx, which aren't HTML and so don't go through
// html/template's HTML-aware contextual autoescaping the way the XHTML
// templates above do.
func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func renderNCX(opts Options, uuid string, chapters []chapterInfo) string {
	tree := buildNavTree(chapters)
	flat := flattenNavTree(tree)

	var buf strings.Builder
	buf.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	buf.WriteString(`<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">` + "\n")
	buf.WriteString("<head>\n")
	fmt.Fprintf(&buf, `<meta name="dtb:uid" content="urn:uuid:%s"/>`+"\n", uuid)
	buf.WriteString(`<meta name="dtb:depth" content="1"/>` + "\n")
	buf.WriteString(`<meta name="dtb:totalPageCount" content="0"/>` + "\n")
	buf.WriteString(`<meta name="dtb:maxPageNumber" content="0"/>` + "\n")
	buf.WriteString("</head>\n")
	fmt.Fprintf(&buf, "<docTitle><text>%s</text></docTitle>\n", xmlEscape(opts.Title))
	buf.WriteString("<navMap>\n")
	for i, item := range flat {
		fmt.Fprintf(&buf, `<navPoint id="navPoint-%d" playOrder="%d">`+"\n", i+1, i+1)
		fmt.Fprintf(&buf, "<navLabel><text>%s</text></navLabel>\n", xmlEscape(item.Label))
		fmt.Fprintf(&buf, `<content src="text/%s"/>`+"\n", xmlEscape(item.File))
		buf.WriteString("</navPoint>\n")
	}
	buf.WriteString("</navMap>\n")
	buf.WriteString("</ncx>\n")
	return buf.String()
}

func renderOPF(opts Options, uuid, modified string, chapters []chapterInfo, coverFile, coverMediaType string) string {
	hasCover := coverFile != ""

	var buf strings.Builder
	buf.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	fmt.Fprintf(&buf, `<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id" xml:lang="%s">`+"\n", xmlEscape(opts.Language))

	buf.WriteString(`<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` + "\n")
	fmt.Fprintf(&buf, `<dc:identifier id="pub-id">urn:uuid:%s</dc:identifier>`+"\n", uuid)
	fmt.Fprintf(&buf, "<dc:title>%s</dc:title>\n", xmlEscape(opts.Title))
	fmt.Fprintf(&buf, "<dc:language>%s</dc:language>\n", xmlEscape(opts.Language))
	if opts.Author != "" {
		fmt.Fprintf(&buf, "<dc:creator>%s</dc:creator>\n", xmlEscape(opts.Author))
	}
	if opts.Subtitle != "" {
		fmt.Fprintf(&buf, "<dc:description>%s</dc:description>\n", xmlEscape(opts.Subtitle))
	}
	fmt.Fprintf(&buf, `<meta property="dcterms:modified">%s</meta>`+"\n", modified)
	if hasCover {
		buf.WriteString(`<meta name="cover" content="cover-img"/>` + "\n")
	}
	buf.WriteString("</metadata>\n")

	buf.WriteString("<manifest>\n")
	buf.WriteString(`<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>` + "\n")
	buf.WriteString(`<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>` + "\n")
	buf.WriteString(`<item id="style" href="css/style.css" media-type="text/css"/>` + "\n")
	if hasCover {
		fmt.Fprintf(&buf, `<item id="cover-img" href="images/%s" media-type="%s" properties="cover-image"/>`+"\n", xmlEscape(coverFile), xmlEscape(coverMediaType))
		buf.WriteString(`<item id="cover-page" href="text/cover.xhtml" media-type="application/xhtml+xml"/>` + "\n")
	}
	for _, ch := range chapters {
		fmt.Fprintf(&buf, `<item id="%s" href="text/%s" media-type="application/xhtml+xml"/>`+"\n", xmlEscape(ch.id), xmlEscape(ch.file))
	}
	buf.WriteString("</manifest>\n")

	buf.WriteString(`<spine toc="ncx">` + "\n")
	if hasCover {
		buf.WriteString(`<itemref idref="cover-page" linear="yes"/>` + "\n")
	}
	for _, ch := range chapters {
		fmt.Fprintf(&buf, `<itemref idref="%s"/>`+"\n", xmlEscape(ch.id))
	}
	buf.WriteString("</spine>\n")

	if hasCover {
		buf.WriteString("<guide>\n")
		buf.WriteString(`<reference type="cover" title="Cover" href="text/cover.xhtml"/>` + "\n")
		buf.WriteString("</guide>\n")
	}

	buf.WriteString("</package>\n")
	return buf.String()
}

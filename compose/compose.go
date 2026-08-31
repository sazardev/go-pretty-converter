package compose

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/sazardev/go-pretty-converter/mdx"
)

//go:embed assets/template.html
var defaultTemplate string

//go:embed assets/print.css
var defaultCSS string

type Options struct {
	Title     string
	Subtitle  string
	Author    string
	Template  string
	CSS       string
	ShowCover bool
	ShowTOC   bool
}

func DefaultOptions() Options {
	return Options{
		Title:     "Document",
		Subtitle:  "",
		Author:    "go-pretty-converter",
		ShowCover: true,
		ShowTOC:   true,
	}
}

func ComposeHTML(docs []*mdx.Document, opts Options) (string, error) {
	tmplHTML := opts.Template
	if tmplHTML == "" {
		tmplHTML = defaultTemplate
	}
	css := opts.CSS
	if css == "" {
		css = defaultCSS
	}

	tmpl, err := template.New("book").Parse(tmplHTML)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	keywords := collectKeywords(docs)

	var bodyBuf bytes.Buffer
	if opts.ShowTOC {
		bodyBuf.WriteString(buildTOC(docs))
		bodyBuf.WriteString(`<div style="page-break-before:always; break-before:page;"></div>` + "\n")
	}

	for _, doc := range docs {
		fmt.Fprintf(&bodyBuf, `<section id="%s">`+"\n", template.HTMLEscapeString(mdx.AnchorID(doc.ID())))
		bodyBuf.WriteString(doc.HTML)
		bodyBuf.WriteString("</section>\n")
	}

	data := templateData{
		Title:     opts.Title,
		Subtitle:  opts.Subtitle,
		Author:    opts.Author,
		Keywords:  keywords,
		CSS:       template.CSS(escapeStyleBlockClose(css)),
		Body:      template.HTML(bodyBuf.String()),
		BuiltAt:   time.Now().Format("2006-01-02 15:04:05 UTC"),
		TotalDocs: len(docs),
		ShowCover: opts.ShowCover,
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return out.String(), nil
}

// styleBlockCloseRe matches the literal byte sequence that ends an HTML
// <style> element per the HTML parsing spec: "</" followed immediately by
// characters that could start the tag name "style" (case-insensitively),
// wherever it appears — no closing ">" is even required for the browser's
// tokenizer to leave raw-text mode. css is marked template.CSS below so
// html/template inserts it into <style>{{.CSS}}</style> completely
// unescaped, which is required for legitimate CSS to survive intact — but
// it also means css itself must never be allowed to contain that sequence,
// whether it came from a builtin theme, a user-authored .theme.yml's raw
// `css:` escape hatch, or a --css file. Real CSS has no legitimate use for
// a literal "</style" substring, so neutralizing it here can't break a
// well-formed stylesheet.
var styleBlockCloseRe = regexp.MustCompile(`(?i)</(style)`)

func escapeStyleBlockClose(css string) string {
	return styleBlockCloseRe.ReplaceAllString(css, `<\/$1`)
}

type templateData struct {
	Title     string
	Subtitle  string
	Author    string
	Keywords  string
	CSS       template.CSS
	Body      template.HTML
	BuiltAt   string
	TotalDocs int
	ShowCover bool
}

func collectKeywords(docs []*mdx.Document) string {
	seen := make(map[string]bool)
	var tags []string
	for _, d := range docs {
		for _, t := range d.Tags() {
			if !seen[t] {
				seen[t] = true
				tags = append(tags, t)
			}
		}
	}
	sort.Strings(tags)
	return strings.Join(tags, ", ")
}

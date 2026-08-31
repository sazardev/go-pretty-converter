// Package analyze statically inspects already-parsed MDX documents for
// content patterns that render poorly (or break outright) across the
// project's output formats — PDF, EPUB, and Kindle. It needs neither
// headless Chrome nor Calibre: every check runs directly over
// mdx.Document.HTML and the source directory on disk, so it can catch
// problems before a build ever starts.
//
// It complements, rather than replaces, two existing layers:
//   - mdx.Validator checks frontmatter/structure (required fields, id
//     format, duplicate ids, heading depth).
//   - render.AuditReport checks the actual rendered DOM/PDF bytes after a
//     Chrome render (overflow, contrast, broken PDF output, ...).
//
// analyze sits before both: a fast, static pass over content structure
// that generalizes across every output format instead of just PDF.
package analyze

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	_ "golang.org/x/image/webp"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sazardev/go-pretty-converter/mdx"
)

// Severity classifies how urgently an Issue should be addressed.
type Severity string

const (
	// SeverityError marks content that will actually break in at least
	// one output format (a dead link, a missing image file, ...).
	SeverityError Severity = "error"
	// SeverityWarning marks content that will render, but poorly, in at
	// least one output format (a table too wide for Kindle, a heading
	// level skip that breaks navigation, ...).
	SeverityWarning Severity = "warning"
	// SeverityInfo marks a suggestion: not broken, not necessarily bad,
	// but worth a second look for a better result across formats.
	SeverityInfo Severity = "info"
)

// Issue is one finding against a single document.
type Issue struct {
	// File is the document's source path, e.g. "book/02-intro.mdx".
	File string
	// DocID is the document's frontmatter [X.Y.Z] id, if any.
	DocID string
	// DocTitle is the document's frontmatter title, if any.
	DocTitle string
	// Check is a short, stable, kebab-case identifier for the rule that
	// produced this issue (e.g. "wide-table"), suitable for filtering or
	// suppressing in tooling.
	Check string
	// Severity is Error, Warning, or Info.
	Severity Severity
	// Message is a human-readable explanation of the problem and, where
	// useful, why it matters for PDF/EPUB/Kindle rendering specifically.
	Message string
}

// Options configures the analyzer's thresholds. Zero-value fields fall
// back to the defaults in DefaultOptions via applyDefaults.
type Options struct {
	// MaxTableColumns is the most columns a table can have before it's
	// flagged as likely to overflow/not reflow on narrow readers.
	MaxTableColumns int
	// MaxCodeLineLength is the longest a single code-block line can be
	// before it's flagged as likely to clip in PDF or wrap awkwardly on
	// EPUB/Kindle.
	MaxCodeLineLength int
	// MaxListDepth is the deepest a nested list can go before it's
	// flagged as likely to collapse into unreadable indentation on a
	// narrow e-reader screen.
	MaxListDepth int
	// LongChapterWords is the word count above which a document with no
	// subheadings (h2+) is flagged: since each MDX file becomes one
	// EPUB/Kindle chapter/nav entry, an undivided chapter this long has
	// no in-chapter navigation.
	LongChapterWords int
	// OversizedImageWidth is the pixel width above which a local raster
	// image is flagged as larger than useful for print or e-ink,
	// needlessly bloating output file size.
	OversizedImageWidth int
}

// DefaultOptions returns the thresholds analyze uses when a caller
// doesn't override them.
func DefaultOptions() Options {
	return applyDefaults(Options{})
}

func applyDefaults(opts Options) Options {
	if opts.MaxTableColumns <= 0 {
		opts.MaxTableColumns = 6
	}
	if opts.MaxCodeLineLength <= 0 {
		opts.MaxCodeLineLength = 100
	}
	if opts.MaxListDepth <= 0 {
		opts.MaxListDepth = 3
	}
	if opts.LongChapterWords <= 0 {
		opts.LongChapterWords = 3000
	}
	if opts.OversizedImageWidth <= 0 {
		opts.OversizedImageWidth = 3000
	}
	return opts
}

// headingLevels maps the heading atoms to their numeric level, since
// atom.Atom values aren't ordered the way h1..h6 conceptually are.
var headingLevels = map[atom.Atom]int{
	atom.H1: 1,
	atom.H2: 2,
	atom.H3: 3,
	atom.H4: 4,
	atom.H5: 5,
	atom.H6: 6,
}

// Analyze runs every check against docs (in the order given — callers
// typically pass mdx.Parser.ParseDir's already-sorted result) and returns
// every finding, grouped by document in that same order and sorted within
// a document by severity (errors first) then check name.
func Analyze(docs []*mdx.Document, opts Options) []Issue {
	opts = applyDefaults(opts)

	all := make([]Issue, 0, len(docs))
	for _, doc := range docs {
		all = append(all, analyzeDoc(doc, opts)...)
	}
	return all
}

func analyzeDoc(doc *mdx.Document, opts Options) []Issue {
	var issues []Issue
	emit := func(check string, sev Severity, format string, args ...any) {
		issues = append(issues, Issue{
			File:     doc.Path,
			DocID:    doc.ID(),
			DocTitle: doc.Title(),
			Check:    check,
			Severity: sev,
			Message:  fmt.Sprintf(format, args...),
		})
	}

	nodes, err := parseFragment(doc.HTML)
	if err != nil {
		// Malformed HTML isn't this analyzer's concern — goldmark's own
		// output is always well-formed; a parse failure here would be a
		// bug in this package's fragment handling, not the document's.
		return issues
	}

	ids := map[string]int{}
	var anchorTargets []string
	lastHeadingLevel := 0
	h1Count := 0
	headingCount := 0
	hasSubheading := false
	wordCount := 0

	var visit func(n *html.Node, listDepth int)
	visit = func(n *html.Node, listDepth int) {
		if n.Type == html.ElementNode {
			if id, ok := attrVal(n, "id"); ok && id != "" {
				ids[id]++
			}

			if lvl, ok := headingLevels[n.DataAtom]; ok {
				headingCount++
				if lvl >= 2 {
					hasSubheading = true
				}
				if lvl == 1 {
					h1Count++
				}
				if lastHeadingLevel > 0 && lvl > lastHeadingLevel+1 {
					emit("heading-level-skip", SeverityWarning,
						"heading jumps from h%d to h%d (%q) — a skipped level breaks the outline PDF bookmarks, EPUB nav, and Kindle's TOC all build from headings",
						lastHeadingLevel, lvl, strings.TrimSpace(textContent(n)))
				}
				lastHeadingLevel = lvl
			}

			switch n.DataAtom {
			case atom.A:
				if href, ok := attrVal(n, "href"); ok && strings.HasPrefix(href, "#") && len(href) > 1 {
					anchorTargets = append(anchorTargets, href[1:])
				}
			case atom.Img:
				checkImage(n, doc, opts, emit)
			case atom.Table:
				checkTable(n, opts, emit)
			case atom.Pre:
				checkCodeBlock(n, opts, emit)
			case atom.Ul, atom.Ol:
				listDepth++
				if listDepth == opts.MaxListDepth+1 {
					emit("deep-list-nesting", SeverityWarning,
						"list nested %d levels deep (recommended max %d) — deep nesting collapses to barely-visible indentation on Kindle's narrow display",
						listDepth, opts.MaxListDepth)
				}
			}
		}

		if n.Type == html.TextNode {
			wordCount += len(strings.Fields(n.Data))
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c, listDepth)
		}
	}

	for _, n := range nodes {
		visit(n, 0)
	}

	for _, target := range anchorTargets {
		if ids[target] == 0 {
			emit("broken-internal-anchor", SeverityError,
				"link points to \"#%s\" but no element with that id exists in this document", target)
		}
	}

	var dupIDs []string
	for id, count := range ids {
		if count > 1 {
			dupIDs = append(dupIDs, id)
		}
	}
	sort.Strings(dupIDs)
	for _, id := range dupIDs {
		emit("duplicate-element-id", SeverityError,
			"id %q is used on %d elements — breaks anchors and TOC/bookmark navigation, which key off unique ids", id, ids[id])
	}

	if h1Count > 1 {
		emit("multiple-h1", SeverityWarning,
			"document has %d top-level headings (h1) — each MDX file becomes one chapter, so multiple h1s produce an ambiguous chapter title in the EPUB/Kindle navigation", h1Count)
	}

	if headingCount == 0 && wordCount > 0 {
		emit("no-headings", SeverityInfo,
			"document has no headings — readers get no in-chapter navigation in PDF bookmarks, EPUB nav, or Kindle's TOC")
	}

	if wordCount >= opts.LongChapterWords && !hasSubheading {
		emit("long-chapter-no-subheadings", SeverityWarning,
			"chapter has ~%d words with no subheadings (h2+) — since each MDX file becomes one chapter/nav entry, a long undivided chapter has no in-chapter navigation on Kindle/EPUB; consider adding subheadings or splitting into more files",
			wordCount)
	}

	if len(doc.Tags()) == 0 {
		emit("no-tags", SeverityInfo, "document has no frontmatter tags — consider adding some for search/discoverability")
	}

	sort.SliceStable(issues, func(i, j int) bool {
		ri, rj := severityRank(issues[i].Severity), severityRank(issues[j].Severity)
		if ri != rj {
			return ri < rj
		}
		return issues[i].Check < issues[j].Check
	})

	return issues
}

func severityRank(s Severity) int {
	switch s {
	case SeverityError:
		return 0
	case SeverityWarning:
		return 1
	default:
		return 2
	}
}

func checkImage(n *html.Node, doc *mdx.Document, opts Options, emit func(check string, sev Severity, format string, args ...any)) {
	src, _ := attrVal(n, "src")
	if src == "" {
		return
	}
	alt, _ := attrVal(n, "alt")
	if strings.TrimSpace(alt) == "" {
		emit("image-missing-alt", SeverityWarning,
			"image %q has no alt text — needed for accessibility and for EPUB/Kindle readers (or screen readers) that can't display it", src)
	}

	switch {
	case strings.HasPrefix(src, "http://"), strings.HasPrefix(src, "https://"):
		emit("image-external-url", SeverityInfo,
			"image %q is a remote URL — PDF rendering blocks network access by default, so this will render blank unless network access is explicitly enabled, and most e-readers fetch nothing while offline either", src)
		return
	case strings.HasPrefix(src, "data:"):
		return
	}

	imgPath := src
	if !filepath.IsAbs(imgPath) {
		imgPath = filepath.Join(filepath.Dir(doc.Path), imgPath)
	}

	f, err := os.Open(imgPath)
	if err != nil {
		emit("image-file-not-found", SeverityError,
			"image %q could not be found at %s — this will break PDF, EPUB, and Kindle output", src, imgPath)
		return
	}
	defer func() { _ = f.Close() }()

	cfg, _, err := image.DecodeConfig(f)
	if err == nil && cfg.Width > opts.OversizedImageWidth {
		emit("image-oversized", SeverityInfo,
			"image %q is %dpx wide — print pages and e-ink Kindle displays rarely benefit from more than ~%dpx; consider downscaling to reduce output file size",
			src, cfg.Width, opts.OversizedImageWidth)
	}
}

func checkTable(n *html.Node, opts Options, emit func(check string, sev Severity, format string, args ...any)) {
	maxCols := 0
	var walkRows func(*html.Node)
	walkRows = func(row *html.Node) {
		if row.Type == html.ElementNode && row.DataAtom == atom.Tr {
			cols := 0
			for c := row.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.DataAtom == atom.Td || c.DataAtom == atom.Th) {
					cols++
				}
			}
			if cols > maxCols {
				maxCols = cols
			}
			return
		}
		for c := row.FirstChild; c != nil; c = c.NextSibling {
			walkRows(c)
		}
	}
	walkRows(n)

	if maxCols > opts.MaxTableColumns {
		emit("wide-table", SeverityWarning,
			"table has %d columns (recommended max %d) — wide tables don't reflow on Kindle's narrow e-ink display and often overflow or clip in PDF",
			maxCols, opts.MaxTableColumns)
	}
}

func checkCodeBlock(n *html.Node, opts Options, emit func(check string, sev Severity, format string, args ...any)) {
	for _, line := range strings.Split(textContent(n), "\n") {
		if length := utf8.RuneCountInString(line); length > opts.MaxCodeLineLength {
			emit("long-code-line", SeverityWarning,
				"code block has a line %d characters long (recommended max %d) — long lines clip in PDF and wrap awkwardly in EPUB/Kindle's reflowed, narrower view",
				length, opts.MaxCodeLineLength)
			return
		}
	}
}

func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

func attrVal(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

// parseFragment parses an HTML fragment (mdx.Document.HTML is a body
// fragment, not a full document) into its top-level nodes, using "body"
// as the parsing context per the html package's documented fragment API.
func parseFragment(htmlStr string) ([]*html.Node, error) {
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	return html.ParseFragment(strings.NewReader(htmlStr), body)
}

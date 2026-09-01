// Package format converts raw, unstructured .txt files into clean,
// structured .mdx files (proper [X.Y.Z] frontmatter, Markdown headings,
// lists, and fenced code blocks) ready for the existing mdx/compose/build
// pipeline. It is heuristic and deterministic — no AI/LLM dependency and no
// network access — and deliberately conservative: every detection rule is
// tuned to avoid false positives (misreading ordinary prose as structure)
// rather than to maximize how much structure it finds. See the package's
// heading.go doc comments for the specific tradeoffs.
//
// This package is upstream of, and does not depend on, mdx/compose/analyze:
// it only ever produces plain Markdown source text and frontmatter, which
// the existing mdx.Parser then ingests completely normally. Callers
// (cmd/pretty-converter's `format` command) are expected to run the
// existing `check`/`analyze`/`build` commands over the output afterward —
// this package never renders a PDF and never touches Chrome.
package format

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sazardev/go-pretty-converter/mdx"
)

// Options configures the heuristic detector. Zero-value fields fall back to
// sensible defaults in Convert via applyDefaults.
type Options struct {
	// StartID is the major id of the first detected chapter ([StartID.0.0]),
	// incrementing by one per chapter thereafter. Default 1.
	StartID int
	// MaxHeadingChars/MaxHeadingWords bound how long a line can be and
	// still plausibly read as a title rather than a sentence — see
	// isHeadingCandidate. Defaults 70 / 12.
	MaxHeadingChars int
	MaxHeadingWords int
}

func DefaultOptions() Options {
	return applyDefaults(Options{})
}

func applyDefaults(opts Options) Options {
	if opts.StartID <= 0 {
		opts.StartID = 1
	}
	if opts.MaxHeadingChars <= 0 {
		opts.MaxHeadingChars = 70
	}
	if opts.MaxHeadingWords <= 0 {
		opts.MaxHeadingWords = 12
	}
	return opts
}

// Chapter is one detected chapter's content and detection statistics,
// before frontmatter/filename assignment.
type Chapter struct {
	Title      string
	Body       string // rendered Markdown source, not HTML
	SourceFile string
	Headings   int // Tier-2 (##) subheadings detected
	Lists      int
	CodeBlocks int
	Paragraphs int
}

// Doc is a Chapter with its final frontmatter/filename assigned — Content
// is the complete file body (frontmatter block + Body) ready to write
// as-is to Filename.
type Doc struct {
	ID       string // "[N.0.0]"
	Title    string
	Filename string // "01-introduction.mdx"
	Content  string
	Chapter  Chapter
}

// Report is Convert's result: every detected document plus aggregate
// counts and an advisory (never auto-applied) theme suggestion.
type Report struct {
	Documents         []Doc
	TotalHeadings     int
	TotalLists        int
	TotalCodeBlocks   int
	TotalParagraphs   int
	SuggestedTheme    string // builtin theme name; "" if no confident signal
	SuggestedCategory string
}

var slugNonAlnumRe = regexp.MustCompile(`[^a-z0-9]+`)

// defaultSlug is the filename fragment slugify falls back to when title
// has no alphanumeric characters at all (e.g. "" or "---").
const defaultSlug = "untitled"

// slugify turns title into a lowercase, hyphenated filename fragment — the
// reverse direction of mdx.HumanizeTitle, e.g. "Getting Started!" ->
// "getting-started".
func slugify(title string) string {
	s := strings.ToLower(title)
	s = slugNonAlnumRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return defaultSlug
	}
	return s
}

// idWidth returns how many digits to zero-pad chapter numbers to, so
// filenames sort correctly regardless of book size (e.g. "02-..." before
// "10-..." rather than after).
func idWidth(maxID int) int {
	w := 1
	for maxID >= 10 {
		maxID /= 10
		w++
	}
	if w < 2 {
		w = 2
	}
	return w
}

type frontmatter struct {
	ID    string `yaml:"id"`
	Title string `yaml:"title"`
}

// renderDocument assembles a complete .mdx file: a YAML frontmatter block
// (via yaml.Marshal, so titles containing ":" or other YAML-significant
// characters are quoted correctly without any hand-rolled escaping) plus
// the rendered Markdown body.
func renderDocument(id, title, body string) string {
	fm, err := yaml.Marshal(frontmatter{ID: id, Title: title})
	if err != nil {
		// yaml.Marshal on two plain strings cannot realistically fail;
		// fall back to an equivalent hand-built block rather than
		// propagating an error from what is otherwise pure in-memory
		// formatting.
		fm = []byte(fmt.Sprintf("id: %q\ntitle: %q\n", id, title))
	}
	return "---\n" + string(fm) + "---\n\n" + body + "\n"
}

// Convert reads every path in paths (each a raw .txt file) and applies the
// heuristic structure detector, returning fully-formed Doc content ready
// to write to disk. Files are processed in the order given — callers
// resolve file-vs-directory input and sort paths themselves (mirrors how
// epub.Write/kindle.Write take pre-resolved input rather than doing their
// own directory walk).
func Convert(paths []string, opts Options) (*Report, error) {
	opts = applyDefaults(opts)

	type sourcedChapter struct {
		rawChapter
		sourceFile string
	}
	var all []sourcedChapter

	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		for _, ch := range splitChapters(string(raw), opts) {
			all = append(all, sourcedChapter{rawChapter: ch, sourceFile: path})
		}
	}

	report := &Report{}
	usedSlugs := map[string]int{}
	width := idWidth(opts.StartID + len(all) - 1)
	var signals contentSignals

	for i, ch := range all {
		title := ch.Title
		if title == "" {
			base := strings.TrimSuffix(filepath.Base(ch.sourceFile), filepath.Ext(ch.sourceFile))
			title = mdx.HumanizeTitle(base)
		}

		body, headings, lists, code, paragraphs := renderChapterBody(ch.Blocks, opts)
		signals.accumulate(title, ch.Blocks, headings, lists, code, paragraphs)

		slug := slugify(title)
		usedSlugs[slug]++
		if n := usedSlugs[slug]; n > 1 {
			slug = fmt.Sprintf("%s-%d", slug, n)
		}

		id := opts.StartID + i
		docID := fmt.Sprintf("[%d.0.0]", id)
		filename := fmt.Sprintf("%0*d-%s.mdx", width, id, slug)
		content := renderDocument(docID, title, body)

		report.Documents = append(report.Documents, Doc{
			ID:       docID,
			Title:    title,
			Filename: filename,
			Content:  content,
			Chapter: Chapter{
				Title:      title,
				Body:       body,
				SourceFile: ch.sourceFile,
				Headings:   headings,
				Lists:      lists,
				CodeBlocks: code,
				Paragraphs: paragraphs,
			},
		})

		report.TotalHeadings += headings
		report.TotalLists += lists
		report.TotalCodeBlocks += code
		report.TotalParagraphs += paragraphs
	}

	report.SuggestedTheme, report.SuggestedCategory = signals.suggestTheme()

	return report, nil
}

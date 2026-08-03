package mdx

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkHtml "github.com/yuin/goldmark/renderer/html"

	"github.com/sazardev/go-pretty-pdf/theme"
)

type ParseFileError struct {
	File string
	Err  error
}

func (e ParseFileError) Error() string {
	return fmt.Sprintf("%s: %v", e.File, e.Err)
}

func (e ParseFileError) Unwrap() error {
	return e.Err
}

type ParseErrors []ParseFileError

func (pe ParseErrors) Error() string {
	if len(pe) == 0 {
		return ""
	}
	if len(pe) == 1 {
		return pe[0].Error()
	}
	return fmt.Sprintf("%d file(s) failed to parse (first: %v)", len(pe), pe[0])
}

type Parser struct {
	md         goldmark.Markdown
	components *ComponentRegistry
	varsMu     sync.RWMutex
	vars       map[string]string
}

type ParserOption func(*Parser)

func WithComponent(name string, handler ComponentHandler) ParserOption {
	return func(p *Parser) {
		p.components.Register(name, handler)
	}
}

func WithVars(vars map[string]string) ParserOption {
	return func(p *Parser) {
		p.vars = vars
	}
}

func NewParser(opts ...ParserOption) *Parser {
	p := &Parser{
		md: goldmark.New(
			goldmark.WithExtensions(
				meta.New(meta.WithStoresInDocument()),
				extension.GFM,
				highlighting.NewHighlighting(
					highlighting.WithFormatOptions(
						chromahtml.WithClasses(true),
						chromahtml.ClassPrefix(theme.ChromaClassPrefix),
						chromahtml.WithLineNumbers(false),
					),
				),
			),
			goldmark.WithParserOptions(
				parser.WithAutoHeadingID(),
			),
			goldmark.WithRendererOptions(
				goldmarkHtml.WithUnsafe(),
				// XHTML-style self-closing void elements (<img/>, <hr/>,
				// <br/>) are still perfectly valid HTML5 — Chrome's print
				// pipeline doesn't care — but they're required for the
				// epub package's chapter files to be well-formed XHTML, so
				// this is set globally rather than per-consumer.
				goldmarkHtml.WithXHTML(),
			),
		),
		components: NewComponentRegistry(),
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func (p *Parser) RegisterComponent(name string, handler ComponentHandler) {
	p.components.Register(name, handler)
}

func (p *Parser) SetVars(vars map[string]string) {
	p.varsMu.Lock()
	defer p.varsMu.Unlock()
	p.vars = vars
}

func (p *Parser) ParseDir(dir string) ([]*Document, error) {
	var docFiles []string
	var txtFiles []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		switch {
		case strings.HasSuffix(name, ".mdx") || strings.HasSuffix(name, ".md"):
			docFiles = append(docFiles, path)
		case strings.HasSuffix(name, ".txt"):
			txtFiles = append(txtFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking source dir: %w", err)
	}

	if len(docFiles) == 0 && len(txtFiles) == 0 {
		return nil, fmt.Errorf("no .md, .mdx, or .txt files found in %s", dir)
	}

	var docs []*Document
	var parseErrs ParseErrors

	type pendingAutoDoc struct {
		path, html string
	}
	var pendingAuto []pendingAutoDoc

	for _, file := range docFiles {
		frontmatter, html, err := p.convert(file)
		if err != nil {
			parseErrs = append(parseErrs, ParseFileError{File: file, Err: err})
			continue
		}
		if frontmatter == nil {
			// No --- frontmatter block at all: fall back to the same
			// filename-based auto id/title as .txt, rather than failing.
			// A file with a malformed/partial frontmatter block still
			// reaches the validator and errors there, unchanged.
			pendingAuto = append(pendingAuto, pendingAutoDoc{path: file, html: html})
			continue
		}
		docs = append(docs, &Document{Path: file, Frontmatter: frontmatter, HTML: html})
	}

	if len(pendingAuto) > 0 {
		paths := make([]string, len(pendingAuto))
		htmlByPath := make(map[string]string, len(pendingAuto))
		for i, pa := range pendingAuto {
			paths[i] = pa.path
			htmlByPath[pa.path] = pa.html
		}
		for _, e := range assignAutoIDs(paths, docs) {
			docs = append(docs, &Document{
				Path: e.path,
				Frontmatter: map[string]interface{}{
					"id":    e.id,
					"title": e.title,
				},
				HTML: htmlByPath[e.path],
			})
		}
	}

	if len(txtFiles) > 0 {
		txtDocs, txtErrs := p.parseTextFiles(txtFiles, docs)
		docs = append(docs, txtDocs...)
		parseErrs = append(parseErrs, txtErrs...)
	}

	sortDocuments(docs)

	if len(parseErrs) > 0 {
		if len(docs) == 0 {
			return nil, parseErrs
		}
		return docs, parseErrs
	}

	return docs, nil
}

// autoEntry is a filename-derived id/title assignment for a file that has
// no way of specifying its own (no frontmatter block at all).
type autoEntry struct {
	path, id, title string
}

// assignAutoIDs auto-generates the frontmatter that a file without a ---
// block doesn't carry: a "NN-slug" name yields id "[NN.0.0]" and title
// "Slug" (mirroring the project's numbered-filename convention). Names
// without a numeric prefix get the next major version free after every id
// already in use — by existing/already-numbered documents passed in
// existing, and by other auto-numbered files in this same paths batch — so
// an author who just drops a file in never has to think about ids or
// collides with one that's explicitly numbered.
func assignAutoIDs(paths []string, existing []*Document) []autoEntry {
	sort.Strings(paths)

	entries := make([]autoEntry, len(paths))
	var pending []int

	maxMajor := 0
	for _, doc := range existing {
		if m := idExtractRe.FindStringSubmatch(doc.ID()); m != nil {
			if n := atoi(m[1]); n > maxMajor {
				maxMajor = n
			}
		}
	}

	for i, path := range paths {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if major, rest, ok := splitTxtName(base); ok {
			entries[i] = autoEntry{path: path, id: fmt.Sprintf("[%d.0.0]", major), title: humanizeTitle(rest)}
			if major > maxMajor {
				maxMajor = major
			}
			continue
		}
		entries[i] = autoEntry{path: path, title: humanizeTitle(base)}
		pending = append(pending, i)
	}

	nextMajor := maxMajor + 1
	for _, i := range pending {
		entries[i].id = fmt.Sprintf("[%d.0.0]", nextMajor)
		nextMajor++
	}
	return entries
}

// parseTextFiles auto-generates the frontmatter that plain .txt files don't
// carry (see assignAutoIDs) and renders their content as literal, escaped
// text (see renderPlainText) rather than markdown.
func (p *Parser) parseTextFiles(paths []string, existing []*Document) ([]*Document, ParseErrors) {
	var docs []*Document
	var errs ParseErrors
	for _, e := range assignAutoIDs(paths, existing) {
		doc, err := p.parseTextFile(e.path, e.id, e.title)
		if err != nil {
			errs = append(errs, ParseFileError{File: e.path, Err: err})
			continue
		}
		docs = append(docs, doc)
	}
	return docs, errs
}

func (p *Parser) parseTextFile(path, id, title string) (*Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	raw = p.substituteVars(raw)

	return &Document{
		Path: path,
		Frontmatter: map[string]interface{}{
			"id":    id,
			"title": title,
		},
		HTML: renderPlainText(string(raw)),
	}, nil
}

// convert reads and renders a .md/.mdx file, returning its frontmatter and
// rendered HTML. frontmatter is nil only when the file has no --- block at
// all; a block that's present but fails to parse as YAML is a hard error
// (via meta.TryGet, which — unlike meta.Get — distinguishes "no block" from
// "malformed block" instead of returning nil for both) rather than being
// silently treated the same as a file with no frontmatter.
func (p *Parser) convert(path string) (frontmatter map[string]interface{}, html string, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("reading %s: %w", path, err)
	}

	raw = p.substituteVars(raw)

	ctx := parser.NewContext()
	var buf bytes.Buffer

	if err = p.md.Convert(raw, &buf, parser.WithContext(ctx)); err != nil {
		return nil, "", fmt.Errorf("parsing %s: %w", path, err)
	}

	frontmatter, err = meta.TryGet(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("parsing frontmatter in %s: %w", path, err)
	}
	html = p.components.Transpile(buf.String())
	return frontmatter, html, nil
}

// ParseFile parses a single .md/.mdx file, requiring an explicit ---
// frontmatter block (unlike ParseDir, it has no directory context to
// auto-number a file that has none).
func (p *Parser) ParseFile(path string) (*Document, error) {
	frontmatter, html, err := p.convert(path)
	if err != nil {
		return nil, err
	}
	if frontmatter == nil {
		return nil, fmt.Errorf("%s: missing frontmatter", path)
	}

	return &Document{
		Path:        path,
		Frontmatter: frontmatter,
		HTML:        html,
	}, nil
}

func (p *Parser) ParseAll(paths []string) ([]*Document, error) {
	var docs []*Document
	var parseErrs ParseErrors

	for _, path := range paths {
		doc, err := p.ParseFile(path)
		if err != nil {
			parseErrs = append(parseErrs, ParseFileError{File: path, Err: err})
			continue
		}
		docs = append(docs, doc)
	}

	sortDocuments(docs)

	if len(parseErrs) > 0 {
		if len(docs) == 0 {
			return nil, parseErrs
		}
		return docs, parseErrs
	}

	return docs, nil
}

// substituteVars replaces every {{key}} placeholder with its configured
// value in a single pass over raw via strings.Replacer, rather than looping
// over the vars map and doing a sequential ReplaceAll per key. Two things
// would otherwise go wrong: map iteration order is randomized in Go, and a
// sequential ReplaceAll rescans the *already-substituted* text on each
// iteration, so one var's value can itself contain another var's
// placeholder and get expanded again — making the result depend on
// iteration order and change from run to run for identical input. A single
// Replacer pass finds all matches against the original text and expands
// each exactly once, so the result is deterministic and placeholders never
// chain into each other.
func (p *Parser) substituteVars(raw []byte) []byte {
	p.varsMu.RLock()
	vars := p.vars
	p.varsMu.RUnlock()

	if len(vars) == 0 {
		return raw
	}
	pairs := make([]string, 0, len(vars)*2)
	for k, v := range vars {
		pairs = append(pairs, "{{"+k+"}}", v)
	}
	return []byte(strings.NewReplacer(pairs...).Replace(string(raw)))
}

func sortDocuments(docs []*Document) {
	sort.Slice(docs, func(i, j int) bool {
		ai := docs[i].SortKey()
		aj := docs[j].SortKey()
		if ai[0] != aj[0] {
			return ai[0] < aj[0]
		}
		if ai[1] != aj[1] {
			return ai[1] < aj[1]
		}
		return ai[2] < aj[2]
	})
}

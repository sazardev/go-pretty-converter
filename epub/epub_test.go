package epub

import (
	"archive/zip"
	"encoding/xml"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sazardev/go-pretty-converter/mdx"
)

func mustParseDoc(t *testing.T, dir, filename, id, title, body string) *mdx.Document {
	t.Helper()
	content := "---\nid: \"" + id + "\"\ntitle: " + title + "\n---\n\n" + body + "\n"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := mdx.NewParser().ParseFile(path)
	if err != nil {
		t.Fatalf("parsing fixture %s: %v", filename, err)
	}
	return doc
}

// assertWellFormedXML confirms data is well-formed XML (the level EPUB
// reading systems require of every .xhtml/.opf/.ncx file) using the
// standard library's own decoder rather than a schema-validating parser —
// enough to catch the class of bug this package actually risks (a broken
// template producing mismatched tags or an unescaped "&"), without a new
// dependency.
func assertWellFormedXML(t *testing.T, name string, data []byte) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Errorf("%s is not well-formed XML: %v", name, err)
			return
		}
	}
}

func readZip(t *testing.T, path string) map[string][]byte {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("opening EPUB as zip: %v", err)
	}
	defer func() { _ = r.Close() }()

	files := make(map[string][]byte)
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("opening zip entry %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("reading zip entry %s: %v", f.Name, err)
		}
		files[f.Name] = data
	}
	return files
}

func writeTestCoverPNG(t *testing.T, dir string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	path := filepath.Join(dir, "cover.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWriteMimetypeFirstAndStored(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Chapter One", "# Chapter One\n\nHello.")
	outPath := filepath.Join(dir, "out.epub")

	if err := Write([]*mdx.Document{doc}, DefaultOptions(), outPath); err != nil {
		t.Fatalf("Write: %v", err)
	}

	r, err := zip.OpenReader(outPath)
	if err != nil {
		t.Fatalf("opening EPUB as zip: %v", err)
	}
	defer func() { _ = r.Close() }()

	if len(r.File) == 0 {
		t.Fatal("expected a non-empty zip")
	}
	first := r.File[0]
	if first.Name != "mimetype" {
		t.Errorf("expected first zip entry to be 'mimetype', got %q", first.Name)
	}
	if first.Method != zip.Store {
		t.Errorf("expected mimetype entry to be stored uncompressed (Method=%d), got Method=%d", zip.Store, first.Method)
	}

	rc, err := first.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "application/epub+zip" {
		t.Errorf("mimetype content = %q, want 'application/epub+zip'", data)
	}
}

func TestWriteAllXMLFilesAreWellFormed(t *testing.T) {
	dir := t.TempDir()
	docs := []*mdx.Document{
		mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Overview & Intro", "# Overview & Intro\n\nSome <b>raw</b> HTML, an image ![alt](x.png), and a rule.\n\n---\n\nMore text."),
		mustParseDoc(t, dir, "b.mdx", "[1.1.0]", "Details", "# Details\n\nMore content here."),
	}
	outPath := filepath.Join(dir, "out.epub")

	opts := DefaultOptions()
	opts.Title = "Test & Book"
	opts.Author = "A. Uthor"
	opts.CoverImage = writeTestCoverPNG(t, dir, 400, 600)

	if err := Write(docs, opts, outPath); err != nil {
		t.Fatalf("Write: %v", err)
	}

	files := readZip(t, outPath)
	checked := 0
	for name, data := range files {
		if strings.HasSuffix(name, ".xhtml") || strings.HasSuffix(name, ".opf") || strings.HasSuffix(name, ".ncx") || strings.HasSuffix(name, ".xml") {
			assertWellFormedXML(t, name, data)
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("expected at least one XML/XHTML file to check")
	}
}

// TestWriteHandlesRawUnclosedVoidElementsAndEntities guards a real bug: an
// MDX author typing raw HTML in their markdown (goldmark's WithUnsafe
// passes it through untouched) can write an unclosed void element like
// <br> or an HTML named entity like &nbsp; that's undefined in XML without
// a DTD. Chrome's lenient HTML parser tolerates both when rendering the
// PDF, but a chapter emitted verbatim into an XHTML file breaks the well-
// formedness EPUB readers require. xhtmlifyFragment (templates.go) exists
// to fix this by reparsing/re-serializing through x/net/html before the
// chapter template runs.
func TestWriteHandlesRawUnclosedVoidElementsAndEntities(t *testing.T) {
	dir := t.TempDir()
	docs := []*mdx.Document{
		mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Raw HTML",
			"# Raw HTML\n\nLine one.<br>Line two with a&nbsp;non-breaking space and an em&mdash;dash.\n"),
	}
	outPath := filepath.Join(dir, "out.epub")

	if err := Write(docs, DefaultOptions(), outPath); err != nil {
		t.Fatalf("Write: %v", err)
	}

	files := readZip(t, outPath)
	chapter, ok := files["OEBPS/text/ch0001.xhtml"]
	if !ok {
		t.Fatal("expected OEBPS/text/ch0001.xhtml in the EPUB")
	}
	assertWellFormedXML(t, "OEBPS/text/ch0001.xhtml", chapter)

	if strings.Contains(string(chapter), "&nbsp;") || strings.Contains(string(chapter), "&mdash;") {
		t.Errorf("expected named entities to be resolved to literal characters, got:\n%s", chapter)
	}
}

func TestWriteSpineAndTOCOrderMatchesDocOrder(t *testing.T) {
	dir := t.TempDir()
	docs := []*mdx.Document{
		mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "First", "# First"),
		mustParseDoc(t, dir, "b.mdx", "[1.1.0]", "Second", "# Second"),
		mustParseDoc(t, dir, "c.mdx", "[2.0.0]", "Third", "# Third"),
	}
	outPath := filepath.Join(dir, "out.epub")

	if err := Write(docs, DefaultOptions(), outPath); err != nil {
		t.Fatalf("Write: %v", err)
	}

	files := readZip(t, outPath)
	opf := string(files["OEBPS/content.opf"])

	// The spine's itemref order must be ch0001, ch0002, ch0003 — the same
	// order the docs were given in (mdx.Parser.ParseDir sorts by ID
	// before Write ever sees them; Write must preserve that order rather
	// than re-sorting).
	i1 := strings.Index(opf, `idref="ch0001"`)
	i2 := strings.Index(opf, `idref="ch0002"`)
	i3 := strings.Index(opf, `idref="ch0003"`)
	if i1 < 0 || i2 < 0 || i3 < 0 {
		t.Fatalf("expected all three chapters in the spine, got:\n%s", opf)
	}
	if i1 >= i2 || i2 >= i3 {
		t.Errorf("expected spine order ch0001 < ch0002 < ch0003, got offsets %d, %d, %d", i1, i2, i3)
	}

	nav := string(files["OEBPS/nav.xhtml"])
	// [1.1.0] is a child of [1.0.0] in the nav tree, so it must be nested
	// inside an <ol> that comes after ch0001's own <li>, not a sibling.
	if !strings.Contains(nav, `<a href="text/ch0001.xhtml">[1.0.0] First</a><ol><li><a href="text/ch0002.xhtml">[1.1.0] Second</a>`) {
		t.Errorf("expected [1.1.0] to nest under [1.0.0] in nav.xhtml, got:\n%s", nav)
	}
}

func TestWriteCoverImageManifestAndGuide(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "One", "# One")
	outPath := filepath.Join(dir, "out.epub")

	opts := DefaultOptions()
	opts.CoverImage = writeTestCoverPNG(t, dir, 300, 300)

	if err := Write([]*mdx.Document{doc}, opts, outPath); err != nil {
		t.Fatalf("Write: %v", err)
	}

	files := readZip(t, outPath)
	if _, ok := files["OEBPS/images/cover.png"]; !ok {
		t.Error("expected OEBPS/images/cover.png to be embedded")
	}
	if _, ok := files["OEBPS/text/cover.xhtml"]; !ok {
		t.Error("expected OEBPS/text/cover.xhtml to be generated")
	}

	opf := string(files["OEBPS/content.opf"])
	if !strings.Contains(opf, `properties="cover-image"`) {
		t.Error(`expected content.opf to mark the cover image item with properties="cover-image"`)
	}
	if !strings.Contains(opf, `<meta name="cover" content="cover-img"/>`) {
		t.Error("expected content.opf to declare the EPUB2-compat <meta name=\"cover\"> pointer")
	}
	if !strings.Contains(opf, `<reference type="cover" title="Cover" href="text/cover.xhtml"/>`) {
		t.Error("expected content.opf's <guide> to reference the cover page")
	}
	if !strings.Contains(opf, `idref="cover-page"`) {
		t.Error("expected the cover page to be the first spine item")
	}
}

func TestWriteWithoutCoverImageOmitsCoverFiles(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "One", "# One")
	outPath := filepath.Join(dir, "out.epub")

	if err := Write([]*mdx.Document{doc}, DefaultOptions(), outPath); err != nil {
		t.Fatalf("Write: %v", err)
	}

	files := readZip(t, outPath)
	if _, ok := files["OEBPS/text/cover.xhtml"]; ok {
		t.Error("expected no cover.xhtml when no cover image is configured")
	}
	opf := string(files["OEBPS/content.opf"])
	if strings.Contains(opf, "cover-image") {
		t.Error("expected content.opf to have no cover references when no cover image is configured")
	}
}

func TestWriteRejectsUnsupportedCoverFormat(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "One", "# One")
	badCover := filepath.Join(dir, "cover.bmp")
	if err := os.WriteFile(badCover, []byte("not an image"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := DefaultOptions()
	opts.CoverImage = badCover

	err := Write([]*mdx.Document{doc}, opts, filepath.Join(dir, "out.epub"))
	if err == nil {
		t.Error("expected an error for an unsupported cover image format, got nil")
	}
}

// TestWriteWebPCoverImage guards the parity between the PDF and EPUB cover
// paths: build's --cover-image documents .webp, and epub must accept it too
// rather than rejecting it after preflight already passed.
func TestWriteWebPCoverImage(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "One", "# One")

	webpCover := filepath.Join(dir, "cover.webp")
	if err := os.WriteFile(webpCover, []byte("RIFF....WEBPVP8 "), 0644); err != nil {
		t.Fatal(err)
	}

	opts := DefaultOptions()
	opts.CoverImage = webpCover

	outPath := filepath.Join(dir, "out.epub")
	if err := Write([]*mdx.Document{doc}, opts, outPath); err != nil {
		t.Fatalf("Write with .webp cover: %v", err)
	}

	files := readZip(t, outPath)
	if _, ok := files["OEBPS/images/cover.webp"]; !ok {
		t.Error("expected OEBPS/images/cover.webp to be embedded")
	}
	opf := string(files["OEBPS/content.opf"])
	if !strings.Contains(opf, "media-type=\"image/webp\"") {
		t.Error(`expected content.opf to declare the webp cover with media-type="image/webp"`)
	}
}

func TestWriteAcceptsSVGCoverImage(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "One", "# One")
	svgCover := filepath.Join(dir, "cover.svg")
	svgContent := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><rect fill="red" width="100" height="100"/></svg>`
	if err := os.WriteFile(svgCover, []byte(svgContent), 0644); err != nil {
		t.Fatal(err)
	}

	opts := DefaultOptions()
	opts.CoverImage = svgCover
	outPath := filepath.Join(dir, "out.epub")

	if err := Write([]*mdx.Document{doc}, opts, outPath); err != nil {
		t.Fatalf("Write with SVG cover: %v", err)
	}

	zr, err := zip.OpenReader(outPath)
	if err != nil {
		t.Fatalf("opening EPUB: %v", err)
	}
	defer func() { _ = zr.Close() }()

	found := false
	for _, f := range zr.File {
		if f.Name == "OEBPS/images/cover.svg" {
			found = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("opening cover.svg in EPUB: %v", err)
			}
			data, _ := io.ReadAll(rc)
			_ = rc.Close()
			if string(data) != svgContent {
				t.Errorf("cover.svg content mismatch")
			}
			break
		}
	}
	if !found {
		t.Error("expected OEBPS/images/cover.svg in EPUB, not found")
	}

	for _, f := range zr.File {
		if f.Name == "OEBPS/content.opf" {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			_ = rc.Close()
			opf := string(data)
			if !strings.Contains(opf, `media-type="image/svg+xml"`) {
				t.Error("expected content.opf to declare image/svg+xml media type for SVG cover")
			}
			break
		}
	}
}

// TestWriteFailureDoesNotClobberExistingOutput checks two things: that a
// failed rebuild leaves a previously-written, valid output file untouched
// rather than truncated, and that Write's temp-file-then-rename mechanism
// (see the os.CreateTemp/os.Rename dance in Write) leaves no stray
// "out.epub.tmp-*" file behind. Write used to open outputPath directly with
// os.Create, which truncates it immediately — any failure after that point
// (mid-archive, not just this early cover-format check) would have
// destroyed a previously good EPUB.
func TestWriteFailureDoesNotClobberExistingOutput(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.epub")

	goodDoc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "One", "# One")
	if err := Write([]*mdx.Document{goodDoc}, DefaultOptions(), outPath); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading previously-written output: %v", err)
	}

	badCover := filepath.Join(dir, "cover.bmp")
	if writeErr := os.WriteFile(badCover, []byte("not an image"), 0644); writeErr != nil {
		t.Fatal(writeErr)
	}
	opts := DefaultOptions()
	opts.CoverImage = badCover
	if writeErr := Write([]*mdx.Document{goodDoc}, opts, outPath); writeErr == nil {
		t.Fatal("expected the second Write to fail on the unsupported cover format")
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading output after failed Write: %v", err)
	}
	if string(got) != string(want) {
		t.Error("expected the failed Write to leave the previously-written output untouched")
	}

	leftovers, _ := filepath.Glob(filepath.Join(dir, "out.epub.tmp-*"))
	if len(leftovers) != 0 {
		t.Errorf("expected no leftover temp files, got: %v", leftovers)
	}
}

func TestWriteMetadataFields(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "One", "# One")
	outPath := filepath.Join(dir, "out.epub")

	opts := Options{
		Title:    "My Book",
		Subtitle: "A subtitle",
		Author:   "Jane Doe",
		Language: "es",
	}
	if err := Write([]*mdx.Document{doc}, opts, outPath); err != nil {
		t.Fatalf("Write: %v", err)
	}

	files := readZip(t, outPath)
	opf := string(files["OEBPS/content.opf"])
	for _, want := range []string{
		"<dc:title>My Book</dc:title>",
		"<dc:creator>Jane Doe</dc:creator>",
		"<dc:description>A subtitle</dc:description>",
		"<dc:language>es</dc:language>",
	} {
		if !strings.Contains(opf, want) {
			t.Errorf("expected content.opf to contain %q, got:\n%s", want, opf)
		}
	}
}

func TestApplyDefaults(t *testing.T) {
	opts := applyDefaults(Options{})
	if opts.Title != defaultTitle {
		t.Errorf("expected default title %q, got %q", defaultTitle, opts.Title)
	}
	if opts.Language != "en" {
		t.Errorf("expected default language 'en', got %q", opts.Language)
	}
}

func TestNewUUIDv4LooksLikeAUUID(t *testing.T) {
	id := newUUIDv4()
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Fatalf("expected 5 dash-separated groups, got %d: %q", len(parts), id)
	}
	lens := []int{8, 4, 4, 4, 12}
	for i, p := range parts {
		if len(p) != lens[i] {
			t.Errorf("group %d: expected length %d, got %d (%q)", i, lens[i], len(p), p)
		}
	}
	if id[14] != '4' {
		t.Errorf("expected version nibble '4' at position 14, got %q", id)
	}
}

func TestWriteEmbedsCustomCSS(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Chapter One", "# Chapter One\n\nHello.")
	outPath := filepath.Join(dir, "out.epub")

	customCSS := "/* custom-theme */ body { color: #ff0000; }"
	opts := DefaultOptions()
	opts.CSS = customCSS

	if err := Write([]*mdx.Document{doc}, opts, outPath); err != nil {
		t.Fatalf("Write: %v", err)
	}

	files := readZip(t, outPath)
	cssData, ok := files["OEBPS/css/style.css"]
	if !ok {
		t.Fatal("expected OEBPS/css/style.css in EPUB archive")
	}
	if !strings.Contains(string(cssData), "custom-theme") {
		t.Error("expected custom CSS to be embedded in style.css")
	}
	if !strings.Contains(string(cssData), "color: #ff0000") {
		t.Error("expected custom CSS content to be preserved")
	}
}

func TestWriteUsesDefaultCSSWhenOptionsCSSEmpty(t *testing.T) {
	dir := t.TempDir()
	doc := mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Chapter One", "# Chapter One\n\nHello.")
	outPath := filepath.Join(dir, "out.epub")

	opts := DefaultOptions()

	if err := Write([]*mdx.Document{doc}, opts, outPath); err != nil {
		t.Fatalf("Write: %v", err)
	}

	files := readZip(t, outPath)
	cssData, ok := files["OEBPS/css/style.css"]
	if !ok {
		t.Fatal("expected OEBPS/css/style.css in EPUB archive")
	}
	if !strings.Contains(string(cssData), "--pdf-bg") {
		t.Error("expected default CSS to use CSS custom properties")
	}
}

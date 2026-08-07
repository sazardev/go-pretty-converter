// Package epub converts parsed MDX documents into a single EPUB 3 file —
// no headless Chrome involved, unlike the PDF pipeline in render. Each
// mdx.Document becomes its own XHTML chapter, in the same reading order
// mdx.Parser.ParseDir already sorts them into.
package epub

import (
	"archive/zip"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sazardev/go-pretty-pdf/mdx"
)

// Options configures the generated EPUB. Zero-value fields fall back to
// sensible defaults in Write (see applyDefaults).
type Options struct {
	Title    string
	Subtitle string
	Author   string
	// Language is a BCP-47 tag (e.g. "en", "es"). Defaults to "en".
	Language string
	// CSS, if set, replaces the bundled default stylesheet outright.
	CSS string
	// CoverImage, when set, becomes the book's cover: a standalone
	// full-page image, first in reading order. Must be .png/.jpg/.jpeg/
	// .svg/.webp.
	CoverImage string
}

// defaultTitle mirrors compose.DefaultOptions' own "Document" fallback,
// used unless the caller sets Options.Title.
const defaultTitle = "Document"

func DefaultOptions() Options {
	return Options{
		Title:    defaultTitle,
		Author:   "go-pretty-pdf",
		Language: "en",
	}
}

func applyDefaults(opts Options) Options {
	if opts.Title == "" {
		opts.Title = defaultTitle
	}
	if opts.Language == "" {
		opts.Language = "en"
	}
	return opts
}

// chapterInfo pairs a parsed document with the manifest id/filename it
// will be written under — ch0001.xhtml, ch0002.xhtml, ... rather than
// anything derived from the document's own ID/title, so the file system
// name never has to deal with arbitrary title characters.
type chapterInfo struct {
	doc  *mdx.Document
	id   string
	file string
}

// Write composes docs (in the order given — mdx.Parser.ParseDir already
// sorts by frontmatter ID) into a single EPUB 3 file at outputPath.
func Write(docs []*mdx.Document, opts Options, outputPath string) error {
	opts = applyDefaults(opts)

	var coverBytes []byte
	var coverFile, coverMediaType string
	if opts.CoverImage != "" {
		var err error
		coverBytes, coverFile, coverMediaType, err = loadCoverImage(opts.CoverImage)
		if err != nil {
			return err
		}
	}

	chapters := make([]chapterInfo, len(docs))
	for i, doc := range docs {
		chapters[i] = chapterInfo{
			doc:  doc,
			id:   fmt.Sprintf("ch%04d", i+1),
			file: fmt.Sprintf("ch%04d.xhtml", i+1),
		}
	}

	uuid := newUUIDv4()
	modified := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	outDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	// Written to a temp file and renamed into place only once the archive
	// is fully built, so a failure partway through (a bad chapter, a full
	// disk, an interrupted process) can't truncate or corrupt a
	// previously good outputPath the way writing directly into it with
	// os.Create would.
	f, err := os.CreateTemp(outDir, filepath.Base(outputPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp output file: %w", err)
	}
	tmpPath := f.Name()
	// Registered before the Close defer below so it runs after it (defers
	// run LIFO): removing tmpPath while f is still open fails silently on
	// Windows, which — unlike POSIX — doesn't allow deleting an open file.
	// Both are no-ops by the time Write returns successfully (f is closed
	// explicitly below, and tmpPath no longer exists once renamed).
	defer func() { _ = os.Remove(tmpPath) }()
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)

	// mimetype must be the first entry in the archive and stored
	// uncompressed — that's how many EPUB readers/OS file-type sniffers
	// identify an EPUB without a full zip parse (OCF spec, section 3.3).
	mimeW, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		return fmt.Errorf("writing mimetype entry: %w", err)
	}
	if _, err = mimeW.Write([]byte("application/epub+zip")); err != nil {
		return fmt.Errorf("writing mimetype entry: %w", err)
	}

	if err = writeZipString(zw, "META-INF/container.xml", containerXML); err != nil {
		return err
	}

	css := opts.CSS
	if css == "" {
		css = defaultCSS
	}
	if err = writeZipString(zw, "OEBPS/css/style.css", css); err != nil {
		return err
	}

	if coverBytes != nil {
		if err = writeZipBytes(zw, "OEBPS/images/"+coverFile, coverBytes); err != nil {
			return err
		}
		var coverXHTML string
		if coverXHTML, err = renderCoverXHTML(opts, coverFile); err != nil {
			return err
		}
		if err = writeZipString(zw, "OEBPS/text/cover.xhtml", coverXHTML); err != nil {
			return err
		}
	}

	for _, ch := range chapters {
		var xhtml string
		if xhtml, err = renderChapterXHTML(opts, ch); err != nil {
			return fmt.Errorf("rendering chapter for %s: %w", ch.doc.ID(), err)
		}
		if err = writeZipString(zw, "OEBPS/text/"+ch.file, xhtml); err != nil {
			return err
		}
	}

	navXHTML, err := renderNavXHTML(opts, buildNavTree(chapters))
	if err != nil {
		return err
	}
	if err = writeZipString(zw, "OEBPS/nav.xhtml", navXHTML); err != nil {
		return err
	}

	ncx := renderNCX(opts, uuid, chapters)
	if err = writeZipString(zw, "OEBPS/toc.ncx", ncx); err != nil {
		return err
	}

	opf := renderOPF(opts, uuid, modified, chapters, coverFile, coverMediaType)
	if err = writeZipString(zw, "OEBPS/content.opf", opf); err != nil {
		return err
	}

	if err = zw.Close(); err != nil {
		return fmt.Errorf("finalizing EPUB archive: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("closing temp output file: %w", err)
	}
	if err = os.Rename(tmpPath, outputPath); err != nil {
		return fmt.Errorf("finalizing output file: %w", err)
	}
	return nil
}

func writeZipString(zw *zip.Writer, name, content string) error {
	return writeZipBytes(zw, name, []byte(content))
}

func writeZipBytes(zw *zip.Writer, name string, content []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	if _, err := w.Write(content); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	return nil
}

// loadCoverImage validates and reads opts.CoverImage, returning its bytes,
// the filename to store it under in OEBPS/images/, and its MIME type.
func loadCoverImage(path string) (data []byte, file, mediaType string, err error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		mediaType = "image/png"
	case ".jpg", ".jpeg":
		mediaType = "image/jpeg"
	case ".svg":
		mediaType = "image/svg+xml"
	case ".webp":
		mediaType = "image/webp"
	default:
		return nil, "", "", fmt.Errorf("cover image %s: unsupported format (expected .png, .jpg, .jpeg, .svg, or .webp)", path)
	}

	data, err = os.ReadFile(path)
	if err != nil {
		return nil, "", "", fmt.Errorf("reading cover image: %w", err)
	}
	return data, "cover" + ext, mediaType, nil
}

// newUUIDv4 generates a random RFC 4122 version 4 UUID for the package's
// required dc:identifier — regenerated on every Write call rather than
// derived from content, since EPUB readers key their library entries off
// it and a build tool has no stable "this is edition N" concept to hang a
// deterministic identifier on.
func newUUIDv4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

const containerXML = `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`

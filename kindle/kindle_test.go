package kindle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sazardev/go-pretty-pdf/mdx"
)

// fakeEbookConvertPath is a compiled stand-in for Calibre's ebook-convert,
// built once in TestMain from testdata/fakeebookconvert. Its behavior is
// controlled per-test via the FAKE_EBOOK_CONVERT_MODE env var (see that
// program's doc comment) so these tests never need Calibre installed.
var fakeEbookConvertPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "kindle-fake-calibre-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	bin := filepath.Join(dir, "fakeebookconvert")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./testdata/fakeebookconvert")
	if out, buildErr := cmd.CombinedOutput(); buildErr != nil {
		fmt.Fprintf(os.Stderr, "building fake ebook-convert: %v\n%s\n", buildErr, out)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}
	fakeEbookConvertPath = bin

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

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

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.Timeout != 5*time.Minute {
		t.Errorf("expected a 5 minute default timeout, got %v", opts.Timeout)
	}
	if opts.EPUB.Language != "en" {
		t.Errorf("expected the embedded epub.Options to carry its own defaults, got language %q", opts.EPUB.Language)
	}
}

func TestResolveCalibreExplicitPath(t *testing.T) {
	path, err := ResolveCalibre(fakeEbookConvertPath)
	if err != nil {
		t.Fatalf("ResolveCalibre: %v", err)
	}
	if path != fakeEbookConvertPath {
		t.Errorf("got %q, want %q", path, fakeEbookConvertPath)
	}
}

func TestResolveCalibreExplicitPathMissing(t *testing.T) {
	_, err := ResolveCalibre(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected an error for a missing --calibre-path")
	}
}

func TestResolveCalibreExplicitPathIsDir(t *testing.T) {
	_, err := ResolveCalibre(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("expected a 'directory' error, got %v", err)
	}
}

func TestResolveCalibreNotFoundOnPATH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := ResolveCalibre("")
	if !errors.Is(err, ErrCalibreNotFound) {
		t.Fatalf("expected ErrCalibreNotFound, got %v", err)
	}
}

func TestWriteSuccess(t *testing.T) {
	t.Setenv("FAKE_EBOOK_CONVERT_MODE", "success")

	dir := t.TempDir()
	docs := []*mdx.Document{mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Chapter One", "Hello.")}
	outPath := filepath.Join(dir, "book.mobi")

	opts := Options{CalibrePath: fakeEbookConvertPath}
	if err := Write(context.Background(), docs, opts, outPath); err != nil {
		t.Fatalf("Write: %v", err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
	if info.Size() == 0 {
		t.Error("expected a non-empty output file (the fake converter copies the intermediate EPUB through)")
	}
}

func TestWriteEbookConvertFailure(t *testing.T) {
	t.Setenv("FAKE_EBOOK_CONVERT_MODE", "fail")

	dir := t.TempDir()
	docs := []*mdx.Document{mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Chapter One", "Hello.")}
	outPath := filepath.Join(dir, "book.mobi")

	err := Write(context.Background(), docs, Options{CalibrePath: fakeEbookConvertPath}, outPath)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "ebook-convert failed") {
		t.Errorf("expected the error to mention ebook-convert failing, got: %v", err)
	}
	if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
		t.Error("expected no output file on a failed conversion")
	}
}

func TestWriteTimeout(t *testing.T) {
	t.Setenv("FAKE_EBOOK_CONVERT_MODE", "hang")

	dir := t.TempDir()
	docs := []*mdx.Document{mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Chapter One", "Hello.")}
	outPath := filepath.Join(dir, "book.mobi")

	opts := Options{CalibrePath: fakeEbookConvertPath, Timeout: 200 * time.Millisecond}
	err := Write(context.Background(), docs, opts, outPath)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected a timeout error, got: %v", err)
	}
}

func TestWriteCalibreNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	dir := t.TempDir()
	docs := []*mdx.Document{mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Chapter One", "Hello.")}
	outPath := filepath.Join(dir, "book.mobi")

	err := Write(context.Background(), docs, Options{}, outPath)
	if !errors.Is(err, ErrCalibreNotFound) {
		t.Fatalf("expected ErrCalibreNotFound, got %v", err)
	}
	if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
		t.Error("expected no output file when Calibre can't be resolved")
	}
}

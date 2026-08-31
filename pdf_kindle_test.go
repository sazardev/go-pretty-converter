package prettypdf

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

	"github.com/sazardev/go-pretty-pdf/kindle"
)

// fakeEbookConvertPath is a compiled stand-in for Calibre's ebook-convert,
// built once here from kindle/testdata/fakeebookconvert so the FormatKindle
// Build() tests below run without Calibre installed. See that program's
// doc comment for the FAKE_EBOOK_CONVERT_MODE env var it honors.
var fakeEbookConvertPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pretty-pdf-fake-calibre-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	bin := filepath.Join(dir, "fakeebookconvert")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./kindle/testdata/fakeebookconvert")
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

func TestParseFormatsKindle(t *testing.T) {
	formats, err := ParseFormats("kindle")
	if err != nil {
		t.Fatalf("ParseFormats: %v", err)
	}
	if len(formats) != 1 || formats[0] != FormatKindle {
		t.Errorf("got %v, want [kindle]", formats)
	}

	formats, err = ParseFormats("pdf,epub,kindle")
	if err != nil {
		t.Fatalf("ParseFormats: %v", err)
	}
	want := []OutputFormat{FormatPDF, FormatEPUB, FormatKindle}
	if len(formats) != len(want) {
		t.Fatalf("got %v, want %v", formats, want)
	}
	for i := range want {
		if formats[i] != want[i] {
			t.Errorf("got %v, want %v", formats, want)
			break
		}
	}
}

func TestParseFormatsUnsupportedMentionsKindle(t *testing.T) {
	_, err := ParseFormats("mobi")
	if err == nil || !strings.Contains(err.Error(), "kindle") {
		t.Fatalf("expected the unsupported-format error to mention kindle, got %v", err)
	}
}

func TestKindleOutputPath(t *testing.T) {
	const wantMobi = "mybook.mobi"
	tests := []struct {
		outputFile string
		want       string
	}{
		{"mybook.pdf", wantMobi},
		{"mybook.epub", wantMobi},
		{"mybook.mobi", wantMobi},
		{"mybook.azw3", "mybook.azw3"},
		{"mybook", wantMobi},
	}
	for _, tt := range tests {
		p := &PDF{outputFile: tt.outputFile}
		if got := p.kindleOutputPath(); got != tt.want {
			t.Errorf("kindleOutputPath(%q) = %q, want %q", tt.outputFile, got, tt.want)
		}
	}
}

func TestNeedsCalibre(t *testing.T) {
	dir := t.TempDir()
	p, err := New(WithSourceDir(dir), WithFormats(FormatKindle))
	if err != nil {
		t.Fatal(err)
	}
	if !p.NeedsCalibre() {
		t.Error("expected NeedsCalibre() to be true for FormatKindle")
	}

	p2, err := New(WithSourceDir(dir), WithFormats(FormatPDF))
	if err != nil {
		t.Fatal(err)
	}
	if p2.NeedsCalibre() {
		t.Error("expected NeedsCalibre() to be false without FormatKindle")
	}
}

func TestBuildKindle(t *testing.T) {
	t.Setenv("FAKE_EBOOK_CONVERT_MODE", "success")

	dir := t.TempDir()
	writeFixtureMDX(t, dir, "a.mdx", "[1.0.0]", "Chapter One")

	outPath := filepath.Join(t.TempDir(), "book.mobi")
	p, err := New(
		WithSourceDir(dir),
		WithOutputFile(outPath),
		WithFormats(FormatKindle),
		WithCalibreExecPath(fakeEbookConvertPath),
	)
	if err != nil {
		t.Fatal(err)
	}
	if buildErr := p.Build(context.Background()); buildErr != nil {
		t.Fatalf("Build: %v", buildErr)
	}

	info, statErr := os.Stat(outPath)
	if statErr != nil || info.Size() == 0 {
		t.Fatal("expected a non-empty Kindle file to be produced")
	}
}

func TestBuildKindleCalibreNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	dir := t.TempDir()
	writeFixtureMDX(t, dir, "a.mdx", "[1.0.0]", "Chapter One")

	outPath := filepath.Join(t.TempDir(), "book.mobi")
	p, err := New(WithSourceDir(dir), WithOutputFile(outPath), WithFormats(FormatKindle))
	if err != nil {
		t.Fatal(err)
	}
	buildErr := p.Build(context.Background())
	if !errors.Is(buildErr, kindle.ErrCalibreNotFound) {
		t.Fatalf("expected ErrCalibreNotFound, got %v", buildErr)
	}
}

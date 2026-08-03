package render

import (
	"bytes"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// coverPageCenterPixel extracts the embedded image from pdfPath's page 1
// and decodes the RGB color at its center. Used to confirm the cover page
// actually contains the source image's pixels, not just a page sized to
// match it — a regression test for a real bug where the image silently
// failed to load (see the comment on the data: URI in coverPDFTasks) while
// the page's size, set independently via PrintToPDF's paperWidth/
// paperHeight parameters, stayed correct either way.
func coverPageCenterPixel(t *testing.T, pdfPath string) (r, g, b uint32) {
	t.Helper()

	f, err := os.Open(pdfPath)
	if err != nil {
		t.Fatalf("opening PDF for image extraction: %v", err)
	}
	defer func() { _ = f.Close() }()

	imgsByPage, err := api.ExtractImagesRaw(f, []string{"1"}, nil)
	if err != nil {
		t.Fatalf("extracting images from cover page: %v", err)
	}
	if len(imgsByPage) == 0 {
		t.Fatal("expected the cover page to contain an embedded image, found none")
	}

	for _, byPage := range imgsByPage {
		for _, img := range byPage {
			data, err := io.ReadAll(img)
			if err != nil {
				t.Fatalf("reading extracted image bytes: %v", err)
			}
			decoded, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("decoding extracted cover image: %v", err)
			}
			bounds := decoded.Bounds()
			cr, cg, cb, _ := decoded.At(bounds.Dx()/2, bounds.Dy()/2).RGBA()
			return cr >> 8, cg >> 8, cb >> 8
		}
	}
	t.Fatal("unreachable: imgsByPage had entries but no image was returned")
	return 0, 0, 0
}

// writeTestPNG writes a solid-color w x h PNG to dir/name and returns its
// path.
func writeTestPNG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating test PNG: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encoding test PNG: %v", err)
	}
	return path
}

func TestCoverImageDimensionsIn(t *testing.T) {
	dir := t.TempDir()
	path := writeTestPNG(t, dir, "cover.png", 960, 480)

	w, h, err := coverImageDimensionsIn(path)
	if err != nil {
		t.Fatalf("coverImageDimensionsIn: %v", err)
	}
	if w != 10 || h != 5 {
		t.Errorf("coverImageDimensionsIn() = %v x %v, want 10 x 5 (960x480px at 96px/in)", w, h)
	}
}

func TestCoverImageDimensionsInRejectsUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cover.gif")
	if err := os.WriteFile(path, []byte("not a real gif"), 0644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	if _, _, err := coverImageDimensionsIn(path); err == nil {
		t.Error("expected an error for an unsupported cover image extension, got nil")
	}
}

func TestCoverImageDimensionsInMissingFile(t *testing.T) {
	if _, _, err := coverImageDimensionsIn(filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Error("expected an error for a missing cover image file, got nil")
	}
}

func TestCoverImageDimensionsInRejectsOversizedDimensions(t *testing.T) {
	dir := t.TempDir()
	path := writeTestPNG(t, dir, "huge.png", maxCoverImageDimension+1, 1)

	if _, _, err := coverImageDimensionsIn(path); err == nil {
		t.Error("expected an error for a cover image exceeding the pixel dimension limit, got nil")
	}
}

func TestCoverPDFTasksRejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating fixture: %v", err)
	}
	if err := f.Truncate(maxCoverImageBytes + 1); err != nil {
		t.Fatalf("truncating fixture to size: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing fixture: %v", err)
	}

	if _, _, err := coverPDFTasks(path, 1, 1, new([]byte)); err == nil {
		t.Error("expected an error for a cover image file exceeding the byte size limit, got nil")
	}
}

// TestRenderToPDFWithCoverImageUsesImageDimensions is an end-to-end check
// that a custom cover image produces a first page sized to the image's own
// pixel dimensions (a square image gets a square page) while the rest of
// the document keeps the configured paper size untouched.
func TestRenderToPDFWithCoverImageUsesImageDimensions(t *testing.T) {
	requireChrome(t)

	dir := t.TempDir()
	coverPath := writeTestPNG(t, dir, "cover.png", 480, 480) // 5in x 5in square
	outPath := filepath.Join(dir, "out.pdf")

	opts := DefaultOptions()
	opts.CoverImagePath = coverPath
	// A4-ish default paper size stays as the body's page size.
	if opts.PaperWidth == opts.PaperHeight {
		t.Fatal("test assumes a non-square default paper size to distinguish it from the square cover")
	}

	html := `<html><body><h1>Page one</h1><div style="page-break-before:always">Page two</div></body></html>`
	if _, err := RenderToPDFWithAudit(html, outPath, opts); err != nil {
		t.Fatalf("RenderToPDFWithAudit with cover image: %v", err)
	}

	dims, err := api.PageDimsFile(outPath)
	if err != nil {
		t.Fatalf("reading merged PDF page dimensions: %v", err)
	}
	if len(dims) < 3 {
		t.Fatalf("expected at least 3 pages (1 cover + 2 body), got %d", len(dims))
	}

	const ptPerIn = 72.0
	wantCoverPt := 5.0 * ptPerIn
	if diff := dims[0].Width - wantCoverPt; diff < -0.5 || diff > 0.5 {
		t.Errorf("cover page width = %.2fpt, want ~%.2fpt (5in)", dims[0].Width, wantCoverPt)
	}
	if dims[0].Width != dims[0].Height {
		t.Errorf("cover page is not square: %v x %v", dims[0].Width, dims[0].Height)
	}

	wantBodyWidthPt := opts.PaperWidth * ptPerIn
	if diff := dims[1].Width - wantBodyWidthPt; diff < -0.5 || diff > 0.5 {
		t.Errorf("body page width = %.2fpt, want ~%.2fpt (configured paper size)", dims[1].Width, wantBodyWidthPt)
	}
	if dims[1].Width == dims[0].Width && dims[1].Height == dims[0].Height {
		t.Error("expected body pages to keep the normal paper size, not the cover's square dimensions")
	}

	// The page being sized correctly doesn't guarantee the image actually
	// loaded onto it — PaperWidth/PaperHeight are set independently of
	// the <img> tag succeeding. writeTestPNG fills the whole image with
	// RGB(200, 40, 40), so the cover page's center pixel must match it.
	r, g, b := coverPageCenterPixel(t, outPath)
	const tol = 8
	if abs(int(r)-200) > tol || abs(int(g)-40) > tol || abs(int(b)-40) > tol {
		t.Errorf("cover page center pixel = rgb(%d,%d,%d), want ~rgb(200,40,40) — the cover image likely failed to load onto the page", r, g, b)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func TestMergeCoverAndBodyPreservesExistingFileOnFailure(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "book.pdf")

	const goodContent = "existing good pdf bytes"
	if err := os.WriteFile(outputPath, []byte(goodContent), 0644); err != nil {
		t.Fatalf("seeding output file: %v", err)
	}

	if err := mergeCoverAndBody([]byte("not a pdf"), []byte("also not a pdf"), outputPath); err == nil {
		t.Fatal("expected mergeCoverAndBody to fail on invalid PDF input")
	}

	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if string(got) != goodContent {
		t.Errorf("existing output file was modified on merge failure: got %q, want %q", got, goodContent)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected no leftover temp files, got %d entries: %v", len(entries), entries)
	}
}

func TestRenderToPDFWithCoverImageRejectsUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "cover.bmp")
	if err := os.WriteFile(badPath, []byte("not an image"), 0644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	opts := DefaultOptions()
	opts.CoverImagePath = badPath

	// Deliberately not gated on requireChrome: the format check happens
	// before any browser is launched, so this must fail fast regardless of
	// whether Chrome is available in the test environment.
	if _, err := RenderToPDFWithAudit(`<html><body>x</body></html>`, filepath.Join(dir, "out.pdf"), opts); err == nil {
		t.Error("expected an error for an unsupported cover image format, got nil")
	}
}

func writeTestSVG(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test SVG: %v", err)
	}
	return path
}

func TestSVGPixelDimensionsWithWidthHeight(t *testing.T) {
	dir := t.TempDir()
	path := writeTestSVG(t, dir, "cover.svg",
		`<svg xmlns="http://www.w3.org/2000/svg" width="960" height="480"></svg>`)

	w, h, err := coverImageDimensionsIn(path)
	if err != nil {
		t.Fatalf("coverImageDimensionsIn: %v", err)
	}
	if w != 10 || h != 5 {
		t.Errorf("got %v x %v, want 10 x 5 (960x480px at 96px/in)", w, h)
	}
}

func TestSVGPixelDimensionsWithViewBox(t *testing.T) {
	dir := t.TempDir()
	path := writeTestSVG(t, dir, "cover.svg",
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1920 960"></svg>`)

	w, h, err := coverImageDimensionsIn(path)
	if err != nil {
		t.Fatalf("coverImageDimensionsIn: %v", err)
	}
	if w != 20 || h != 10 {
		t.Errorf("got %v x %v, want 20 x 10 (1920x960 from viewBox at 96px/in)", w, h)
	}
}

func TestSVGPixelDimensionsWithInches(t *testing.T) {
	dir := t.TempDir()
	path := writeTestSVG(t, dir, "cover.svg",
		`<svg xmlns="http://www.w3.org/2000/svg" width="10in" height="5in"></svg>`)

	w, h, err := coverImageDimensionsIn(path)
	if err != nil {
		t.Fatalf("coverImageDimensionsIn: %v", err)
	}
	if w != 10 || h != 5 {
		t.Errorf("got %v x %v, want 10 x 5 (10in x 5in at 96px/in)", w, h)
	}
}

func TestSVGPixelDimensionsWidthHeightOverridesViewBox(t *testing.T) {
	dir := t.TempDir()
	path := writeTestSVG(t, dir, "cover.svg",
		`<svg xmlns="http://www.w3.org/2000/svg" width="960" height="480" viewBox="0 0 1920 960"></svg>`)

	w, h, err := coverImageDimensionsIn(path)
	if err != nil {
		t.Fatalf("coverImageDimensionsIn: %v", err)
	}
	if w != 10 || h != 5 {
		t.Errorf("got %v x %v, want 10 x 5 (width/height should take precedence over viewBox)", w, h)
	}
}

func TestSVGPixelDimensionsFallsBackToViewBoxForRelativeUnits(t *testing.T) {
	dir := t.TempDir()
	path := writeTestSVG(t, dir, "cover.svg",
		`<svg xmlns="http://www.w3.org/2000/svg" width="100%" height="100%" viewBox="0 0 480 240"></svg>`)

	w, h, err := coverImageDimensionsIn(path)
	if err != nil {
		t.Fatalf("coverImageDimensionsIn: %v", err)
	}
	if w != 5 || h != 2.5 {
		t.Errorf("got %v x %v, want 5 x 2.5 (should fall back to viewBox for %% units)", w, h)
	}
}

func TestSVGPixelDimensionsRejectsNoDimensions(t *testing.T) {
	dir := t.TempDir()
	path := writeTestSVG(t, dir, "cover.svg",
		`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)

	if _, _, err := coverImageDimensionsIn(path); err == nil {
		t.Error("expected an error for SVG with no dimensions or viewBox, got nil")
	}
}

func TestCoverImageMIMEType(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"cover.png", mimeTypePNG},
		{"cover.PNG", mimeTypePNG},
		{"cover.jpg", mimeTypeJPEG},
		{"cover.jpeg", mimeTypeJPEG},
		{"cover.JPEG", mimeTypeJPEG},
		{"cover.svg", mimeTypeSVG},
		{"cover.SVG", mimeTypeSVG},
		{"cover.webp", mimeTypeWebP},
		{"cover.WEBP", mimeTypeWebP},
	}
	for _, tt := range tests {
		if got := coverImageMIMEType(tt.path); got != tt.want {
			t.Errorf("coverImageMIMEType(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

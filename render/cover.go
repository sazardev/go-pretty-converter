package render

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "golang.org/x/image/webp"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// coverPxPerIn matches the "px" convention config.ParseCSSUnit already uses
// for CSS lengths (1in == 96px), so a cover image ends up printed at its
// native pixel size rather than being rescaled to fit a paper size.
const coverPxPerIn = 96.0

// maxCoverImageBytes bounds how much of a cover image file coverPDFTasks
// will read into memory for its data: URI. Nothing about the format
// validation above limits file size, so an unbounded os.ReadFile here would
// happily load an arbitrarily large file — worth capping since, unlike the
// rest of the document, WithCoverImage's path can come from outside the
// trusted MDX source tree (e.g. a pipeline parameter).
const maxCoverImageBytes = 32 * 1024 * 1024 // 32MB

// maxCoverImageDimension bounds the pixel width/height coverImageDimensionsIn
// will accept, since that value is later passed straight through as
// PrintToPDF's paperWidth/paperHeight with no other limit — an image whose
// header merely *claims* an enormous size would otherwise reach Chrome as a
// request to print an enormous page.
const maxCoverImageDimension = 20000

// coverImageDimensionsIn decodes imagePath's pixel dimensions (without
// loading the full pixel data) and converts them to inches for use as an
// exact PrintToPDF paper size.
func coverImageDimensionsIn(imagePath string) (widthIn, heightIn float64, err error) {
	ext := strings.ToLower(filepath.Ext(imagePath))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
	case ".svg":
		var w, h float64
		w, h, err = svgPixelDimensions(imagePath)
		if err != nil {
			return 0, 0, err
		}
		return w / coverPxPerIn, h / coverPxPerIn, nil
	default:
		return 0, 0, fmt.Errorf("cover image %s: unsupported format (expected .png, .jpg, .jpeg, .svg, or .webp)", imagePath)
	}

	f, err := os.Open(imagePath)
	if err != nil {
		return 0, 0, fmt.Errorf("opening cover image: %w", err)
	}
	defer func() { _ = f.Close() }()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, fmt.Errorf("decoding cover image %s: %w", imagePath, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, fmt.Errorf("cover image %s has no usable dimensions", imagePath)
	}
	if cfg.Width > maxCoverImageDimension || cfg.Height > maxCoverImageDimension {
		return 0, 0, fmt.Errorf("cover image %s is %dx%d px, exceeding the %dx%d px limit",
			imagePath, cfg.Width, cfg.Height, maxCoverImageDimension, maxCoverImageDimension)
	}

	return float64(cfg.Width) / coverPxPerIn, float64(cfg.Height) / coverPxPerIn, nil
}

const (
	mimeTypePNG  = "image/png"
	mimeTypeJPEG = "image/jpeg"
	mimeTypeSVG  = "image/svg+xml"
	mimeTypeWebP = "image/webp"
)

// coverImageMIMEType maps a cover image's extension to its MIME type for a
// data: URI. Callers must have already validated the extension via
// coverImageDimensionsIn.
func coverImageMIMEType(imagePath string) string {
	switch strings.ToLower(filepath.Ext(imagePath)) {
	case ".png":
		return mimeTypePNG
	case ".svg":
		return mimeTypeSVG
	case ".webp":
		return mimeTypeWebP
	default:
		return mimeTypeJPEG
	}
}

// svgPixelDimensions extracts the pixel width and height from an SVG file
// by parsing its XML. It checks width/height attributes first (converting
// absolute units to pixels at 96px/in), then falls back to the viewBox
// attribute if dimensions are missing or use relative units.
func svgPixelDimensions(imagePath string) (widthPx, heightPx float64, err error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return 0, 0, fmt.Errorf("opening cover image: %w", err)
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	var svgWidth, svgHeight, viewBox string
	var tok xml.Token
	for {
		tok, err = decoder.Token()
		if err != nil {
			return 0, 0, fmt.Errorf("parsing SVG %s: %w", imagePath, err)
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "svg" {
			for _, attr := range se.Attr {
				switch attr.Name.Local {
				case "width":
					svgWidth = attr.Value
				case "height":
					svgHeight = attr.Value
				case "viewBox":
					viewBox = attr.Value
				}
			}
			break
		}
	}

	w, h, err := parseSVGDimensions(svgWidth, svgHeight, viewBox)
	if err != nil {
		return 0, 0, fmt.Errorf("cover image %s: %w", imagePath, err)
	}
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("cover image %s has no usable dimensions", imagePath)
	}
	if w > maxCoverImageDimension || h > maxCoverImageDimension {
		return 0, 0, fmt.Errorf("cover image %s is %.0fx%.0f px, exceeding the %dx%d px limit",
			imagePath, w, h, maxCoverImageDimension, maxCoverImageDimension)
	}
	return w, h, nil
}

// parseSVGDimensions resolves width and height in pixels from SVG attributes.
// It tries width/height first (converting absolute units), then falls back
// to viewBox if dimensions are missing or use relative units like em/%.
func parseSVGDimensions(widthAttr, heightAttr, viewBox string) (w, h float64, err error) {
	w = svgLengthToPixels(widthAttr)
	h = svgLengthToPixels(heightAttr)

	if w > 0 && h > 0 {
		return w, h, nil
	}

	if viewBox != "" {
		parts := strings.Fields(viewBox)
		if len(parts) == 4 {
			vbW, errW := strconv.ParseFloat(parts[2], 64)
			vbH, errH := strconv.ParseFloat(parts[3], 64)
			if errW == nil && errH == nil && vbW > 0 && vbH > 0 {
				if w <= 0 {
					w = vbW
				}
				if h <= 0 {
					h = vbH
				}
				return w, h, nil
			}
		}
	}

	return 0, 0, fmt.Errorf("cannot determine SVG dimensions (need width/height with absolute units or viewBox)")
}

// svgLengthToPixels converts an SVG length attribute (e.g. "100px", "2.5in",
// "8cm") to pixels at 96px/in. Returns 0 if the value is missing, uses
// relative units (em, %, etc), or is unparseable.
func svgLengthToPixels(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	const pxPerIn = 96.0
	var numStr string
	var factor float64

	switch {
	case strings.HasSuffix(s, "px"):
		numStr = strings.TrimSuffix(s, "px")
		factor = 1
	case strings.HasSuffix(s, "in"):
		numStr = strings.TrimSuffix(s, "in")
		factor = pxPerIn
	case strings.HasSuffix(s, "cm"):
		numStr = strings.TrimSuffix(s, "cm")
		factor = pxPerIn / 2.54
	case strings.HasSuffix(s, "mm"):
		numStr = strings.TrimSuffix(s, "mm")
		factor = pxPerIn / 25.4
	case strings.HasSuffix(s, "pt"):
		numStr = strings.TrimSuffix(s, "pt")
		factor = pxPerIn / 72.0
	case strings.HasSuffix(s, "pc"):
		numStr = strings.TrimSuffix(s, "pc")
		factor = pxPerIn / 6.0
	default:
		if len(s) > 0 && (s[len(s)-1] < '0' || s[len(s)-1] > '9') && s[len(s)-1] != '.' {
			return 0
		}
		numStr = s
		factor = 1
	}

	v, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64)
	if err != nil {
		return 0
	}
	return v * factor
}

// coverPDFTasks builds the chromedp actions that print imagePath, full
// bleed and edge to edge, as a standalone single page sized to exactly
// widthIn x heightIn, writing the resulting PDF bytes into *out once the
// returned tasks run inside a chromedp.Run call. The returned cleanup func
// removes the temporary HTML file the tasks navigate to and must be called
// after that Run completes (defer it right after a nil error check).
func coverPDFTasks(imagePath string, widthIn, heightIn float64, out *[]byte) (chromedp.Tasks, func(), error) {
	if info, err := os.Stat(imagePath); err == nil && info.Size() > maxCoverImageBytes {
		return nil, nil, fmt.Errorf("cover image %s is %d bytes, exceeding the %d byte limit",
			imagePath, info.Size(), maxCoverImageBytes)
	}

	imgBytes, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading cover image: %w", err)
	}
	// Embedded as a data: URI rather than referenced by a file:// src:
	// RenderToPDFWithAudit blocks all network requests by default
	// (Options.NetworkAccess == false) via Network.setBlockedURLs("*://*/*"),
	// which — perhaps surprisingly — also matches and blocks file://
	// subresource fetches, not just remote http(s) ones. The top-level
	// Navigate to this cover page's own temp file still works (navigation
	// isn't blocked the same way), but an <img src="file://..."> inside it
	// would silently fail to load under that same default. A data: URI
	// carries the pixels inline, so no subresource fetch — blocked or
	// not — ever happens.
	dataURI := "data:" + coverImageMIMEType(imagePath) + ";base64," + base64.StdEncoding.EncodeToString(imgBytes)

	// No theme CSS, header/footer, or margin here on purpose: this page is
	// the image, exactly as-is, at exactly its own size — nothing else
	// should compete for pixels on it.
	html := fmt.Sprintf(
		`<!DOCTYPE html><html><head><meta charset="UTF-8">`+
			`<style>html,body{margin:0;padding:0;}img{display:block;width:100%%;height:100%%;}</style>`+
			`</head><body><img src="%s"></body></html>`,
		dataURI,
	)

	navURL, cleanup, err := navigationURLFor(html)
	if err != nil {
		return nil, nil, err
	}

	tasks := chromedp.Tasks{
		chromedp.Navigate(navURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			*out, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithMarginTop(0).
				WithMarginBottom(0).
				WithMarginLeft(0).
				WithMarginRight(0).
				WithPaperWidth(widthIn).
				WithPaperHeight(heightIn).
				Do(ctx)
			return err
		}),
	}
	return tasks, cleanup, nil
}

// mergeCoverAndBody concatenates coverPDF in front of bodyPDF into a single
// PDF written to outputPath, with each side keeping its own page size.
// Page.printToPDF only accepts one paperWidth/paperHeight per call, so a
// cover page sized to a custom image can never share a print pass with the
// rest of the document — the two are printed separately and stitched back
// together here instead.
func mergeCoverAndBody(coverPDF, bodyPDF []byte, outputPath string) error {
	outDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Written to a temp file and renamed into place only on success, so a
	// mid-merge failure (malformed cover PDF, pdfcpu error) can't truncate
	// or corrupt a previously good outputPath — os.Create would otherwise
	// blow away the existing file before MergeRaw has produced anything.
	tmp, err := os.CreateTemp(outDir, filepath.Base(outputPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp output PDF: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	rsc := []io.ReadSeeker{bytes.NewReader(coverPDF), bytes.NewReader(bodyPDF)}
	mergeErr := api.MergeRaw(rsc, tmp, false, nil)
	closeErr := tmp.Close()
	if mergeErr != nil {
		return fmt.Errorf("merging cover and body PDFs: %w", mergeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("closing temp output PDF: %w", closeErr)
	}

	if err := os.Rename(tmpPath, outputPath); err != nil {
		return fmt.Errorf("finalizing output PDF: %w", err)
	}
	return nil
}

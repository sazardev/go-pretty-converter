// Package kindle converts parsed MDX documents into a Kindle-ready ebook
// file (MOBI/AZW3). It builds the same intermediate EPUB the epub package
// would produce, then converts it with Calibre's ebook-convert — the de
// facto standard converter now that Amazon retired its own KindleGen tool
// in 2022. No Chrome involved, but Calibre must be installed with
// ebook-convert reachable on PATH (or pointed to explicitly).
package kindle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sazardev/go-pretty-pdf/epub"
	"github.com/sazardev/go-pretty-pdf/mdx"
)

// Options configures the generated Kindle file. EPUB carries the same
// metadata/CSS/cover options the epub package accepts: Write builds an
// intermediate EPUB from them and hands it to ebook-convert, so a Kindle
// build is the same book as an EPUB build, just repackaged for Kindle.
type Options struct {
	EPUB epub.Options
	// CalibrePath, when set, pins the exact ebook-convert executable to
	// invoke instead of resolving one from PATH.
	CalibrePath string
	// Timeout bounds how long ebook-convert may run. Defaults to 5 minutes.
	Timeout time.Duration
}

func DefaultOptions() Options {
	return Options{
		EPUB:    epub.DefaultOptions(),
		Timeout: 5 * time.Minute,
	}
}

func applyDefaults(opts Options) Options {
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Minute
	}
	return opts
}

// ErrCalibreNotFound wraps a failed ebook-convert lookup so callers can
// distinguish "Calibre isn't installed" from other resolution errors.
var ErrCalibreNotFound = errors.New("ebook-convert not found")

// ResolveCalibre finds the ebook-convert executable: explicitPath if set
// (existence-checked, used as-is — mirrors chromemgr.EnsureChrome's
// explicit-path-wins resolution order), else a PATH lookup.
func ResolveCalibre(explicitPath string) (string, error) {
	if explicitPath != "" {
		info, err := os.Stat(explicitPath)
		if err != nil {
			return "", fmt.Errorf("calibre-path %q: %w", explicitPath, err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("calibre-path %q is a directory, not an executable", explicitPath)
		}
		return explicitPath, nil
	}

	path, err := exec.LookPath("ebook-convert")
	if err != nil {
		return "", fmt.Errorf("%w: install Calibre (https://calibre-ebook.com/download) so ebook-convert is on PATH, or pass --calibre-path", ErrCalibreNotFound)
	}
	return path, nil
}

// Write composes docs into an intermediate EPUB (the same content/theme a
// direct EPUB build would produce) and converts it to a Kindle-ready file
// at outputPath via Calibre's ebook-convert. ebook-convert infers the
// target format (MOBI, AZW3, ...) from outputPath's own extension.
func Write(ctx context.Context, docs []*mdx.Document, opts Options, outputPath string) error {
	opts = applyDefaults(opts)

	calibrePath, err := ResolveCalibre(opts.CalibrePath)
	if err != nil {
		return err
	}

	outDir := filepath.Dir(outputPath)
	if mkdirErr := os.MkdirAll(outDir, 0755); mkdirErr != nil {
		return fmt.Errorf("creating output directory: %w", mkdirErr)
	}

	tmpEpub, err := os.CreateTemp(outDir, filepath.Base(outputPath)+".tmp-*.epub")
	if err != nil {
		return fmt.Errorf("creating temp EPUB file: %w", err)
	}
	tmpEpubPath := tmpEpub.Name()
	_ = tmpEpub.Close()
	defer func() { _ = os.Remove(tmpEpubPath) }()

	if writeErr := epub.Write(docs, opts.EPUB, tmpEpubPath); writeErr != nil {
		return fmt.Errorf("building intermediate EPUB: %w", writeErr)
	}

	// ebook-convert infers the target format from the destination
	// filename's extension, so the temp output file must keep outputPath's
	// own extension rather than a generic ".tmp" suffix.
	ext := filepath.Ext(outputPath)
	tmpOut, err := os.CreateTemp(outDir, filepath.Base(outputPath)+".tmp-*"+ext)
	if err != nil {
		return fmt.Errorf("creating temp output file: %w", err)
	}
	tmpOutPath := tmpOut.Name()
	_ = tmpOut.Close()
	defer func() { _ = os.Remove(tmpOutPath) }()

	cctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, calibrePath, tmpEpubPath, tmpOutPath)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		if errors.Is(cctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("ebook-convert timed out after %s", opts.Timeout)
		}
		return fmt.Errorf("ebook-convert failed: %w\n%s", runErr, out)
	}

	if err := os.Rename(tmpOutPath, outputPath); err != nil {
		return fmt.Errorf("finalizing output file: %w", err)
	}
	return nil
}

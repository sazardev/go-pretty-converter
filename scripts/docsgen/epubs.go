package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sazardev/go-pretty-pdf/epub"
	"github.com/sazardev/go-pretty-pdf/mdx"
	"github.com/sazardev/go-pretty-pdf/theme"
)

// docsEPUBDefault is the canonical, stable download URL for the default
// (classic) theme's EPUB, mirroring docsPDFDefault.
const docsEPUBDefault = "go-pretty-pdf-docs.epub"

// docsEPUBFilename returns the per-theme EPUB artifact name, mirroring
// docsPDFFilename.
func docsEPUBFilename(themeID string) string {
	return "go-pretty-pdf-docs-" + themeID + ".epub"
}

// buildDocsEPUBs renders the same three docs sources (README + CLI + changelog)
// into one EPUB 3 per builtin theme. EPUB needs no Chrome at all, so these
// are cheap and run fully concurrently — unlike the PDF pipeline there's no
// per-process browser launch to serialize, just a zip write per theme.
func buildDocsEPUBs(outDir string, docs []*mdx.Document, log *buildLogger, jobs int) []renderResult {
	themes := theme.List()
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]renderResult, 0, len(themes))

	for _, t := range themes {
		wg.Add(1)
		go func(t theme.Theme) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			outPath := filepath.Join(outDir, docsEPUBFilename(t.Name))
			t0 := time.Now()

			css, err := theme.ResolveForEPUB(t, theme.Options{})
			res := renderResult{name: t.Name, group: groupEPUB, elapsed: time.Since(t0)}
			if err == nil {
				opts := epub.DefaultOptions()
				opts.Title = siteName
				opts.Subtitle = "Write Markdown. Ship a book."
				opts.Author = "sazardev"
				opts.Language = "en"
				opts.CSS = css
				err = epub.Write(docs, opts, outPath)
			}
			res.ok = err == nil
			if err != nil {
				res.err = err
			} else if info, serr := os.Stat(outPath); serr == nil {
				res.bytes = info.Size()
				res.note = formatBytes(int(info.Size()))
			}
			if res.ok && t.Name == theme.NameClassic {
				if cerr := copyFile(outPath, filepath.Join(outDir, docsEPUBDefault)); cerr != nil {
					res.note = "built; could not stage default copy: " + cerr.Error()
				}
			}

			mu.Lock()
			results = append(results, res)
			mu.Unlock()

			if res.ok {
				log.Vf("  ✓ epub %-16s %-10s %s", t.Name, res.note, res.elapsed.Round(time.Millisecond))
			} else {
				log.Vf("  ✗ epub %-16s %v", t.Name, res.err)
			}
		}(t)
	}

	wg.Wait()
	return results
}

// parseDocsSource parses the three docs sources (already written as MDX in
// srcDir by prepareDocsSource) into documents once, so both the PDF and
// EPUB builders share the same parsed content instead of re-parsing per
// theme.
func parseDocsSource(srcDir string) ([]*mdx.Document, error) {
	p := mdx.NewParser()
	docs, err := p.ParseDir(srcDir)
	if err != nil && len(docs) == 0 {
		return nil, fmt.Errorf("parsing docs source: %w", err)
	}
	return docs, nil
}

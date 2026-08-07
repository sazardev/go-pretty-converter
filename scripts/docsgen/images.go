package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// generateRasterAssets renders the apple-touch-icon, favicon, PWA icons,
// and the Open Graph card as PNGs using headless Chrome. Each image is
// rendered in its own browser tab on a single shared Chrome process, up to
// jobs concurrent tabs at a time — far faster than one sequential browser
// round-trip per image. It is best-effort: docsgen must still produce a
// working site for contributors who don't have Chrome/Chromium installed
// locally, so a failure is reported and skipped rather than aborting the
// build. CI always has Chrome available (installed one step earlier in
// docs.yml), so production deploys always get real images.
func generateRasterAssets(outDir string, log *buildLogger, jobs int) []renderResult {
	var results []renderResult

	allocCtx, allocCancel := chromedp.NewExecAllocator(
		context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.DisableGPU,
			chromedp.NoSandbox,
			chromedp.Headless,
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.WSURLReadTimeout(45*time.Second),
		)...,
	)
	defer allocCancel()

	// Verify Chrome boots before fanning out: a missing binary would
	// otherwise make every tab fail with the same confusing error.
	probeCtx, probeCancel := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(probeCtx); err != nil {
		probeCancel()
		log.Warnf("Chrome unavailable, skipping favicon/OG/icon generation: %v", err)
		return results
	}
	probeCancel()

	renders := []struct {
		name          string
		width, height int64
		html          string
	}{
		{"apple-touch-icon.png", 180, 180, appleTouchIconHTML()},
		{"favicon-32.png", 32, 32, appleTouchIconHTML()},
		{"icon-192.png", 192, 192, appleTouchIconHTML()},
		{"icon-512.png", 512, 512, appleTouchIconHTML()},
		{"og-image.png", 1200, 630, ogImageHTML()},
	}

	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, r := range renders {
		wg.Add(1)
		go func(name string, width, height int64, html string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			t0 := time.Now()
			// Each tab gets its own chromedp context on the shared
			// allocator; tabs are independent and run concurrently.
			tabCtx, cancel := chromedp.NewContext(allocCtx)
			defer cancel()
			tabCtx, cancel = context.WithTimeout(tabCtx, 45*time.Second)
			defer cancel()

			buf, err := screenshotHTML(tabCtx, html, width, height)
			res := renderResult{name: name, elapsed: time.Since(t0), ok: err == nil}
			if err != nil {
				res.err = fmt.Errorf("render: %w", err)
			} else if werr := os.WriteFile(filepath.Join(outDir, name), buf, 0644); werr != nil {
				res.err = fmt.Errorf("write: %w", werr)
				res.ok = false
			} else {
				res.note = formatBytes(len(buf))
			}

			mu.Lock()
			results = append(results, res)
			mu.Unlock()
			if res.ok {
				log.Vf("  ✓ %-24s %s", name, res.note)
			} else {
				log.Vf("  ✗ %-24s %v", name, res.err)
			}
		}(r.name, r.width, r.height, r.html)
	}

	wg.Wait()
	return results
}

func formatBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func screenshotHTML(ctx context.Context, htmlContent string, width, height int64) ([]byte, error) {
	encoded := base64.StdEncoding.EncodeToString([]byte(htmlContent))
	dataURI := "data:text/html;charset=utf-8;base64," + encoded

	var buf []byte
	tasks := chromedp.Tasks{
		chromedp.EmulateViewport(width, height),
		chromedp.Navigate(dataURI),
		chromedp.CaptureScreenshot(&buf),
	}
	if err := chromedp.Run(ctx, tasks...); err != nil {
		return nil, fmt.Errorf("chromedp screenshot: %w", err)
	}
	return buf, nil
}

// appleTouchIconHTML renders the same "> _" ink-on-paper mark as
// favicon.svg, scaled to fill whatever viewport it's captured at.
func appleTouchIconHTML() string {
	return `<!DOCTYPE html><html><head><meta charset="utf-8"><style>
html,body{margin:0;padding:0;background:#282828;height:100%;}
.mark{width:100vw;height:100vh;display:flex;align-items:center;justify-content:center;}
.mark span{
  font-family:ui-monospace,'SF Mono','JetBrains Mono',Consolas,'Courier New',monospace;
  font-weight:700;color:#fe8019;font-size:58vw;line-height:1;
}
</style></head><body><div class="mark"><span>&gt;_</span></div></body></html>`
}

// ogImageHTML is the 1200x630 social-share card, using the same gruvbox
// tokens (bg/ink/accent) the landing page previews by default — see
// landingDefaultTheme in landing.go.
func ogImageHTML() string {
	return `<!DOCTYPE html><html><head><meta charset="utf-8"><style>
html,body{margin:0;padding:0;width:1200px;height:630px;background:#282828;}
.card{
  width:1200px;height:630px;box-sizing:border-box;
  padding:80px 90px;display:flex;flex-direction:column;justify-content:center;
  border-left:14px solid #fe8019;
  font-family:Georgia,'Iowan Old Style','Palatino',serif;
}
.eyebrow{
  font-family:ui-monospace,'SF Mono','JetBrains Mono',Consolas,'Courier New',monospace;
  font-size:22px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;
  color:#fe8019;margin-bottom:22px;
}
.title{font-size:80px;font-weight:700;color:#ebdbb2;margin-bottom:26px;line-height:1;}
.title em{font-style:italic;color:#fe8019;}
.tagline{font-size:32px;color:#d5c4a1;line-height:1.5;max-width:920px;}
.footer{
  margin-top:48px;font-family:ui-monospace,'SF Mono','JetBrains Mono',Consolas,'Courier New',monospace;
  font-size:22px;color:#a89984;
}
</style></head><body>
<div class="card">
  <div class="eyebrow">go &middot; CLI + library</div>
  <div class="title">Write Markdown. Ship a beautiful <em>PDF.</em></div>
  <div class="tagline">pretty-pdf turns a folder of Markdown into a typeset, print-ready PDF via headless Chrome.</div>
  <div class="footer">go install github.com/sazardev/go-pretty-pdf/cmd/pretty-pdf@latest</div>
</div>
</body></html>`
}

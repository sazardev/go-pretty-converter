package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
)

// generateRasterAssets renders the apple-touch-icon and Open Graph card as
// PNGs using headless Chrome, mirroring the allocator setup in render.go.
// It is best-effort: docsgen must still produce a working site for
// contributors who don't have Chrome/Chromium installed locally, so a
// failure here is logged and skipped rather than aborting the build. CI
// always has Chrome available (installed one step earlier in docs.yml), so
// production deploys always get real images.
func generateRasterAssets(outDir string) {
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

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := chromedp.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: Chrome unavailable, skipping favicon/OG image generation: %v\n", err)
		return
	}

	renders := []struct {
		name          string
		width, height int64
		html          string
	}{
		{"apple-touch-icon.png", 180, 180, appleTouchIconHTML()},
		{"favicon-32.png", 32, 32, appleTouchIconHTML()},
		{"og-image.png", 1200, 630, ogImageHTML()},
	}

	for _, r := range renders {
		buf, err := screenshotHTML(ctx, r.html, r.width, r.height)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to render %s: %v\n", r.name, err)
			continue
		}
		if err := os.WriteFile(filepath.Join(outDir, r.name), buf, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write %s: %v\n", r.name, err)
		}
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

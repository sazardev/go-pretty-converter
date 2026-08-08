package render

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/chromedp/chromedp"
)

// TestSharedBrowserReusesOneChrome verifies that RenderToPDFWithAuditBrowser
// renders correctly on a browser context created once and reused for
// multiple documents — the correctness contract of WithSharedBrowser.
//
// Note: it does NOT assert a speedup. Measured on this machine, sharing a
// browser is within noise of booting a fresh Chrome per render (roughly
// 0.94-1.02x), because each chromedp.NewContext still pays a CDP handshake
// and the ~400ms Chrome startup is a small fraction once render time
// dominates. The real parallelism win comes from running renders
// concurrently (each with its own Chrome), not from reusing one browser.
func TestSharedBrowserReusesOneChrome(t *testing.T) {
	requireChrome(t)

	html := `<html><body><h1>Hello</h1><p>Some body text.</p></body></html>`
	dir := t.TempDir()

	alloc, cancel := NewBrowser(context.Background(), DefaultOptions())
	defer cancel()

	for i := 0; i < 3; i++ {
		tab, tabCancel := chromedp.NewContext(alloc)
		_, err := RenderToPDFWithAuditBrowser(tab, html, filepath.Join(dir, fmt.Sprintf("shared-%d.pdf", i)), DefaultOptions())
		tabCancel()
		if err != nil {
			t.Fatalf("shared render %d failed: %v", i, err)
		}
	}
}

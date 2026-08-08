package render

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPostPrintFlagCost measures the render-time impact of the two
// post-print PDF steps (outline + tagging) on a large document, to inform
// the defaults and the speed flags. Logs the three configurations; not
// asserted, so it's a diagnostic rather than a flaky timing test.
func TestPostPrintFlagCost(t *testing.T) {
	requireChrome(t)

	body := strings.Repeat("<h2>Heading</h2><p>Some paragraph text that gives the print engine real work to paginate and the tagger structure to emit.</p>", 1500)
	html := `<html><body>` + body + `</body></html>`
	dir := t.TempDir()

	type conf struct {
		name    string
		outline bool
		tagged  bool
	}
	configs := []conf{
		{"both (default)", true, true},
		{"no outline", false, true},
		{"no tagged", true, false},
		{"neither (fastest)", false, false},
	}

	ctx := context.Background()
	for _, c := range configs {
		opts := DefaultOptions()
		opts.GenerateDocumentOutline = c.outline
		opts.GenerateTaggedPDF = c.tagged

		// warm-up then time the real render
		_ = func() error {
			_, err := RenderToPDFWithAuditContext(ctx, html, filepath.Join(dir, "warmup.pdf"), opts)
			return err
		}()
		start := time.Now()
		_, err := RenderToPDFWithAuditContext(ctx, html, filepath.Join(dir, c.name+".pdf"), opts)
		el := time.Since(start)
		if err != nil {
			t.Fatalf("%s render failed: %v", c.name, err)
		}
		t.Logf("%-22s %s", c.name, el.Round(time.Millisecond))
	}
}

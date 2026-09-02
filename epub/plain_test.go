package epub

import (
	"strings"
	"testing"
)

func TestFlattenCodeBlocksStripsTokenSpans(t *testing.T) {
	fragment := `<p>Intro text.</p>
<pre class="chroma-chroma"><code><span class="chroma-line"><span class="chroma-cl"><span class="chroma-kn">package</span><span class="chroma-w"> </span><span class="chroma-nx">main</span><span class="chroma-w">
</span></span></span><span class="chroma-line"><span class="chroma-cl"><span class="chroma-kd">func</span><span class="chroma-w"> </span><span class="chroma-func">main</span><span class="chroma-p">()</span><span class="chroma-w"> </span><span class="chroma-p">{</span><span class="chroma-w">
</span></span></span></code></pre>
<p>Trailing text.</p>`

	got, err := flattenCodeBlocks(fragment)
	if err != nil {
		t.Fatalf("flattenCodeBlocks: %v", err)
	}
	if strings.Contains(got, "<span") {
		t.Errorf("expected all token spans stripped, got:\n%s", got)
	}
	wantCode := "package main\nfunc main() {"
	if !strings.Contains(got, wantCode) {
		t.Errorf("expected code text %q preserved, got:\n%s", wantCode, got)
	}
	if !strings.Contains(got, "<p>Intro text.</p>") || !strings.Contains(got, "<p>Trailing text.</p>") {
		t.Errorf("expected surrounding paragraphs untouched, got:\n%s", got)
	}
}

func TestFlattenCodeBlocksUnwrappedPre(t *testing.T) {
	fragment := `<pre><code>git clone https://example.com/repo.git
cd repo</code></pre>`
	got, err := flattenCodeBlocks(fragment)
	if err != nil {
		t.Fatalf("flattenCodeBlocks: %v", err)
	}
	if !strings.Contains(got, "git clone https://example.com/repo.git") {
		t.Errorf("expected plain pre content to survive, got:\n%s", got)
	}
}

func TestValidateWellFormedXHTML(t *testing.T) {
	valid := `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"><body><p>Hello &amp; goodbye &#8212; ok</p><br/></body></html>`
	if err := validateWellFormedXHTML(valid); err != nil {
		t.Errorf("valid XHTML rejected: %v", err)
	}

	invalid := `<html><body><p>This is & broken</body></html>`
	if err := validateWellFormedXHTML(invalid); err == nil {
		t.Error("expected malformed XHTML to fail validation")
	}
}

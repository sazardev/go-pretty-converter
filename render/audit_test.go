package render

import "testing"

func TestAuditPDFBytesEmpty(t *testing.T) {
	issues := auditPDFBytes(nil)
	if len(issues) == 0 {
		t.Fatal("expected issues for empty buffer")
	}
	for _, i := range issues {
		if !i.HasError() {
			t.Errorf("empty PDF issues should be errors, got %s (%s)", i.Check, i.Severity)
		}
	}
}

func TestAuditPDFBytesMissingEOFPagesPresent(t *testing.T) {
	// A buffer with page objects but no trailing %%EOF must be flagged as
	// an error (truncation), even though page counting passes.
	buf := []byte("%PDF-1.4\n1 0 obj\n/Type /Page\nendobj\n%%EOFXX")
	issues := auditPDFBytes(buf)
	found := map[string]bool{}
	for _, i := range issues {
		found[i.Check] = true
	}
	if !found["pdf-eof-missing"] {
		t.Errorf("expected pdf-eof-missing issue, got %v", found)
	}
	if found["page-count"] {
		t.Errorf("page-count should pass when /Type /Page objects exist, got %v", found)
	}
}

func TestAuditPDFBytesEOFWithTrailingNewline(t *testing.T) {
	// Chrome appends a final newline after %%EOF — must NOT be flagged.
	buf := []byte("%PDF-1.4\n1 0 obj\n/Type /Page\nendobj\n%%EOF\n")
	if issues := auditPDFBytes(buf); len(issues) != 0 {
		t.Errorf("trailing-newline EOF should pass, got %v", issues)
	}
}

func TestAuditPDFBytesNoPages(t *testing.T) {
	buf := []byte("%PDF-1.4\nnot a real pdf\n%%EOF")
	issues := auditPDFBytes(buf)
	found := map[string]bool{}
	for _, i := range issues {
		found[i.Check] = true
		if !i.HasError() {
			t.Errorf("no-pages should be an error, got %s", i.Severity)
		}
	}
	if !found["page-count"] {
		t.Errorf("expected page-count issue, got %v", found)
	}
}

func TestAuditPDFBytesClean(t *testing.T) {
	buf := []byte("%PDF-1.4\n1 0 obj\n/Type /Page\nendobj\n%%EOF")
	if issues := auditPDFBytes(buf); len(issues) != 0 {
		t.Errorf("clean buffer should have no issues, got %v", issues)
	}
}

func TestCountPDFPagesExcludesPagesTree(t *testing.T) {
	buf := []byte("/Type /Page /Type /Pages /Type /Page")
	if got := countPDFPages(buf); got != 2 {
		t.Errorf("expected 2 page objects (excluding /Pages), got %d", got)
	}
}

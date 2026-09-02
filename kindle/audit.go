package kindle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/sazardev/go-pretty-converter/mdx"
)

// Severity classifies an Audit Finding, mirroring render.Severity: a
// Kindle build that fails to round-trip its own text is a real defect,
// not an advisory.
type Severity string

const (
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Finding is one damaged spot detected in a finished Kindle file.
type Finding struct {
	Severity Severity
	// Chapter names the affected document (its [X.Y.Z] id), or "" for
	// findings that span the whole book.
	Chapter string
	Message string
}

// Audit is the outcome of VerifyConverted: every problem found in the
// converted file's text layer. A clean conversion returns an Audit with
// no Error-severity findings.
type Audit struct {
	Findings []Finding
}

// OK reports whether no Error-severity finding was recorded — the
// signal callers use to decide whether the converted file is safe to
// ship.
func (a *Audit) OK() bool {
	for _, f := range a.Findings {
		if f.Severity == SeverityError {
			return false
		}
	}
	return true
}

func (a *Audit) add(sev Severity, chapter, format string, args ...any) {
	a.Findings = append(a.Findings, Finding{
		Severity: sev,
		Chapter:  chapter,
		Message:  fmt.Sprintf(format, args...),
	})
}

// leakedMarkupRe matches markup fragments that must never appear in a
// Kindle file's extracted text layer. `aid="..."` spans are injected by
// Calibre's own KF8 conversion (one per text fragment) — always
// invisible; if one becomes visible text, the device's renderer dumped
// raw markup. `class="chroma-` / `chroma-` are this project's
// syntax-highlighting token classes — ditto. Neither string is
// plausible in authored prose or code samples.
var leakedMarkupRe = regexp.MustCompile(`aid=|class="chroma-|chroma-(kd|w|s|p|k|n|o|cl)\b`)

// DefaultVerifyTimeout bounds how long extracting the converted file's
// text layer may take; a 150-chapter book extracts in well under it.
const DefaultVerifyTimeout = 5 * time.Minute

// VerifyConverted checks a finished Kindle file (MOBI/AZW3) for
// conversion damage, using Calibre itself to extract the file's text
// layer — the same render path a device would follow, minus the screen.
// It verifies two things:
//
//  1. No markup leak: raw HTML/KF8 fragments (`aid=`, `class="chroma-`,
//     highlight token classes) appearing in visible text. Kindle
//     renderers leak exactly this when handed markup they can't
//     reflow — the splices and `class="chroma-w" aid="4GULHE">`-style
//     garbage this audit exists to catch.
//  2. Chapter integrity: every source document's title must survive the
//     conversion into the extracted text. A missing title means the
//     conversion mangled or dropped that chapter's content.
//
// VerifyConverted needs Calibre (same ebook-convert Verify's Write phase
// already used). Callers that can't afford the extraction round-trip
// (it takes seconds to a couple of minutes depending on book size) can
// skip it and rely on the EPUB-side checks Write itself performs.
func VerifyConverted(ctx context.Context, calibrePath, outputPath string, docs []*mdx.Document, timeout time.Duration) (*Audit, error) {
	audit := &Audit{}
	if timeout <= 0 {
		timeout = DefaultVerifyTimeout
	}

	tmp, err := os.CreateTemp(filepath.Dir(outputPath), filepath.Base(outputPath)+".verify-*.txt")
	if err != nil {
		return nil, fmt.Errorf("creating verification temp file: %w", err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, calibrePath, outputPath, tmpPath)
	if out, runErr := cmd.CombinedOutput(); runErr != nil {
		if cctx.Err() != nil {
			return nil, fmt.Errorf("verification timed out after %s extracting text from %s (use --no-verify to skip)", timeout, outputPath)
		}
		return nil, fmt.Errorf("verification failed extracting text from %s: %w\n%s", outputPath, runErr, out)
	}

	textBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("reading extracted text: %w", err)
	}

	auditScanText(audit, string(textBytes), docs)
	return audit, nil
}

// auditScanText runs the leak and chapter-integrity checks over an
// extracted text layer. Split out from VerifyConverted so the checks are
// unit-testable without Calibre.
func auditScanText(audit *Audit, text string, docs []*mdx.Document) {
	text = normalizeText(text)

	for _, loc := range leakedMarkupRe.FindAllStringIndex(text, -1) {
		snippet := snippetAround(text, loc[0])
		audit.add(SeverityError, "", "markup leaked into visible text near %q — broken conversion", snippet)
	}

	// Every chapter heading must have survived. Titles are matched
	// against text normalized the same way so line breaks and case
	// differences don't cause false alarms.
	for _, doc := range docs {
		title := normalizeText(doc.Title())
		if title == "" {
			continue
		}
		if !strings.Contains(text, title) {
			audit.add(SeverityError, doc.ID(), "chapter %s %q is missing from the converted file — its content was mangled or dropped during conversion", doc.ID(), title)
		}
	}
}

// normalizeText lowercases and collapses runs of whitespace, so heading
// matches survive case and reflow differences between the source title
// and the extracted text.
func normalizeText(s string) string {
	return strings.ToLower(strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == '\u00a0'
	}), " "))
}

func snippetAround(text string, idx int) string {
	start := idx - 40
	if start < 0 {
		start = 0
	}
	end := idx + 40
	if end > len(text) {
		end = len(text)
	}
	return text[start:end]
}

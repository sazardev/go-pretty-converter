package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/chromedp/chromedp"
)

// Severity classifies an audit Issue. The audit is advisory by default —
// every finding is a warning, never a hard failure — but a handful of
// checks (a corrupt or empty output, for instance) report at SeverityError
// so callers can decide to fail the build on them.
type Severity string

const (
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Issue is one finding from a post-compose/pre-print visual/structural
// audit of the document: something that renders but is likely wrong —
// clipped, overlapping, unreadable, or missing — rather than a hard error.
type Issue struct {
	// Check is a short, stable, machine-readable identifier for the rule
	// that produced this issue (e.g. "overflow-x", "low-contrast",
	// "heading-clip-risk"), suitable for filtering or documentation links.
	Check    string
	Severity Severity
	Message  string
}

// HasError reports whether any finding reached SeverityError — the signal
// callers can use to fail a build on output they consider corrupt.
func (i Issue) HasError() bool {
	return i.Severity == SeverityError
}

// AuditReport collects every Issue found while rendering a single document.
// A nil report or one with no Issues means the audit found nothing to flag
// — it does not guarantee the PDF is perfect, only that none of the checks
// it runs caught a problem.
type AuditReport struct {
	Issues []Issue
}

// HasIssues reports whether the audit found anything worth surfacing to the
// caller. Safe to call on a nil report.
func (r *AuditReport) HasIssues() bool {
	return r != nil && len(r.Issues) > 0
}

// domAuditJS runs inside the already-navigated document, before
// Page.printToPDF, and returns a JSON array of {check, severity, message}
// objects. It checks what's actually observable from the DOM/CSSOM at this
// stage:
//
//   - overflow-x: content wider than its own box (long code lines, wide
//     tables, oversized images) that print will most likely clip instead
//     of wrapping, since printed pages don't get a horizontal scrollbar.
//   - overflow-y: content taller than its own box. Only meaningful for
//     elements with a bounded height (a fixed-height table cell or an
//     image with explicit dimensions); unbounded block elements are
//     skipped to avoid noise from normal page-length flows.
//   - broken-image: an <img> whose source never resolved to real pixels.
//   - empty-content: the whole document has next to no visible text,
//     usually a sign composition/parsing silently produced nothing.
//   - low-contrast: visible text whose color is too close to its
//     effective background to read comfortably (a common symptom of a
//     custom --color-* override clashing with a theme's own palette).
//     Uses WCAG 2.2 thresholds: 4.5:1 for normal text, 3:1 for large text
//     (>= 18.66px normal or >= 14pt bold).
//   - heading-clip-risk: an element that forces a page break
//     (page-break-before/break-before) but doesn't keep enough top margin
//     to clear the ~0.3in strip chrome-headless-shell silently clips
//     whenever a header or page-number footer is displayed — see
//     TestBaseCSSH1HasTopMarginBuffer and the CHANGELOG entry it guards.
//     This mirrors a real, previously-shipped bug so custom themes/CSS get
//     the same protection builtin themes now have.
//   - broken-anchor: an <a href="#fragment"> pointing at an id that
//     doesn't exist anywhere in the document (dead in-document links break
//     both the TOC and PDF bookmarks).
//   - duplicate-id: the same id attribute appears more than once, which
//     breaks anchor navigation, the TOC, and PDF bookmarks (they resolve
//     to the first match only).
//   - toc-mismatch: the table of contents lists a section id that has no
//     matching element, or an in-body section heading never got a TOC
//     entry. Only runs when a TOC is actually present.
//   - font-load-fail: a font family the page asks for could not be
//     resolved by the browser (missing local font, or a Google Fonts
//     request blocked by the default network lockdown). The PDF then
//     renders with a silent fallback that may look nothing like the theme
//     intended.
//   - image-low-res: an image displayed much larger than its intrinsic
//     size, so it will look pixelated when printed.
//   - page-break-inside-risk: a table or code block without
//     page-break-inside/break-inside: avoid, which print will happily
//     slice mid-row/mid-line.
//   - line-break-risk: a block whose CSS orphans/widows are below the
//     print-safe minimum, so a heading or paragraph can strand a single
//     line at the bottom/top of a page.
//
// On large documents several checks sample the first N matching elements
// (N is a few thousand) rather than walking the entire tree, because
// style reads (getComputedStyle, scrollWidth/Height) force layout and get
// very expensive on documents with tens of thousands of nodes — the
// findings are deduped and capped anyway, so the same signal is captured
// with bounded cost.
//
// It deliberately cannot see the two things that live purely in Chrome's
// print engine rather than the DOM: the fixed ~0.2in header/footer inset,
// and how the browser's print pagination actually slices this content
// into pages. Those are covered by base.css's own layout rules (and their
// regression tests), not by this runtime audit.
const domAuditJS = `(() => {
  const issues = [];
  const needsHeaderFooter = %t;

  function pushIssue(check, message, severity) {
    issues.push({check, severity: severity || 'warning', message});
  }

  function elLabel(el) {
    return el.id ? ('#' + el.id) : ('<' + el.tagName.toLowerCase() + '>');
  }

  // overflow-x AND overflow-y for bounded elements. Sampled: a huge
  // document can have tens of thousands of these, and scrollWidth/Height
  // force layout reads; capping the scan keeps the audit fast without
  // losing signal (the same overflow pattern repeats across a document).
  const maxScrollScan = 5000;
  const scrollableSel = 'pre, code, table, img, .component-deep-dive, .component-warning, .component-axiom';
  const scrollEls = document.querySelectorAll(scrollableSel);
  const scrollCount = Math.min(scrollEls.length, maxScrollScan);
  for (let si = 0; si < scrollCount; si++) {
    const el = scrollEls[si];
    const cs = getComputedStyle(el);
    if (el.scrollWidth > el.clientWidth + 2) {
      pushIssue('overflow-x', elLabel(el) + ' is wider than its box (' + el.scrollWidth + 'px vs ' + el.clientWidth + 'px) and may be clipped when printed');
    }
    // overflow-y is only a real problem when the element has a fixed
    // height that clips it; an unbounded block flows to page length.
    if (el.scrollHeight > el.clientHeight + 2 && cs.height !== 'auto' && cs.maxHeight !== 'none') {
      pushIssue('overflow-y', elLabel(el) + ' is taller than its box (' + el.scrollHeight + 'px vs ' + el.clientHeight + 'px) and its content may be clipped when printed');
    }
  }

  document.querySelectorAll('img').forEach(img => {
    if (img.complete && img.naturalWidth === 0) {
      const src = (img.getAttribute('src') || '(no src)').slice(0, 80);
      pushIssue('broken-image', 'image failed to load: ' + src);
    } else if (img.complete && img.naturalWidth > 0) {
      // Image upscaled beyond ~2x intrinsic size will look pixelated on
      // paper. Only flag when there's an actual rendered box to compare.
      const renderedW = img.getBoundingClientRect().width;
      if (renderedW > img.naturalWidth * 2 + 1) {
        const src = (img.getAttribute('src') || '(no src)').slice(0, 80);
        pushIssue('image-low-res', 'image displayed at ' + Math.round(renderedW) + 'px but is only ' + img.naturalWidth + 'px wide — will look pixelated when printed: ' + src);
      }
    }
  });

  const textLen = (document.body.innerText || '').trim().length;
  if (textLen < 20) {
    pushIssue('empty-content', 'document has almost no visible text (' + textLen + ' characters) — composition or rendering may have failed');
  }

  // in-document fragment integrity
  const ids = new Map();
  document.querySelectorAll('[id]').forEach(el => {
    if (ids.has(el.id)) {
      pushIssue('duplicate-id', 'duplicate id "' + el.id + '" on ' + elLabel(el) + ' (also on ' + elLabel(ids.get(el.id)) + ')');
    } else {
      ids.set(el.id, el);
    }
  });
  document.querySelectorAll('a[href^="#"]').forEach(a => {
    const frag = a.getAttribute('href').slice(1);
    if (frag && !document.getElementById(frag)) {
      pushIssue('broken-anchor', 'link to #' + frag + ' has no matching element: "' + (a.textContent || '').trim().slice(0, 50) + '"');
    }
  });

  // TOC integrity — only when a TOC is present.
  const tocLinks = document.querySelectorAll('.toc a[href^="#"]');
  if (tocLinks.length > 0) {
    const tocTargets = new Set();
    tocLinks.forEach(a => tocTargets.add(a.getAttribute('href').slice(1)));
    tocLinks.forEach(a => {
      const frag = a.getAttribute('href').slice(1);
      if (!document.getElementById(frag)) {
        pushIssue('toc-mismatch', 'TOC links to #' + frag + ' but no element with that id exists');
      }
    });
    // Every top-level section heading in the body should have a TOC entry.
    // Sampled: the TOC itself is capped by the number of docs, but a
    // document with thousands of sections makes this scan O(headings);
    // capping keeps it bounded.
    const heads = document.querySelectorAll('h1, h2, h3');
    const hcap = Math.min(heads.length, 3000);
    for (let hi = 0; hi < hcap; hi++) {
      const h = heads[hi];
      const sec = h.closest('section');
      if (!sec || !sec.id) continue;
      if (!tocTargets.has(sec.id)) {
        pushIssue('toc-mismatch', 'section #' + sec.id + ' ("' + (h.textContent || '').trim().slice(0, 40) + '") has no TOC entry');
      }
    }
  }

  // font availability — report families the page asks for but the browser
  // can't resolve (blocked network fonts, missing local fonts). Sampling:
  // walking every element and calling getComputedStyle on each is O(n) with
  // a huge constant (style recalc per element), which on a document with
  // thousands of sections can dominate the whole audit. Instead, collect
  // the distinct font-family *declarations* from the stylesheet and probe
  // each against document.fonts — a font that's genuinely missing is
  // declared somewhere in CSS, so this keeps full coverage at O(unique
  // families) instead of O(elements).
  const familyRe = /^(['"])(.+)\1$/;
  const fontFamilies = new Set();
  document.querySelectorAll('style').forEach(style => {
    const css = style.textContent || '';
    css.replace(/font-family\s*:\s*([^;}]+)/gi, (m, list) => {
      list.split(',').forEach(part => {
        const p = part.trim();
        let name = p;
        const mm = name.match(familyRe);
        if (mm) name = mm[2];
        if (name && !/^(sans-serif|serif|monospace|system-ui|cursive|fantasy|math|ui-.*)$/.test(name)) {
          fontFamilies.add(name);
        }
      });
      return m;
    });
  });
  fontFamilies.forEach(name => {
    try {
      if (document.fonts && document.fonts.check && !document.fonts.check('16px "' + name + '"')) {
        pushIssue('font-load-fail', 'font "' + name + '" could not be loaded — will fall back to another family');
      }
    } catch (e) {}
  });

  function parseColor(str) {
    const m = str && str.match(/rgba?\(([^)]+)\)/);
    if (!m) return null;
    const parts = m[1].split(',').map(s => parseFloat(s.trim()));
    if (parts.length < 3 || parts.some(isNaN)) return null;
    return {r: parts[0], g: parts[1], b: parts[2], a: parts.length > 3 ? parts[3] : 1};
  }
  function luminance(c) {
    const chan = v => {
      v /= 255;
      return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
    };
    return 0.2126 * chan(c.r) + 0.7152 * chan(c.g) + 0.0722 * chan(c.b);
  }
  function effectiveBg(el) {
    let node = el;
    while (node) {
      const bg = parseColor(getComputedStyle(node).backgroundColor);
      if (bg && bg.a > 0.01) return bg;
      node = node.parentElement;
    }
    return {r: 255, g: 255, b: 255, a: 1};
  }
  function isLargeText(el) {
    const cs = getComputedStyle(el);
    const px = parseFloat(cs.fontSize);
    const bold = parseInt(cs.fontWeight, 10) >= 700;
    return px >= 18.66 || (px >= 14 && bold);
  }
  // WCAG 2.2 minimum contrast: 4.5:1 normal text, 3:1 large text.
  function contrastThreshold(el) {
    return isLargeText(el) ? 3 : 4.5;
  }

  // low-contrast is sampled, not exhaustive: effectiveBg walks up the
  // ancestor chain calling getComputedStyle per text node, which is fine
  // on a normal book but O(n·depth) and style-recalc-heavy on documents
  // with tens of thousands of nodes. Since findings are deduped and capped
  // at 5, scanning the whole tree never yields more signal — sampling the
  // first maxContrastNodes text nodes keeps the cost bounded on huge
  // documents while still surfacing the same palette collisions.
  const maxContrastNodes = 3000;
  const seenContrast = new Set();
  let contrastIssues = 0;
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  let node;
  let scanned = 0;
  while ((node = walker.nextNode()) && contrastIssues < 5 && scanned < maxContrastNodes) {
    scanned++;
    const text = node.textContent.trim();
    if (text.length < 2) continue;
    const el = node.parentElement;
    if (!el) continue;
    const style = getComputedStyle(el);
    if (style.visibility === 'hidden' || style.display === 'none' || parseFloat(style.opacity) === 0) continue;
    const fg = parseColor(style.color);
    if (!fg || fg.a < 0.5) continue;
    const bg = effectiveBg(el);
    const ratio = (Math.max(luminance(fg), luminance(bg)) + 0.05) / (Math.min(luminance(fg), luminance(bg)) + 0.05);
    const key = style.color + '|' + JSON.stringify(bg);
    if (ratio < contrastThreshold(el) && !seenContrast.has(key)) {
      seenContrast.add(key);
      contrastIssues++;
      pushIssue('low-contrast', 'text "' + text.slice(0, 40) + '" has a contrast ratio of ' + ratio.toFixed(2) + ':1 (needs ' + contrastThreshold(el).toFixed(1) + ':1) and may be hard to read');
    }
  }

  if (needsHeaderFooter) {
    const heads = document.querySelectorAll('h1, h2, h3, h4, h5');
    const hn = Math.min(heads.length, 3000);
    for (let hi = 0; hi < hn; hi++) {
      const h = heads[hi];
      const style = getComputedStyle(h);
      const breaksPage = style.pageBreakBefore === 'always' || style.breakBefore === 'page';
      if (!breaksPage) continue;
      const marginTopIn = parseFloat(style.marginTop) / 96;
      if (marginTopIn < 0.3) {
        const label = (h.textContent || '').trim().slice(0, 40);
        pushIssue('heading-clip-risk', '<' + h.tagName.toLowerCase() + '> "' + label + '" forces a page break but has only ' + marginTopIn.toFixed(2) + 'in of top margin — content flush against a forced page break is clipped by roughly the first 0.3in when a header or page numbers are shown; give it more margin-top');
      }
    }
  }

  // break-inside protection for tables/code: without it, print slices
  // mid-row/mid-line. Sampled like overflow for huge documents.
  {
    const els = document.querySelectorAll('table, pre, code, .component-deep-dive, .component-warning, .component-axiom');
    const n = Math.min(els.length, 3000);
    for (let i = 0; i < n; i++) {
      const el = els[i];
      const cs = getComputedStyle(el);
      const avoid = cs.pageBreakInside === 'avoid' || cs.breakInside === 'avoid';
      if (!avoid && (el.scrollHeight > 300 || el.offsetHeight > 200)) {
        pushIssue('page-break-inside-risk', elLabel(el) + ' has no page-break-inside: avoid and may be split across pages mid-row');
      }
    }
  }

  // orphan/widow protection: a block with orphans/widows < 2 can strand a
  // single line at the bottom or top of a page. Sampled for huge docs.
  {
    const els = document.querySelectorAll('p, h1, h2, h3, h4, h5, li');
    const n = Math.min(els.length, 3000);
    for (let i = 0; i < n; i++) {
      const el = els[i];
      const cs = getComputedStyle(el);
      const orphans = parseInt(cs.orphans, 10);
      const widows = parseInt(cs.widows, 10);
      if ((orphans !== 0 && orphans < 2) || (widows !== 0 && widows < 2)) {
        pushIssue('line-break-risk', elLabel(el) + ' has orphans=' + orphans + ' widows=' + widows + ' — a line can be stranded alone at the top/bottom of a page; set orphans/widows to at least 2');
      }
    }
  }

  return JSON.stringify(issues);
})()`

type domIssue struct {
	Check    string `json:"check"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// runDOMAudit evaluates domAuditJS in the page currently loaded at ctx —
// meant to be called from inside a chromedp.ActionFunc that's already part
// of an in-progress chromedp.Tasks run, after Navigate and before
// PrintToPDF — and converts its findings into Issues. Any failure to run
// or parse the audit script itself is treated as non-fatal — the audit is
// advisory, and a bug in it must never break an otherwise-successful
// render.
func runDOMAudit(ctx context.Context, needsHeaderFooter bool) []Issue {
	var raw string
	script := fmt.Sprintf(domAuditJS, needsHeaderFooter)
	if err := chromedp.Evaluate(script, &raw).Do(ctx); err != nil {
		return nil
	}

	var found []domIssue
	if err := json.Unmarshal([]byte(raw), &found); err != nil {
		return nil
	}

	issues := make([]Issue, 0, len(found))
	for _, f := range found {
		sev := SeverityWarning
		if f.Severity == string(SeverityError) {
			sev = SeverityError
		}
		issues = append(issues, Issue{Check: f.Check, Severity: sev, Message: f.Message})
	}
	return issues
}

var pdfPageObjectRe = regexp.MustCompile(`/Type\s*/Page\b`)

// countPDFPages counts `/Type /Page` objects directly in the raw PDF bytes
// — a small, dependency-free heuristic (no PDF parsing library needed)
// that matches chrome-headless-shell's uncompressed object output, and
// deliberately excludes `/Type /Pages` (the page-tree node) via the word
// boundary after "Page".
func countPDFPages(pdfBuf []byte) int {
	return len(pdfPageObjectRe.FindAll(pdfBuf, -1))
}

// auditPDFBytes runs structural sanity checks on the finished PDF that can
// only be done once it exists — page objects present, EOF marker present,
// and a plausible file size — as a last-resort guard against a render that
// technically succeeded (no error) but produced a corrupt or empty file.
// Unlike the DOM checks (which are advisory), these report at
// SeverityError: a corrupt PDF is a real failure even if Chrome didn't
// return an error.
func auditPDFBytes(pdfBuf []byte) []Issue {
	var issues []Issue

	if len(pdfBuf) == 0 {
		return []Issue{{
			Check:    "pdf-empty",
			Severity: SeverityError,
			Message:  "the generated PDF is zero bytes — the output file is empty",
		}}
	}

	// chrome-headless-shell writes its PDFs uncompressed; the %%EOF marker
	// is always the final line of a well-formed file. Its absence means the
	// buffer was truncated somewhere — a strong corruption signal even if
	// some page objects happen to be present. Tolerate trailing whitespace
	// (the marker is followed by a newline), but nothing else.
	if !hasEOFMaker(pdfBuf) {
		issues = append(issues, Issue{
			Check:    "pdf-eof-missing",
			Severity: SeverityError,
			Message:  "the generated PDF is missing its %%EOF marker — the output file may be truncated or corrupt",
		})
	}

	if countPDFPages(pdfBuf) == 0 {
		issues = append(issues, Issue{
			Check:    "page-count",
			Severity: SeverityError,
			Message:  "could not find any page objects in the generated PDF — the output file may be empty or corrupt",
		})
	}

	return issues
}

// hasEOFMaker reports whether buf ends with the %%EOF marker allowing only
// trailing whitespace after it (Chrome appends a final newline).
func hasEOFMaker(buf []byte) bool {
	trimmed := bytes.TrimRight(buf, " \t\r\n")
	return bytes.HasSuffix(trimmed, []byte("%%EOF"))
}

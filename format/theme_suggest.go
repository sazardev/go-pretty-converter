package format

import (
	"regexp"
	"strings"

	"github.com/sazardev/go-pretty-converter/theme"
)

// Category values mirror the (unexported) constants theme/builtin.go
// assigns to its builtin themes' Category field — matched by value here
// since format deliberately doesn't need any other coupling to theme's
// internals.
const (
	categoryResume    = "resume"
	categoryTechnical = "technical"
	categoryAcademic  = "academic"
	categoryEditorial = "editorial"
)

var (
	resumeSignalRe = regexp.MustCompile(`(?i)^(skills|experience|education|summary|objective|certifications|projects)\s*:?$`)
	emailLikeRe    = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	citationRe     = regexp.MustCompile(`\[\d+\]`)
)

// contentSignals accumulates lightweight, purely statistical signals
// across every chapter of a Convert run, used only for the advisory theme
// suggestion in Report.SuggestedTheme — never for any structural decision.
type contentSignals struct {
	totalLines     int
	codeLines      int
	resumeHits     int
	sawEmailEarly  bool
	sawAbstract    bool
	citationHits   int
	paragraphCount int
	paragraphWords int
	headingCount   int
	linesSeen      int
}

// accumulate folds one chapter's signals in. title is scanned exactly like
// a body line: a short standalone line like "Skills" or "Abstract" is, by
// design (see heading.go), promoted to a Tier-1 chapter *title* rather
// than staying in Blocks — so the signal scan has to look at title too, or
// it would never see the single most common shape these patterns take.
func (s *contentSignals) accumulate(title string, blocks []block, headings, _, _, paragraphs int) {
	s.headingCount += headings
	s.paragraphCount += paragraphs

	s.observeLine(title)
	for _, b := range blocks {
		for _, line := range b.lines {
			s.observeLine(line)
		}
		switch {
		case isFencedBlock(b), isIndentedCodeBlock(b):
			s.codeLines += len(b.lines)
		case !isListBlock(b):
			s.paragraphWords += len(strings.Fields(b.text()))
		}
	}
}

func (s *contentSignals) observeLine(line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}
	s.totalLines++
	if resumeSignalRe.MatchString(trimmed) {
		s.resumeHits++
	}
	if s.linesSeen < 15 && emailLikeRe.MatchString(trimmed) {
		s.sawEmailEarly = true
	}
	if strings.EqualFold(trimmed, "abstract") {
		s.sawAbstract = true
	}
	s.citationHits += len(citationRe.FindAllString(trimmed, -1))
	s.linesSeen++
}

// suggestTheme maps accumulated signals to a builtin theme name + its
// Category, or ("", "") when no signal is confident enough — a wrong
// confident-looking suggestion is worse than none, so this only fires on a
// specific, deliberately narrow pattern, never a vague "looks default"
// guess. First match wins; order reflects specificity (resume/technical/
// academic signals are rarer and more decisive than the editorial-leaning
// prose-shape heuristic, which is checked last).
func (s *contentSignals) suggestTheme() (name, category string) {
	switch {
	case s.resumeHits >= 2 || (s.resumeHits >= 1 && s.sawEmailEarly):
		category = categoryResume
	case s.totalLines > 0 && float64(s.codeLines)/float64(s.totalLines) > 0.15:
		category = categoryTechnical
	case s.sawAbstract || s.citationHits >= 3:
		category = categoryAcademic
	case s.paragraphCount > 0 && s.headingCount > 0 &&
		float64(s.paragraphWords)/float64(s.paragraphCount) > 60 &&
		float64(s.headingCount)/float64(s.paragraphCount) < 0.15:
		category = categoryEditorial
	default:
		return "", ""
	}

	for _, t := range theme.List() {
		if t.Category == category {
			return t.Name, category
		}
	}
	return "", ""
}

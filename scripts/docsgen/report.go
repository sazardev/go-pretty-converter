package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// buildLogger is a small concurrency-safe reporter for the docsgen build.
// Every phase, asset, and PDF render reports through it; --verbose widens
// the output to every step, while the default mode prints only meaningful
// lines (phase timings, warnings, and the final summary).
type buildLogger struct {
	mu      sync.Mutex
	verbose bool
	start   time.Time
}

func newBuildLogger(verbose bool) *buildLogger {
	return &buildLogger{verbose: verbose, start: time.Now()}
}

func (l *buildLogger) Vf(format string, args ...any) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Printf(format+"\n", args...)
}

func (l *buildLogger) Infof(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Printf(format+"\n", args...)
}

func (l *buildLogger) Warnf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}

// phase records a single timed build phase. Phase runs are serialized
// through logPhase, so the summary can rely on monotonic ordering.
type phase struct {
	name    string
	elapsed time.Duration
}

// phaseRecorder accumulates phases and prints them as they finish.
type phaseRecorder struct {
	mu     sync.Mutex
	logger *buildLogger
	phases []phase
}

func newPhaseRecorder(l *buildLogger) *phaseRecorder {
	return &phaseRecorder{logger: l}
}

func (r *phaseRecorder) logPhase(name string, started time.Time) {
	el := time.Since(started)
	r.mu.Lock()
	r.phases = append(r.phases, phase{name: name, elapsed: el})
	r.mu.Unlock()
	r.logger.Vf("  ✓ %-28s %s", name, el.Round(time.Millisecond))
}

func (r *phaseRecorder) allPhases() []phase {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]phase, len(r.phases))
	copy(out, r.phases)
	return out
}

// renderResult is the outcome of one independently-produced artifact.
type renderResult struct {
	name    string
	elapsed time.Duration
	ok      bool
	err     error
	note    string
}

// resultsTable formats render results into aligned columns, one line each,
// grouping failures last.
func resultsTable(results []renderResult, verb string) string {
	if len(results) == 0 {
		return "    (none)"
	}
	var b strings.Builder
	type row struct {
		ok      bool
		name    string
		elapsed string
		note    string
	}
	var rows []row
	for _, r := range results {
		note := r.note
		if r.err != nil {
			note = r.err.Error()
		}
		rows = append(rows, row{ok: r.ok, name: r.name, elapsed: r.elapsed.Round(time.Millisecond).String(), note: note})
	}
	// failures last, then alphabetically
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].ok != rows[j].ok {
			return rows[i].ok
		}
		return rows[i].name < rows[j].name
	})
	width := 0
	for _, r := range rows {
		if len(r.name) > width {
			width = len(r.name)
		}
	}
	okCount := 0
	for _, r := range rows {
		if r.ok {
			okCount++
		}
	}
	fmt.Fprintf(&b, "    %s ok: %d/%d\n", verb, okCount, len(rows))
	for _, r := range rows {
		mark := "ok  "
		if !r.ok {
			mark = "FAIL"
		}
		note := r.note
		if note == "" {
			note = "—"
		}
		fmt.Fprintf(&b, "    [%s] %-*s %8s  %s\n", mark, width, r.name, r.elapsed, note)
	}
	return b.String()
}

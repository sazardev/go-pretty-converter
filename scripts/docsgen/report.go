package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
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

// ---------- timing statistics ----------

// stats summarizes a set of durations with the metrics worth printing:
// count, sum, mean, median, p95, min, max, and standard deviation. All are
// computed from the same slice so the numbers always agree.
type stats struct {
	Count  int           `json:"count"`
	Sum    time.Duration `json:"sum"`
	Mean   time.Duration `json:"mean"`
	Median time.Duration `json:"median"`
	P95    time.Duration `json:"p95"`
	P99    time.Duration `json:"p99"`
	Min    time.Duration `json:"min"`
	Max    time.Duration `json:"max"`
	StdDev time.Duration `json:"stddev"`
}

func computeStats(durs []time.Duration) stats {
	n := len(durs)
	if n == 0 {
		return stats{}
	}
	sorted := append([]time.Duration(nil), durs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var sum time.Duration
	for _, d := range durs {
		sum += d
	}
	mean := sum / time.Duration(n)

	var variance float64
	for _, d := range durs {
		diff := float64(d - mean)
		variance += diff * diff
	}
	stddev := time.Duration(math.Sqrt(variance / float64(n)))

	pct := func(p float64) time.Duration {
		if n == 1 {
			return sorted[0]
		}
		idx := int(math.Ceil(p*float64(n))) - 1
		if idx >= n {
			idx = n - 1
		}
		return sorted[idx]
	}

	return stats{
		Count:  n,
		Sum:    sum,
		Mean:   mean,
		Median: sorted[n/2],
		P95:    pct(0.95),
		P99:    pct(0.99),
		Min:    sorted[0],
		Max:    sorted[n-1],
		StdDev: stddev,
	}
}

// pctOf returns the percentage that d represents of total, for bar widths.
func pctOf(d, total time.Duration) float64 {
	if total <= 0 {
		return 0
	}
	return float64(d) / float64(total) * 100
}

// asciiBar renders a fixed-width horizontal bar for d as a fraction of
// total, using the block character set ▏▎▍▌▋▊▉█ so partial cells render
// proportionally.
func asciiBar(d, total time.Duration, width int) string {
	if total <= 0 {
		return strings.Repeat(" ", width)
	}
	full := float64(width) * float64(d) / float64(total)
	whole := int(full)
	frac := full - float64(whole)
	if whole > width {
		whole = width
		frac = 0
	}
	blocks := []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}
	var b strings.Builder
	b.WriteString(strings.Repeat("█", whole))
	if whole < width && frac > 0 {
		idx := int(math.Round(frac*float64(len(blocks)))) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		b.WriteString(blocks[idx])
		b.WriteString(strings.Repeat(" ", width-whole-1))
	} else {
		b.WriteString(strings.Repeat(" ", width-whole))
	}
	return b.String()
}

// ---------- phases ----------

// phase records a single timed build phase.
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
	r.logger.Vf("  ✓ %-32s %s", name, el.Round(time.Millisecond))
}

func (r *phaseRecorder) allPhases() []phase {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]phase, len(r.phases))
	copy(out, r.phases)
	return out
}

// phaseStats computes sum/mean/% over all recorded phases.
func (r *phaseRecorder) phaseStats() (sum, mean time.Duration) {
	ph := r.allPhases()
	if len(ph) == 0 {
		return 0, 0
	}
	var s time.Duration
	for _, p := range ph {
		s += p.elapsed
	}
	return s, s / time.Duration(len(ph))
}

// ---------- render results ----------

// Artifact groups used to bucket render results in tables and report.json.
const (
	groupRaster = "raster"
	groupPDF    = "pdf"
	groupEPUB   = "epub"
)

// renderResult is the outcome of one independently-produced artifact.
type renderResult struct {
	name    string
	elapsed time.Duration
	ok      bool
	err     error
	note    string
	bytes   int64
	group   string
}

// renderResultJSON is the machine-readable form of renderResult for
// report.json.
type renderResultJSON struct {
	Name      string `json:"name"`
	Group     string `json:"group"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	Elapsed   string `json:"elapsed"`
	ElapsedMS int64  `json:"elapsed_ms"`
	Bytes     int64  `json:"bytes,omitempty"`
	Size      string `json:"size,omitempty"`
}

// renderResultsTable builds a wide, padded table with every metric column.
func renderResultsTable(results []renderResult, title string) string {
	if len(results) == 0 {
		return fmt.Sprintf("\n  ── %s ──\n    (none)\n", title)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "\n  %s\n", title)

	// rows
	type row struct {
		ok      bool
		name    string
		elapsed time.Duration
		note    string
		bytes   int64
	}
	var rows []row
	for _, r := range results {
		note := r.note
		if r.err != nil {
			note = r.err.Error()
		}
		rows = append(rows, row{ok: r.ok, name: r.name, elapsed: r.elapsed, note: note, bytes: r.bytes})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].ok != rows[j].ok {
			return rows[i].ok
		}
		return rows[i].name < rows[j].name
	})

	nameW := 0
	for _, r := range rows {
		if len(r.name) > nameW {
			nameW = len(r.name)
		}
	}

	okCount := 0
	var totalBytes int64
	var durTotal time.Duration
	for _, r := range rows {
		if r.ok {
			okCount++
			totalBytes += r.bytes
			durTotal += r.elapsed
		}
	}

	st := stats{}
	var okDurs []time.Duration
	for _, r := range rows {
		if r.ok {
			okDurs = append(okDurs, r.elapsed)
		}
	}
	st = computeStats(okDurs)

	// header line
	fmt.Fprintf(&b, "    status  %-*s  %10s  %10s  %9s  %s\n", nameW, "artifact", "elapsed", "mean", "size", "detail")
	b.WriteString(strings.Repeat("    ", 1) + strings.Repeat("─", 6+nameW+10+10+9+8) + "\n")

	for _, r := range rows {
		mark := "ok "
		if !r.ok {
			mark = "FAIL"
		}
		mean := st.Mean
		meanStr := ""
		if r.ok {
			meanStr = mean.Round(time.Millisecond).String()
		}
		sizeStr := ""
		if r.bytes > 0 {
			sizeStr = formatBytes(int(r.bytes))
		}
		note := r.note
		if note == "" {
			note = "—"
		}
		fmt.Fprintf(&b, "    [%s] %-*s  %10s  %10s  %9s  %s\n",
			mark, nameW, r.name, r.elapsed.Round(time.Millisecond), meanStr, sizeStr, note)
	}

	// footer stats
	fmt.Fprintf(&b, "    %s %d/%d ok · sum %s · mean %s · p95 %s · min %s · max %s · σ %s\n",
		strings.Repeat("─", nameW+2), okCount, len(rows),
		st.Sum.Round(time.Millisecond), st.Mean.Round(time.Millisecond),
		st.P95.Round(time.Millisecond), st.Min.Round(time.Millisecond),
		st.Max.Round(time.Millisecond), st.StdDev.Round(time.Millisecond))
	if totalBytes > 0 {
		fmt.Fprintf(&b, "    %s total size %s across %d artifact(s)\n",
			strings.Repeat("─", nameW+2), formatBytes(int(totalBytes)), okCount)
	}
	return b.String()
}

// ---------- phases table with bars ----------

// phasesTable renders phases sorted by elapsed (largest first) with a
// proportional ASCII bar and cumulative percentage of wall time.
func phasesTable(phases []phase, total time.Duration) string {
	if len(phases) == 0 {
		return "    (no phases)"
	}
	sorted := append([]phase(nil), phases...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].elapsed > sorted[j].elapsed })

	nameW := 0
	for _, p := range sorted {
		if len(p.name) > nameW {
			nameW = len(p.name)
		}
	}
	barW := 24

	var b strings.Builder
	b.WriteString("\n  Phase timeline (largest → smallest)\n")
	fmt.Fprintf(&b, "    %-*s  %10s  %6s  %-*s  %s\n", nameW, "phase", "elapsed", "share", barW, "distribution", "cumulative")
	b.WriteString(strings.Repeat("    ", 1) + strings.Repeat("─", nameW+10+6+barW+16) + "\n")

	var cum time.Duration
	for _, p := range sorted {
		cum += p.elapsed
		cumPct := pctOf(cum, total)
		fmt.Fprintf(&b, "    %-*s  %10s  %5.1f%%  %s  %5.1f%%\n",
			nameW, p.name, p.elapsed.Round(time.Millisecond), pctOf(p.elapsed, total),
			asciiBar(p.elapsed, total, barW), cumPct)
	}
	fmt.Fprintf(&b, "    %s %s  %5.1f%%  %s\n",
		strings.Repeat("─", nameW), total.Round(time.Millisecond), 100.0, asciiBar(total, total, barW))
	return b.String()
}

// ---------- overall report (report.json) ----------

// buildReportJSON is the machine-readable shape of the full build report,
// written to _site/report.json so tooling can consume every metric.
type buildReportJSON struct {
	GeneratedAt  time.Time          `json:"generated_at"`
	GeneratedAtT string             `json:"generated_at_rfc3339"`
	Duration     string             `json:"duration"`
	DurationMS   int64              `json:"duration_ms"`
	Jobs         int                `json:"jobs"`
	Verbose      bool               `json:"verbose"`
	Summary      buildSummaryJSON   `json:"summary"`
	Phases       []phaseJSON        `json:"phases"`
	Artifacts    []renderResultJSON `json:"artifacts"`
	Metrics      metricsJSON        `json:"metrics"`
}

type buildSummaryJSON struct {
	TotalArtifacts  int     `json:"total_artifacts"`
	OKArtifacts     int     `json:"ok_artifacts"`
	FailedArtifacts int     `json:"failed_artifacts"`
	TotalBytes      int64   `json:"total_bytes"`
	TotalSize       string  `json:"total_size"`
	ArtifactsPerSec float64 `json:"artifacts_per_second"`
	MiBPerSec       float64 `json:"mib_per_second"`
	WallTime        string  `json:"wall_time"`
}

type phaseJSON struct {
	Name      string  `json:"name"`
	Elapsed   string  `json:"elapsed"`
	ElapsedMS int64   `json:"elapsed_ms"`
	Share     float64 `json:"share_percent"`
}

type metricsJSON struct {
	ArtifactMean   string `json:"artifact_mean"`
	ArtifactP95    string `json:"artifact_p95"`
	ArtifactMedian string `json:"artifact_median"`
	ArtifactStdDev string `json:"artifact_stddev"`
	Fastest        string `json:"fastest_artifact"`
	Slowest        string `json:"slowest_artifact"`
}

// writeReportJSON persists the full build report to outDir/report.json.
func writeReportJSON(outDir string, total time.Duration, jobs int, verbose bool, phases []phase, results []renderResult, log *buildLogger) {
	now := time.Now()
	sum := buildSummaryJSON{}
	artifacts := make([]renderResultJSON, 0, len(results))

	var okDurs []time.Duration
	for _, r := range results {
		jr := renderResultJSON{
			Name:      r.name,
			Group:     r.group,
			OK:        r.ok,
			Elapsed:   r.elapsed.Round(time.Millisecond).String(),
			ElapsedMS: r.elapsed.Milliseconds(),
			Bytes:     r.bytes,
			Size:      "",
		}
		if r.err != nil {
			jr.Error = r.err.Error()
		} else {
			sum.TotalBytes += r.bytes
		}
		if r.bytes > 0 {
			jr.Size = formatBytes(int(r.bytes))
		}
		artifacts = append(artifacts, jr)

		sum.TotalArtifacts++
		if r.ok {
			sum.OKArtifacts++
			okDurs = append(okDurs, r.elapsed)
		} else {
			sum.FailedArtifacts++
		}
	}
	sum.TotalSize = formatBytes(int(sum.TotalBytes))
	secs := total.Seconds()
	if secs > 0 {
		sum.ArtifactsPerSec = float64(sum.OKArtifacts) / secs
		sum.MiBPerSec = float64(sum.TotalBytes) / (1 << 20) / secs
	}
	sum.WallTime = total.Round(time.Millisecond).String()

	st := computeStats(okDurs)
	fastest, slowest := "", ""
	var fastestDur, slowestDur time.Duration
	first := true
	for _, r := range results {
		if !r.ok {
			continue
		}
		if first {
			fastest, slowest = r.name, r.name
			fastestDur, slowestDur = r.elapsed, r.elapsed
			first = false
			continue
		}
		if r.elapsed < fastestDur {
			fastest, fastestDur = r.name, r.elapsed
		}
		if r.elapsed > slowestDur {
			slowest, slowestDur = r.name, r.elapsed
		}
	}

	phasesOut := make([]phaseJSON, 0, len(phases))
	for _, p := range phases {
		phasesOut = append(phasesOut, phaseJSON{
			Name:      p.name,
			Elapsed:   p.elapsed.Round(time.Millisecond).String(),
			ElapsedMS: p.elapsed.Milliseconds(),
			Share:     pctOf(p.elapsed, total),
		})
	}

	report := buildReportJSON{
		GeneratedAt:  now,
		GeneratedAtT: now.UTC().Format(time.RFC3339),
		Duration:     total.Round(time.Millisecond).String(),
		DurationMS:   total.Milliseconds(),
		Jobs:         jobs,
		Verbose:      verbose,
		Summary:      sum,
		Phases:       phasesOut,
		Artifacts:    artifacts,
		Metrics: metricsJSON{
			ArtifactMean:   st.Mean.Round(time.Millisecond).String(),
			ArtifactP95:    st.P95.Round(time.Millisecond).String(),
			ArtifactMedian: st.Median.Round(time.Millisecond).String(),
			ArtifactStdDev: st.StdDev.Round(time.Millisecond).String(),
			Fastest:        fastest,
			Slowest:        slowest,
		},
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Warnf("could not marshal report.json: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join(outDir, "report.json"), data, 0644); err != nil {
		log.Warnf("could not write report.json: %v", err)
		return
	}
	log.Vf("  ✓ %-24s %s", "report.json", formatBytes(len(data)))
}

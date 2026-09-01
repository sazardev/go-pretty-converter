// Command benchmark runs an automated performance verification of
// go-pretty-converter across multiple book sizes. It builds the real CLI binary,
// renders real books through it, measures wall time / pages / throughput /
// live CPU-RAM of the whole process tree (including Chrome), shows an
// animated readout per export, and writes benchmark-report.json.
//
// Usage:
//
//	go run ./scripts/benchmark                 # default sizes up to 3000 docs
//	go run ./scripts/benchmark --max 2000
//	go run ./scripts/benchmark --sizes 100,1000,2000 --out /tmp/bench
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- ANSI ----------

const (
	reset     = "\x1b[0m"
	bold      = "\x1b[1m"
	dim       = "\x1b[2m"
	red       = "\x1b[31m"
	green     = "\x1b[32m"
	yellow    = "\x1b[33m"
	cyan      = "\x1b[36m"
	magenta   = "\x1b[35m"
	clearLine = "\x1b[2K\r"
)

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func ansi(code string, enabled bool) string {
	if !enabled {
		return ""
	}
	return code
}

func isTerminal() bool {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// ---------- process-tree sampling (Linux /proc) ----------

type sample struct {
	cpu float64
	rss int64 // KiB
}

// sampler polls the process tree rooted at a command's PID for cumulative
// CPU ticks and RSS every 80ms, computing a per-sample CPU% from deltas.
type sampler struct {
	mu        sync.Mutex
	samples   []sample
	prevTicks int64
	prevTime  time.Time
	rootPID   int
	stop      chan struct{}
	wg        sync.WaitGroup
}

func newSampler(rootPID int) *sampler {
	s := &sampler{rootPID: rootPID, stop: make(chan struct{})}
	s.prevTicks, _ = treeTicks(rootPID)
	s.prevTime = time.Now()
	return s
}

// cpuFromDelta converts a CPU-tick delta over a wall window into a
// percentage across all cores (Linux HZ = 100).
func cpuFromDelta(dTicks int64, since time.Duration) float64 {
	if since <= 0 {
		return 0
	}
	return float64(dTicks) / 100.0 / since.Seconds() * 100.0
}

// finalSample stops the sampler, then reports total CPU% and peak RSS for
// the whole run. It prefers the samples already captured (the process may
// be gone by the time it's called, making a post-exit /proc read return 0).
func (s *sampler) finalSample() (peakRSS int64, avgCPU float64, n int) {
	close(s.stop)
	s.wg.Wait()
	s.mu.Lock()
	all := append([]sample{}, s.samples...)
	s.mu.Unlock()
	if len(all) > 0 {
		var rssMax int64
		var cpuSum float64
		for _, sm := range all {
			if sm.rss > rssMax {
				rssMax = sm.rss
			}
			cpuSum += sm.cpu
		}
		return rssMax, cpuSum / float64(len(all)), len(all)
	}
	// No live samples (process finished <80ms); best-effort post-exit read.
	finalTicks, finalRSS := treeTicks(s.rootPID)
	total := cpuFromDelta(finalTicks-s.prevTicks, time.Since(s.prevTime))
	return finalRSS, total, 1
}

func (s *sampler) run() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				return
			case now := <-ticker.C:
				ticks, rss := treeTicks(s.rootPID)
				dTicks := ticks - s.prevTicks
				dSec := now.Sub(s.prevTime).Seconds()
				s.prevTicks, s.prevTime = ticks, now
				cpu := 0.0
				if dSec > 0 {
					// HZ on Linux is 100; CPU% = ticks_delta / HZ / seconds * 100
					cpu = float64(dTicks) / 100.0 / dSec * 100.0
				}
				s.mu.Lock()
				s.samples = append(s.samples, sample{cpu: cpu, rss: rss})
				s.mu.Unlock()
			}
		}
	}()
}

// treeTicks sums cumulative CPU ticks (utime+stime) and RSS across a
// process and all descendants (so the Chrome children the CLI spawns are
// counted). Non-Linux returns zeroes.
func treeTicks(pid int) (ticks int64, rss int64) {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err == nil {
		body := string(stat)
		if rp := strings.LastIndex(body, ")"); rp >= 0 {
			fields := strings.Fields(body[rp+1:])
			if len(fields) >= 13 {
				u, _ := strconv.ParseInt(fields[11], 10, 64)
				st, _ := strconv.ParseInt(fields[12], 10, 64)
				ticks += u + st
			}
		}
	}
	status, _ := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, _ := strconv.ParseInt(fields[1], 10, 64)
				rss += v
			}
		}
	}
	// children via /proc/<pid>/task/*/children
	seen := map[int]bool{pid: true}
	tasks, _ := os.ReadDir(fmt.Sprintf("/proc/%d/task", pid))
	for _, t := range tasks {
		dir := fmt.Sprintf("/proc/%d/task/%s/children", pid, t.Name())
		kids, _ := os.ReadDir(dir)
		for _, k := range kids {
			cpid, err := strconv.Atoi(k.Name())
			if err != nil || seen[cpid] {
				continue
			}
			seen[cpid] = true
			ct, cr := treeTicks(cpid)
			ticks += ct
			rss += cr
		}
	}
	return ticks, rss
}

// ---------- book generation ----------

func writeDoc(dir string, i int) error {
	ch := i/100 + 1
	sub := i % 100
	content := fmt.Sprintf("---\nid: \"[%d.%d.0]\"\ntitle: \"Doc %d\"\n---\n\n# Doc %d\n\nShort body for document %d with enough words to fill a page line.\n",
		ch, sub, i, i, i)
	return os.WriteFile(filepath.Join(dir, fmt.Sprintf("%05d.mdx", i)), []byte(content), 0644)
}

func mkBook(dir string, n int) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for i := 0; i < n; i++ {
		if err := writeDoc(dir, i); err != nil {
			return err
		}
	}
	return nil
}

func countBytes(dir string) int64 {
	var total int64
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if info, err := e.Info(); err == nil {
			total += info.Size()
		}
	}
	return total
}

func formatBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func elapsedHuman(d time.Duration) string {
	return d.Round(time.Millisecond).String()
}

// ---------- export runner ----------

type exportResult struct {
	Docs        int     `json:"docs"`
	Export      string  `json:"export"`
	WallMS      int64   `json:"wall_ms"`
	Pages       int     `json:"pages"`
	SizeBytes   int64   `json:"size_bytes"`
	PagesPerSec float64 `json:"pages_per_second"`
	MiBPerSec   float64 `json:"mib_per_second"`
	PeakRSSMiB  float64 `json:"peak_rss_mib"`
	AvgCPU      float64 `json:"avg_cpu_percent"`
	Samples     int     `json:"samples"`
	OK          bool    `json:"ok"`
	Error       string  `json:"error,omitempty"`
}

func (r exportResult) Wall() time.Duration { return time.Duration(r.WallMS) * time.Millisecond }

// runExport executes the CLI binary for one book/export, sampling the
// process tree live and reporting aggregate metrics. tick is called ~90ms
// during the run with the latest CPU/RSS for the animation. kind is a short
// label (check/epub/pdf) stored in the result.
func runExport(bin, srcDir, outPath, args string, n int, kind string, tick func(elapsed time.Duration, rssMiB, cpu float64)) exportResult {
	cmd := exec.Command(bin, strings.Fields(args)...)
	// Silence CLI progress output; the animation on stderr is the display.
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return exportResult{Docs: n, Export: kind, OK: false, Error: err.Error()}
	}

	s := newSampler(cmd.Process.Pid)
	s.run()
	start := time.Now()

	done := make(chan struct{})
	var animWG sync.WaitGroup
	animWG.Add(1)
	go func() {
		defer animWG.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			s.mu.Lock()
			var cpu float64
			var rss int64
			if len(s.samples) > 0 {
				last := s.samples[len(s.samples)-1]
				cpu, rss = last.cpu, last.rss
			}
			s.mu.Unlock()
			if tick != nil {
				tick(time.Since(start), float64(rss)/1024, cpu)
			}
			time.Sleep(90 * time.Millisecond)
		}
	}()

	err := cmd.Wait()
	elapsed := time.Since(start)
	close(done)
	animWG.Wait()
	// Short-lived processes (check/epub) may finish before the first 80ms
	// tick; finalSample stops the sampler and folds a post-exit read in so
	// their RSS/CPU aren't reported as zero.
	peakRSS, avgCPU, samples := s.finalSample()

	res := exportResult{
		Docs:       n,
		Export:     kind,
		WallMS:     elapsed.Milliseconds(),
		PeakRSSMiB: float64(peakRSS) / 1024,
		AvgCPU:     avgCPU,
		Samples:    samples,
		OK:         err == nil,
	}
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if info, serr := os.Stat(outPath); serr == nil {
		res.SizeBytes = info.Size()
	}
	res.Pages = countPDFPages(outPath)
	if elapsed.Seconds() > 0 {
		res.PagesPerSec = float64(res.Pages) / elapsed.Seconds()
		res.MiBPerSec = float64(res.SizeBytes) / (1 << 20) / elapsed.Seconds()
	}
	return res
}

func countPDFPages(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	needle := []byte("/Type /Page")
	count := 0
	from := 0
	for {
		idx := strings.Index(string(data[from:]), string(needle))
		if idx < 0 {
			break
		}
		count++
		from += idx + len(needle)
	}
	return count
}

// ---------- display ----------

func progressBar(pct float64, width int, color bool) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	full := int(pct * float64(width))
	bar := strings.Repeat("█", full) + strings.Repeat("░", width-full)
	if color {
		bar = green + bar + reset
	}
	return fmt.Sprintf("%s %3.0f%%", bar, pct*100)
}

func banner(color bool) {
	fmt.Fprintf(os.Stderr, "\n  %s\n", ansi(bold+cyan, color)+"go-pretty-converter · automated performance verification"+ansi(reset, color))
	fmt.Fprintf(os.Stderr, "  %s\n", ansi(dim, color)+"renders real books across sizes, samples CPU/RAM live, reports everything"+ansi(reset, color))
	fmt.Fprintln(os.Stderr)
}

func printResult(res exportResult, color bool) {
	mark := ansi(green, color) + "ok  " + ansi(reset, color)
	if !res.OK {
		mark = ansi(red, color) + "FAIL" + ansi(reset, color)
	}
	if !res.OK {
		fmt.Fprintf(os.Stderr, "  %s %-8s %-12s %s\n", mark, res.Export, fmt.Sprintf("%d docs", res.Docs), res.Error)
		return
	}
	detail := fmt.Sprintf("%s · %s", formatBytes(int(res.SizeBytes)), elapsedHuman(res.Wall()))
	if res.Pages > 0 {
		detail = fmt.Sprintf("%d pages · %s · %.0f pág/s · %s", res.Pages, formatBytes(int(res.SizeBytes)), res.PagesPerSec, elapsedHuman(res.Wall()))
	}
	fmt.Fprintf(os.Stderr, "  %s %-8s %-12s %s\n", mark, res.Export, fmt.Sprintf("%d docs", res.Docs), detail)
}

func printSummary(results []exportResult, color bool) {
	fmt.Fprintf(os.Stderr, "\n  %s\n", ansi(bold+magenta, color)+"════════ SUMMARY ════════"+ansi(reset, color))
	fmt.Fprintf(os.Stderr, "  %-8s %-8s %-12s %-9s %-10s %-10s %-10s\n", "export", "docs", "wall", "pages", "pág/s", "MiB/s", "peak RSS")
	fmt.Fprintf(os.Stderr, "  %s\n", strings.Repeat("─", 76))
	for _, r := range results {
		if !r.OK {
			fmt.Fprintf(os.Stderr, "  %-8s %-8d %-12s %s\n", r.Export, r.Docs, elapsedHuman(r.Wall()), r.Error)
			continue
		}
		pages := "—"
		pps := "—"
		mps := "—"
		if r.Pages > 0 {
			pages = strconv.Itoa(r.Pages)
			pps = fmt.Sprintf("%.0f", r.PagesPerSec)
			mps = fmt.Sprintf("%.2f", r.MiBPerSec)
		}
		fmt.Fprintf(os.Stderr, "  %-8s %-8d %-12s %-9s %-10s %-10s %-10.1f\n",
			r.Export, r.Docs, elapsedHuman(r.Wall()), pages, pps, mps, r.PeakRSSMiB)
	}
	fmt.Fprintf(os.Stderr, "  %s\n", strings.Repeat("─", 76))

	var pdfs []exportResult
	for _, r := range results {
		if r.OK && r.Export == "pdf" {
			pdfs = append(pdfs, r)
		}
	}
	if len(pdfs) >= 2 {
		big := pdfs[len(pdfs)-1]
		small := pdfs[0]
		if big.Pages > 0 && small.WallMS > 0 {
			ratio := float64(big.Pages) / big.Wall().Seconds()
			fmt.Fprintf(os.Stderr, "\n  %slargest PDF: %d pages in %s → %.0f pages/s (%.2f MiB/s, peak RSS %.0f MiB, avg CPU %.0f%%)%s\n",
				ansi(yellow, color), big.Pages, elapsedHuman(big.Wall()), ratio, big.MiBPerSec, big.PeakRSSMiB, big.AvgCPU, ansi(reset, color))
		}
	}
}

func writeReport(workDir string, results []exportResult) {
	data, err := json.MarshalIndent(map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"results":      results,
	}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(workDir, "benchmark-report.json"), data, 0644)
}

func parseSizes(s string, max int) []int {
	var sizes []int
	for _, p := range strings.Split(s, ",") {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err == nil && v > 0 && v <= max {
			sizes = append(sizes, v)
		}
	}
	sort.Ints(sizes)
	if len(sizes) == 0 {
		for _, v := range []int{100, 500, 1000, 2000, max} {
			if v <= max && v > 0 {
				sizes = append(sizes, v)
			}
		}
	}
	return sizes
}

func main() {
	color := flag.Bool("color", isTerminal(), "ANSI color output")
	sizesFlag := flag.String("sizes", "", "comma-separated doc counts (default: 100,500,1000,2000,max)")
	maxFlag := flag.Int("max", 3000, "maximum book size in docs")
	outFlag := flag.String("out", "", "output directory for artifacts and report (default: temp)")
	flag.Parse()

	sizes := parseSizes(*sizesFlag, *maxFlag)

	workDir := *outFlag
	if workDir == "" {
		workDir, _ = os.MkdirTemp("", "pretty-converter-bench-*")
	}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	banner(*color)

	bin := filepath.Join(workDir, "pretty-converter-bin")
	fmt.Fprintf(os.Stderr, "  %s building CLI binary...\n", ansi(dim, *color))
	if err := buildCLI(bin); err != nil {
		fmt.Fprintf(os.Stderr, "  %serror building CLI: %v%s\n", ansi(red, *color), err, ansi(reset, *color))
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "  %s✓ CLI built%s\n", ansi(green, *color), ansi(reset, *color))

	var all []exportResult
	for _, n := range sizes {
		srcDir := filepath.Join(workDir, fmt.Sprintf("book-%d", n))
		if err := mkBook(srcDir, n); err != nil {
			fmt.Fprintf(os.Stderr, "%serror: %v%s\n", ansi(red, *color), err, ansi(reset, *color))
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "\n  %s── book of %d docs (%s source) ──%s\n",
			ansi(bold, *color), n, formatBytes(int(countBytes(srcDir))), ansi(reset, *color))

		exports := []struct{ kind, args, opath string }{
			{"check", fmt.Sprintf("check --source %s", srcDir), ""},
			{"epub", fmt.Sprintf("epub --source %s --out %s/book-%d.epub", srcDir, workDir, n), filepath.Join(workDir, fmt.Sprintf("book-%d.epub", n))},
			{"pdf", fmt.Sprintf("build --source %s --out %s/book-%d.pdf", srcDir, workDir, n), filepath.Join(workDir, fmt.Sprintf("book-%d.pdf", n))},
			// --fast is the shorthand for --no-header --no-page-numbers
			// --no-outline --no-tagged-pdf — run alongside the default "pdf"
			// export so every benchmark run quantifies the tradeoff instead
			// of leaving it to a one-off measurement.
			{"pdf-fast", fmt.Sprintf("build --source %s --out %s/book-%d-fast.pdf --fast", srcDir, workDir, n), filepath.Join(workDir, fmt.Sprintf("book-%d-fast.pdf", n))},
		}
		for _, ex := range exports {
			label := fmt.Sprintf("export %s · %d docs", ex.kind, n)
			done := make(chan struct{})
			var mu sync.Mutex
			var live exportResult
			var animWG sync.WaitGroup
			sp := 0
			animWG.Add(1)
			go func() {
				defer animWG.Done()
				for {
					select {
					case <-done:
						return
					default:
					}
					mu.Lock()
					el := time.Duration(live.WallMS) * time.Millisecond
					rss := live.PeakRSSMiB
					cpu := live.AvgCPU
					mu.Unlock()
					fmt.Fprintf(os.Stderr, "\r  %s %s %-30s %-26s cpu %5.0f%% rss %6.1f MiB  %s",
						ansi(cyan, *color), spinner[sp%len(spinner)], label, progressBar(el.Seconds()/30, 20, *color), cpu, rss, elapsedHuman(el))
					sp++
					time.Sleep(90 * time.Millisecond)
				}
			}()

			// runExport stores the kind (not the full args) so summaries
			// and reports stay readable.
			res := runExport(bin, srcDir, ex.opath, ex.args, n, ex.kind, func(elapsed time.Duration, rssMiB, cpu float64) {
				mu.Lock()
				live = exportResult{WallMS: elapsed.Milliseconds(), PeakRSSMiB: rssMiB, AvgCPU: cpu}
				mu.Unlock()
			})

			close(done)
			animWG.Wait()
			fmt.Fprintf(os.Stderr, "\r%s", clearLine)
			all = append(all, res)
			printResult(res, *color)
		}
	}

	printSummary(all, *color)
	writeReport(workDir, all)
	fmt.Fprintf(os.Stderr, "\n  %s report: %s\n", ansi(dim, *color), filepath.Join(workDir, "benchmark-report.json"))
}

func buildCLI(dest string) error {
	cmd := exec.Command("go", "build", "-o", dest, "github.com/sazardev/go-pretty-converter/cmd/pretty-converter")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

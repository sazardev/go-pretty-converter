package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepDone
	StepError
	StepSkipped
)

type PipelineStep struct {
	Name     string
	Status   StepStatus
	Error    string
	Duration time.Duration
	start    time.Time
	spinner  *Spinner
}

type PipelineProgress struct {
	steps []*PipelineStep
}

func NewPipelineProgress(names ...string) *PipelineProgress {
	pp := &PipelineProgress{}
	for _, name := range names {
		pp.steps = append(pp.steps, &PipelineStep{
			Name:   name,
			Status: StepPending,
		})
	}
	return pp
}

func (pp *PipelineProgress) Start(stepName string) {
	for _, s := range pp.steps {
		if s.Name == stepName {
			s.Status = StepRunning
			s.start = time.Now()
			s.spinner = StartSpinner(s.Name)
			return
		}
	}
}

func (pp *PipelineProgress) Done(stepName string) {
	for _, s := range pp.steps {
		if s.Name == stepName && s.Status == StepRunning {
			s.spinner.Ok()
			s.Status = StepDone
			s.Duration = time.Since(s.start)
			return
		}
	}
}

func (pp *PipelineProgress) Fail(stepName string, err string) {
	for _, s := range pp.steps {
		if s.Name == stepName && s.Status == StepRunning {
			s.spinner.Fail(err)
			s.Status = StepError
			s.Duration = time.Since(s.start)
			s.Error = err
			return
		}
	}
}

func (pp *PipelineProgress) Skip(stepName string, reason string) {
	for _, s := range pp.steps {
		if s.Name == stepName && s.Status == StepPending {
			fmt.Printf("  %s %s — %s\n", WarningSymbol, s.Name, WarningStyle.Render(reason))
			s.Status = StepSkipped
			return
		}
	}
}

type WatchStats struct {
	Builds  int
	Errors  int
	Running bool
	last    time.Time
}

func (w *WatchStats) RecordBuild() {
	w.Builds++
	w.last = time.Now()
}

func (w *WatchStats) RecordError() {
	w.Errors++
	w.last = time.Now()
}

func PrintWatchBanner() {
	fmt.Println()
	fmt.Println(PrimaryStyle.Render("  ⚡ Watching for changes..."))
	fmt.Println("  " + MutedStyle.Render("Press Ctrl+C to stop"))
	fmt.Println()
}

func PrintWatchRebuild() {
	divider := DividerStyle.Render(strings.Repeat("─", 60))
	fmt.Println(divider)
}

func PrintWatchSummary(stats WatchStats) {
	parts := []string{
		KeyValue("Builds", NumberStyle.Render(fmt.Sprintf("%d", stats.Builds))),
	}

	if stats.Errors > 0 {
		parts = append(parts, KeyValue("Errors", ErrorStyle.Render(fmt.Sprintf("%d", stats.Errors))))
	}

	if !stats.last.IsZero() {
		parts = append(parts, KeyValue("Last build", stats.last.Format("15:04:05")))
	}

	fmt.Println(Panel("Watch Mode", lipgloss.JoinVertical(lipgloss.Left, parts...)))
}

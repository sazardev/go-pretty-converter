package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/sazardev/go-pretty-converter/analyze"
	"github.com/sazardev/go-pretty-converter/format"
	"github.com/sazardev/go-pretty-converter/mdx"
)

type BuildStats struct {
	Documents int
	Output    string
	FileSize  string
	Duration  time.Duration
	Theme     string
	Warnings  int
}

func PrintBuildSummary(stats BuildStats) {
	var lines []string
	lines = append(lines, KeyValue("Documents", NumberStyle.Render(fmt.Sprintf("%d", stats.Documents))))
	if stats.FileSize != "" {
		lines = append(lines, KeyValue("Output", fmt.Sprintf("%s (%s)", stats.Output, stats.FileSize)))
	} else {
		lines = append(lines, KeyValue("Output", stats.Output))
	}
	lines = append(lines, KeyValue("Duration", stats.Duration.Round(time.Millisecond).String()))
	if stats.Theme != "" {
		lines = append(lines, KeyValue("Theme", stats.Theme))
	}
	if stats.Warnings > 0 {
		lines = append(lines, KeyValue("Warnings", WarningStyle.Render(fmt.Sprintf("%d", stats.Warnings))))
	} else {
		lines = append(lines, KeyValue("Warnings", "0"))
	}

	fmt.Println()
	fmt.Println(Panel("Build Complete!", strings.Join(lines, "\n")))
}

func PrintValidationSummary(errs []mdx.ValidationError, warnings int, docFiles []string) {
	failedFiles := make(map[string]bool)
	warnFiles := make(map[string]bool)
	for _, e := range errs {
		if e.Field == "content" {
			warnFiles[e.File] = true
		} else {
			failedFiles[e.File] = true
		}
	}

	passed := 0
	errored := 0
	warned := 0

	fmt.Println()
	for _, f := range docFiles {
		if failedFiles[f] {
			fmt.Printf("  %s %s\n", ErrorSymbol, FilePathStyle.Render(f))
			errored++
		} else if warnFiles[f] {
			for _, e := range errs {
				if e.File == f && e.Field == "content" {
					fmt.Printf("  %s %s — %s\n", WarningSymbol, FilePathStyle.Render(f), WarningStyle.Render(e.Message))
				}
			}
			warned++
		} else {
			fmt.Printf("  %s %s\n", SuccessSymbol, FilePathStyle.Render(f))
			passed++
		}
	}

	total := len(docFiles)
	fmt.Println()
	fmt.Println(Panel("Check Results",
		KeyValue("Files", NumberStyle.Render(fmt.Sprintf("%d", total)))+"\n"+
			KeyValue("Passed", SuccessStyle.Render(fmt.Sprintf("%d", passed)))+"\n"+
			KeyValue("Warnings", WarningStyle.Render(fmt.Sprintf("%d", warned)))+"\n"+
			KeyValue("Errors", ErrorStyle.Render(fmt.Sprintf("%d", errored))),
	))
}

// PrintAnalysisSummary prints analyze.Issue findings grouped by document
// (in docFiles' order — a clean file with no findings still gets a ✓ line),
// each issue prefixed by its severity symbol and "[check-name]", followed
// by a Panel totaling files/errors/warnings/improvements.
func PrintAnalysisSummary(issues []analyze.Issue, docFiles []string) {
	byFile := make(map[string][]analyze.Issue, len(docFiles))
	for _, iss := range issues {
		byFile[iss.File] = append(byFile[iss.File], iss)
	}

	errors, warnings, infos := 0, 0, 0

	fmt.Println()
	for _, f := range docFiles {
		fileIssues := byFile[f]
		if len(fileIssues) == 0 {
			fmt.Printf("  %s %s\n", SuccessSymbol, FilePathStyle.Render(f))
			continue
		}
		fmt.Printf("  %s\n", FilePathStyle.Render(f))
		for _, iss := range fileIssues {
			switch iss.Severity {
			case analyze.SeverityError:
				errors++
				fmt.Printf("    %s [%s] %s\n", ErrorSymbol, iss.Check, ErrorStyle.Render(iss.Message))
			case analyze.SeverityWarning:
				warnings++
				fmt.Printf("    %s [%s] %s\n", WarningSymbol, iss.Check, WarningStyle.Render(iss.Message))
			default:
				infos++
				fmt.Printf("    %s [%s] %s\n", InfoSymbol, iss.Check, iss.Message)
			}
		}
	}

	fmt.Println()
	fmt.Println(Panel("Analysis Results",
		KeyValue("Files", NumberStyle.Render(fmt.Sprintf("%d", len(docFiles))))+"\n"+
			KeyValue("Errors", ErrorStyle.Render(fmt.Sprintf("%d", errors)))+"\n"+
			KeyValue("Warnings", WarningStyle.Render(fmt.Sprintf("%d", warnings)))+"\n"+
			KeyValue("Improvements", InfoStyle.Render(fmt.Sprintf("%d", infos))),
	))
}

// PrintFormatSummary prints format.Convert's per-document detection
// results (id/title/counts), then any verify findings (grouped by file, in
// the order Analyze produced them — already document order), then a
// closing Panel with totals, the suggested theme (if any), and outDir.
func PrintFormatSummary(report *format.Report, verifyIssues []analyze.Issue, outDir string) {
	fmt.Println()
	for _, doc := range report.Documents {
		fmt.Printf("  %s %s\n", SuccessSymbol, KeyValue(doc.ID, doc.Title))
		fmt.Printf("      %s\n", MutedStyle.Render(fmt.Sprintf(
			"%s — %d heading(s), %d list(s), %d code block(s), %d paragraph(s)",
			doc.Filename, doc.Chapter.Headings, doc.Chapter.Lists, doc.Chapter.CodeBlocks, doc.Chapter.Paragraphs)))
	}

	errors, warnings, infos := 0, 0, 0
	if len(verifyIssues) > 0 {
		fmt.Println()
		fmt.Println("  " + HeadingStyle.Render("Verification"))
		lastFile := ""
		for _, iss := range verifyIssues {
			if iss.File != lastFile {
				fmt.Printf("  %s\n", FilePathStyle.Render(iss.File))
				lastFile = iss.File
			}
			switch iss.Severity {
			case analyze.SeverityError:
				errors++
				fmt.Printf("    %s [%s] %s\n", ErrorSymbol, iss.Check, ErrorStyle.Render(iss.Message))
			case analyze.SeverityWarning:
				warnings++
				fmt.Printf("    %s [%s] %s\n", WarningSymbol, iss.Check, WarningStyle.Render(iss.Message))
			default:
				infos++
				fmt.Printf("    %s [%s] %s\n", InfoSymbol, iss.Check, iss.Message)
			}
		}
	}

	lines := []string{
		KeyValue("Documents", NumberStyle.Render(fmt.Sprintf("%d", len(report.Documents)))),
		KeyValue("Headings", fmt.Sprintf("%d", report.TotalHeadings)),
		KeyValue("Lists", fmt.Sprintf("%d", report.TotalLists)),
		KeyValue("Code blocks", fmt.Sprintf("%d", report.TotalCodeBlocks)),
	}
	if report.SuggestedTheme != "" {
		lines = append(lines, KeyValue("Suggested theme", fmt.Sprintf("%s (%s)", report.SuggestedTheme, report.SuggestedCategory)))
	}
	if len(verifyIssues) > 0 {
		lines = append(lines, KeyValue("Verify errors", ErrorStyle.Render(fmt.Sprintf("%d", errors))))
		lines = append(lines, KeyValue("Verify warnings", WarningStyle.Render(fmt.Sprintf("%d", warnings))))
		if infos > 0 {
			lines = append(lines, KeyValue("Verify improvements", InfoStyle.Render(fmt.Sprintf("%d", infos))))
		}
	}
	lines = append(lines, KeyValue("Output", outDir))

	fmt.Println()
	fmt.Println(Panel("Format Complete!", strings.Join(lines, "\n")))
	fmt.Println()
	fmt.Println("  " + MutedStyle.Render("Next:") +
		" " + CodeStyle.Render(fmt.Sprintf("cd %s && pretty-converter check", outDir)))
}

type PreFlightResult struct {
	Name    string
	Passed  bool
	Message string
	Warning bool
}

func PrintPreFlight(results []PreFlightResult) {
	fmt.Println()
	fmt.Println("  " + HeadingStyle.Render("Pre-flight checks"))
	fmt.Println()

	for _, r := range results {
		if r.Passed {
			fmt.Printf("  %s %s\n", SuccessSymbol, r.Name)
		} else if r.Warning {
			fmt.Printf("  %s %s — %s\n", WarningSymbol, r.Name, WarningStyle.Render(r.Message))
		} else {
			fmt.Printf("  %s %s — %s\n", ErrorSymbol, r.Name, ErrorStyle.Render(r.Message))
		}
	}

	failed := 0
	for _, r := range results {
		if !r.Passed && !r.Warning {
			failed++
		}
	}

	if failed > 0 {
		fmt.Println()
		fmt.Printf("  %s %s\n", ErrorSymbol, ErrorStyle.Render(fmt.Sprintf("%d pre-flight check(s) failed", failed)))
	}

	fmt.Println()
}

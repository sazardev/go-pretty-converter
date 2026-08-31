package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sazardev/go-pretty-pdf/analyze"
	"github.com/sazardev/go-pretty-pdf/cmd/pretty-pdf/output"
)

func analyzeOptsFromFlags() analyze.Options {
	opts := analyze.DefaultOptions()
	if analyzeMaxTableColumns > 0 {
		opts.MaxTableColumns = analyzeMaxTableColumns
	}
	if analyzeMaxCodeLineLength > 0 {
		opts.MaxCodeLineLength = analyzeMaxCodeLineLength
	}
	if analyzeMaxListDepth > 0 {
		opts.MaxListDepth = analyzeMaxListDepth
	}
	if analyzeLongChapterWords > 0 {
		opts.LongChapterWords = analyzeLongChapterWords
	}
	return opts
}

func countBySeverity(issues []analyze.Issue) (errorsCount, warningsCount int) {
	for _, iss := range issues {
		switch iss.Severity {
		case analyze.SeverityError:
			errorsCount++
		case analyze.SeverityWarning:
			warningsCount++
		}
	}
	return errorsCount, warningsCount
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	if noColor {
		output.NoColor()
	}

	if jsonOutput {
		return runAnalyzeJSON(cmd)
	}

	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	if !quiet {
		fmt.Println("  " + output.KeyValue("Source", cfg.Source))
		fmt.Println()
	}

	parser := parserFromConfig(cfg)

	spinner := output.StartSpinner("Analyzing MDX files...")
	docs, err := parser.ParseDir(cfg.Source)
	if err != nil && len(docs) == 0 {
		spinner.Fail(err.Error())
		return fmt.Errorf("parsing: %w", err)
	}
	spinner.Done(fmt.Sprintf("Found %d document(s)", len(docs)))
	if err != nil {
		fmt.Printf("    %s\n", output.Warn(fmt.Sprintf("Some files failed to parse: %v", err)))
	}

	issues := analyze.Analyze(docs, analyzeOptsFromFlags())
	errorsCount, warningsCount := countBySeverity(issues)

	docFiles := make([]string, len(docs))
	for i, d := range docs {
		docFiles[i] = d.Path
	}

	if !quiet {
		output.PrintAnalysisSummary(issues, docFiles)
	} else if errorsCount > 0 || (strict && warningsCount > 0) {
		for _, iss := range issues {
			if iss.Severity == analyze.SeverityError || (strict && iss.Severity == analyze.SeverityWarning) {
				fmt.Printf("%s: [%s] %s: %s\n", iss.Severity, iss.Check, iss.File, iss.Message)
			}
		}
	}

	failing := errorsCount
	if strict {
		failing += warningsCount
	}
	if failing > 0 {
		return fmt.Errorf("analysis found %d blocking issue(s) (%d error(s), %d warning(s))", failing, errorsCount, warningsCount)
	}

	if warningsCount > 0 && !strict && !quiet {
		fmt.Println(output.Success("Analysis passed — see warnings/improvements above."))
	}

	return nil
}

type analyzeJSONIssue struct {
	File     string `json:"file"`
	DocID    string `json:"doc_id"`
	DocTitle string `json:"doc_title"`
	Check    string `json:"check"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type analyzeJSONResult struct {
	Documents int                `json:"documents"`
	Errors    int                `json:"errors"`
	Warnings  int                `json:"warnings"`
	Info      int                `json:"info"`
	Issues    []analyzeJSONIssue `json:"issues"`
}

func runAnalyzeJSON(cmd *cobra.Command) error {
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	parser := parserFromConfig(cfg)

	docs, err := parser.ParseDir(cfg.Source)
	if err != nil && len(docs) == 0 {
		return fmt.Errorf("parsing: %w", err)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: some files failed to parse: %v\n", err)
	}

	issues := analyze.Analyze(docs, analyzeOptsFromFlags())
	errorsCount, warningsCount := countBySeverity(issues)

	result := analyzeJSONResult{
		Documents: len(docs),
		Errors:    errorsCount,
		Warnings:  warningsCount,
		Info:      len(issues) - errorsCount - warningsCount,
		Issues:    make([]analyzeJSONIssue, 0, len(issues)),
	}
	for _, iss := range issues {
		result.Issues = append(result.Issues, analyzeJSONIssue{
			File:     iss.File,
			DocID:    iss.DocID,
			DocTitle: iss.DocTitle,
			Check:    iss.Check,
			Severity: string(iss.Severity),
			Message:  iss.Message,
		})
	}

	out, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encoding JSON result: %w", err)
	}
	fmt.Println(string(out))

	failing := errorsCount
	if strict {
		failing += warningsCount
	}
	if failing > 0 {
		return fmt.Errorf("analysis found %d blocking issue(s)", failing)
	}
	return nil
}

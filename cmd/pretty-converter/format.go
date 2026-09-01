package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sazardev/go-pretty-converter/analyze"
	"github.com/sazardev/go-pretty-converter/cmd/pretty-converter/output"
	"github.com/sazardev/go-pretty-converter/format"
	"github.com/sazardev/go-pretty-converter/mdx"
)

// resolveFormatInputs resolves inputPath to a sorted list of .txt files:
// itself if it's a single .txt file, or every .txt file found by a
// recursive walk if it's a directory — mirroring mdx.Parser.ParseDir's own
// walk+sort convention for source directories.
func resolveFormatInputs(inputPath string) ([]string, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", inputPath, err)
	}

	if !info.IsDir() {
		if !strings.HasSuffix(strings.ToLower(inputPath), ".txt") {
			return nil, fmt.Errorf("%s is not a .txt file", inputPath)
		}
		return []string{inputPath}, nil
	}

	var paths []string
	err = filepath.WalkDir(inputPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".txt") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", inputPath, err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no .txt files found in %s", inputPath)
	}
	sort.Strings(paths)
	return paths, nil
}

// formatYAMLOptions resolves --title/--author for the scaffolded
// go-pretty-converter.yml. title/author are shared package globals also
// registered on build/epub/kindle/init, so only an explicitly-passed flag
// (cmd.Flags().Changed) is trusted — exactly like initValues in init.go.
func formatYAMLOptions(cmd *cobra.Command) format.YAMLOptions {
	var y format.YAMLOptions
	if cmd.Flags().Changed("title") {
		y.Title = title
	}
	if cmd.Flags().Changed("author") {
		y.Author = author
	}
	return y
}

// verifyFormatOutput re-parses outDir/book with the real mdx.Parser and
// runs the existing analyze.Analyze linter over it — both an implicit
// correctness check (generated MDX must parse cleanly) and free reuse of
// analyze's quality findings. Parse failures are reported as a single
// synthetic issue rather than failing the command: format's verify step is
// purely informational (see runFormat's doc comment on exit status).
func verifyFormatOutput(outDir string) []analyze.Issue {
	bookDir := filepath.Join(outDir, "book")
	docs, err := mdx.NewParser().ParseDir(bookDir)
	if err != nil && len(docs) == 0 {
		return []analyze.Issue{{
			File:     bookDir,
			Check:    "verify",
			Severity: analyze.SeverityError,
			Message:  fmt.Sprintf("generated output failed to parse: %v", err),
		}}
	}
	return analyze.Analyze(docs, analyze.DefaultOptions())
}

func runFormat(cmd *cobra.Command, args []string) error {
	if noColor {
		output.NoColor()
	}

	if jsonOutput {
		return runFormatJSON(cmd, args)
	}

	paths, err := resolveFormatInputs(args[0])
	if err != nil {
		return err
	}

	if !quiet {
		fmt.Println("  " + output.KeyValue("Input", args[0]))
		fmt.Println("  " + output.KeyValue("Output", formatOutPath))
		fmt.Println()
	}

	spinner := output.StartSpinner("Analyzing text structure...")
	report, err := format.Convert(paths, format.DefaultOptions())
	if err != nil {
		spinner.Fail(err.Error())
		return fmt.Errorf("converting: %w", err)
	}
	spinner.Done(fmt.Sprintf("Detected %d document(s)", len(report.Documents)))

	writeSpinner := output.StartSpinner("Writing structured .mdx files...")
	if err := format.Write(report, formatOutPath, formatYAMLOptions(cmd), formatForce); err != nil {
		writeSpinner.Fail(err.Error())
		return fmt.Errorf("writing output: %w", err)
	}
	writeSpinner.Done(fmt.Sprintf("Wrote %s", formatOutPath))

	var verifyIssues []analyze.Issue
	if !formatNoVerify {
		verifyIssues = verifyFormatOutput(formatOutPath)
	}

	if !quiet {
		output.PrintFormatSummary(report, verifyIssues, formatOutPath)
	}

	return nil
}

type formatJSONDoc struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Filename   string `json:"filename"`
	Headings   int    `json:"headings"`
	Lists      int    `json:"lists"`
	CodeBlocks int    `json:"code_blocks"`
	Paragraphs int    `json:"paragraphs"`
}

type formatJSONResult struct {
	OutDir            string             `json:"out_dir"`
	Documents         []formatJSONDoc    `json:"documents"`
	TotalHeadings     int                `json:"total_headings"`
	TotalLists        int                `json:"total_lists"`
	TotalCodeBlocks   int                `json:"total_code_blocks"`
	TotalParagraphs   int                `json:"total_paragraphs"`
	SuggestedTheme    string             `json:"suggested_theme,omitempty"`
	SuggestedCategory string             `json:"suggested_category,omitempty"`
	VerifyIssues      []analyzeJSONIssue `json:"verify_issues"`
	VerifyErrors      int                `json:"verify_errors"`
	VerifyWarnings    int                `json:"verify_warnings"`
}

func runFormatJSON(cmd *cobra.Command, args []string) error {
	paths, err := resolveFormatInputs(args[0])
	if err != nil {
		return err
	}

	report, err := format.Convert(paths, format.DefaultOptions())
	if err != nil {
		return fmt.Errorf("converting: %w", err)
	}

	if writeErr := format.Write(report, formatOutPath, formatYAMLOptions(cmd), formatForce); writeErr != nil {
		return fmt.Errorf("writing output: %w", writeErr)
	}

	var verifyIssues []analyze.Issue
	if !formatNoVerify {
		verifyIssues = verifyFormatOutput(formatOutPath)
	}
	verifyErrors, verifyWarnings := countBySeverity(verifyIssues)

	result := formatJSONResult{
		OutDir:            formatOutPath,
		Documents:         make([]formatJSONDoc, 0, len(report.Documents)),
		TotalHeadings:     report.TotalHeadings,
		TotalLists:        report.TotalLists,
		TotalCodeBlocks:   report.TotalCodeBlocks,
		TotalParagraphs:   report.TotalParagraphs,
		SuggestedTheme:    report.SuggestedTheme,
		SuggestedCategory: report.SuggestedCategory,
		VerifyIssues:      make([]analyzeJSONIssue, 0, len(verifyIssues)),
		VerifyErrors:      verifyErrors,
		VerifyWarnings:    verifyWarnings,
	}
	for _, doc := range report.Documents {
		result.Documents = append(result.Documents, formatJSONDoc{
			ID:         doc.ID,
			Title:      doc.Title,
			Filename:   doc.Filename,
			Headings:   doc.Chapter.Headings,
			Lists:      doc.Chapter.Lists,
			CodeBlocks: doc.Chapter.CodeBlocks,
			Paragraphs: doc.Chapter.Paragraphs,
		})
	}
	for _, iss := range verifyIssues {
		result.VerifyIssues = append(result.VerifyIssues, analyzeJSONIssue{
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
	return nil
}

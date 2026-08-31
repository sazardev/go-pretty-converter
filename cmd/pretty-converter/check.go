package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sazardev/go-pretty-converter/cmd/pretty-converter/output"
	"github.com/sazardev/go-pretty-converter/mdx"
)

func runCheck(cmd *cobra.Command, args []string) error {
	if noColor {
		output.NoColor()
	}

	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	if !quiet {
		lintMode := "lenient"
		if strict {
			lintMode = "strict"
		}
		fmt.Println("  " + output.KeyValue("Source", cfg.Source))
		fmt.Println("  " + output.KeyValue("Lint mode", lintMode) +
			"  " + output.KeyValue("Max heading depth", fmt.Sprintf("h%d", cfg.Lint.MaxHeadingDepth)))
		fmt.Println()
	}

	validator := validatorFromConfig(cfg)
	parser := parserFromConfig(cfg)

	spinner := output.StartSpinner("Checking MDX files...")
	docs, err := parser.ParseDir(cfg.Source)
	if err != nil && len(docs) == 0 {
		spinner.Fail(err.Error())
		return fmt.Errorf("parsing: %w", err)
	}
	if err != nil {
		spinner.Done(fmt.Sprintf("Found %d document(s)", len(docs)))
		fmt.Printf("    %s\n", output.Warn(fmt.Sprintf("Some files failed to parse: %v", err)))
	} else {
		spinner.Done(fmt.Sprintf("Found %d document(s)", len(docs)))
	}

	errs := validator.ValidateAll(docs)

	warnings := 0
	errors := 0

	for _, e := range errs {
		if e.Field == mdx.ContentField && !strict {
			warnings++
		} else {
			errors++
		}
	}

	docFiles := make([]string, len(docs))
	for i, d := range docs {
		docFiles[i] = d.Path
	}

	if !quiet {
		output.PrintValidationSummary(errs, warnings, docFiles)
	} else {
		if errors > 0 {
			for _, e := range errs {
				if e.Field != mdx.ContentField || strict {
					fmt.Printf("ERROR: %v\n", e)
				}
			}
		}
	}

	if errors > 0 {
		return fmt.Errorf("validation failed with %d error(s)", errors)
	}

	if warnings > 0 && !strict && !quiet {
		fmt.Println(output.Success("Check passed with warnings."))
	}

	return nil
}

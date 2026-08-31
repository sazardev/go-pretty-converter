package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	prettypdf "github.com/sazardev/go-pretty-pdf"
	"github.com/sazardev/go-pretty-pdf/cmd/pretty-pdf/output"
	"github.com/sazardev/go-pretty-pdf/kindle"
	"github.com/sazardev/go-pretty-pdf/mdx"
)

func runKindle(cmd *cobra.Command, args []string) error {
	if noColor {
		output.NoColor()
	}

	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	// --out beats the config file's output, mirroring runEpub: a config
	// output ending in .pdf/.epub maps to the .mobi variant.
	kindleOutputPath := kindleOutPath
	if !cmd.Flags().Changed("out") && cfg.Output != "" {
		kindleOutputPath = resolveOutputPaths(cfg.Output, []prettypdf.OutputFormat{prettypdf.FormatKindle})[prettypdf.FormatKindle]
	}

	calibreExecPath, err := kindle.ResolveCalibre(calibrePath)
	if err != nil {
		return err
	}

	if !quiet {
		fmt.Println("  " + output.KeyValue("Source", cfg.Source))
		fmt.Println("  " + output.KeyValue("Output", kindleOutputPath))
		fmt.Println("  " + output.KeyValue("Calibre", calibreExecPath))
		fmt.Println()
	}

	parser := parserFromConfig(cfg)
	validator := validatorFromConfig(cfg)

	spinner := output.StartSpinner("Parsing MDX files...")
	docs, err := parser.ParseDir(cfg.Source)
	if err != nil && len(docs) == 0 {
		spinner.Fail(err.Error())
		return fmt.Errorf("parsing: %w", err)
	}
	spinner.Done(fmt.Sprintf("Found %d document(s)", len(docs)))
	if err != nil {
		fmt.Printf("    %s\n", output.Warn(fmt.Sprintf("Some files failed to parse: %v", err)))
	}

	errs := validator.ValidateAll(docs)
	errorCount := 0
	for _, e := range errs {
		if e.Field != mdx.ContentField {
			errorCount++
			fmt.Printf("  %v\n", e)
		}
	}
	if errorCount > 0 {
		return fmt.Errorf("validation failed with %d error(s)", errorCount)
	}

	css, err := resolveEpubCSS(cfg)
	if err != nil {
		return fmt.Errorf("resolving theme: %w", err)
	}

	kindleOpts := kindle.Options{
		EPUB:        epubOptionsFromConfig(cfg, css, kindleLanguage),
		CalibrePath: calibreExecPath,
	}

	writeSpinner := output.StartSpinner("Converting to Kindle format...")
	if err := kindle.Write(cmd.Context(), docs, kindleOpts, kindleOutputPath); err != nil {
		writeSpinner.Fail(err.Error())
		return fmt.Errorf("writing Kindle file: %w", err)
	}

	size := unknownFileSize
	if info, statErr := os.Stat(kindleOutputPath); statErr == nil {
		size = formatBytes(info.Size())
	}
	writeSpinner.Done(fmt.Sprintf("Wrote %s (%s)", kindleOutputPath, size))

	return nil
}

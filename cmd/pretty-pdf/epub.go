package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	prettypdf "github.com/sazardev/go-pretty-pdf"
	"github.com/sazardev/go-pretty-pdf/cmd/pretty-pdf/output"
	"github.com/sazardev/go-pretty-pdf/config"
	"github.com/sazardev/go-pretty-pdf/epub"
	"github.com/sazardev/go-pretty-pdf/mdx"
	"github.com/sazardev/go-pretty-pdf/theme"
)

func runEpub(cmd *cobra.Command, args []string) error {
	if noColor {
		output.NoColor()
	}

	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	// --out beats the config file's output; when neither an explicit flag
	// nor a config output is present, the default is out.epub. A config
	// output ending in .pdf (the build default) maps to the .epub variant,
	// mirroring how `build --format epub` derives its output path.
	epubOutputPath := epubOutPath
	if !cmd.Flags().Changed("out") && cfg.Output != "" {
		epubOutputPath = resolveOutputPaths(cfg.Output, []prettypdf.OutputFormat{prettypdf.FormatEPUB})[prettypdf.FormatEPUB]
	}

	if !quiet {
		fmt.Println("  " + output.KeyValue("Source", cfg.Source))
		fmt.Println("  " + output.KeyValue("Output", epubOutputPath))
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

	opts := epub.DefaultOptions()
	if cfg.Title != "" {
		opts.Title = cfg.Title
	}
	opts.Subtitle = cfg.Subtitle
	if cfg.Author != "" {
		opts.Author = cfg.Author
	}
	if epubLanguage != "" {
		opts.Language = epubLanguage
	}
	opts.CoverImage = cfg.Render.CoverImage
	opts.CSS = css

	writeSpinner := output.StartSpinner("Writing EPUB...")
	if err := epub.Write(docs, opts, epubOutputPath); err != nil {
		writeSpinner.Fail(err.Error())
		return fmt.Errorf("writing EPUB: %w", err)
	}

	size := "unknown"
	if info, statErr := os.Stat(epubOutputPath); statErr == nil {
		size = formatBytes(info.Size())
	}
	writeSpinner.Done(fmt.Sprintf("Wrote %s (%s)", epubOutputPath, size))

	return nil
}

func resolveEpubCSS(cfg *config.Config) (string, error) {
	cwd, _ := os.Getwd()

	if cfg.CSS != "" {
		data, err := os.ReadFile(cfg.CSS)
		if err != nil {
			return "", fmt.Errorf("reading CSS file %s: %w", cfg.CSS, err)
		}
		return string(data), nil
	}

	themeLabel := cfg.Theme
	if themeLabel == "" {
		themeLabel = defaultTheme
	}

	return theme.ResolveByNameForEPUB(themeLabel, themeOptionsFromConfig(cfg), cwd)
}

func themeOptionsFromConfig(cfg *config.Config) theme.Options {
	to := cfg.ThemeOptions
	return theme.Options{
		Colors: theme.Colors{
			Primary:    to.Colors.Primary,
			Accent:     to.Colors.Accent,
			Text:       to.Colors.Text,
			Muted:      to.Colors.Muted,
			Background: to.Colors.Background,
		},
		Fonts: theme.Fonts{
			Heading:       to.Fonts.Heading,
			Body:          to.Fonts.Body,
			Code:          to.Fonts.Code,
			GoogleImports: to.Fonts.GoogleFonts,
		},
		Density:           theme.Density(to.Density),
		AllowNetworkFonts: to.AllowNetworkFonts,
	}
}

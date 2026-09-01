package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	prettyconverter "github.com/sazardev/go-pretty-converter"
	"github.com/sazardev/go-pretty-converter/config"
	"github.com/sazardev/go-pretty-converter/mdx"
)

const (
	defaultTheme      = "default"
	outputDirWritable = "Output directory writable"
	coverImageExists  = "Cover image exists"
	unknownFileSize   = "unknown"
)

// applyFastMode is --fast's implementation: a shorthand for the four
// existing speed flags (--no-header --no-page-numbers --no-outline
// --no-tagged-pdf) together, since remembering and typing all four is the
// actual barrier to using them — see BENCHMARKS.md for why header/footer
// (page numbers) is now measured as the single most expensive post-print
// step, ahead of outline+tagging combined. Only forces a flag the user
// didn't already set explicitly, so e.g. `--fast --no-page-numbers=false`
// still keeps page numbers.
func applyFastMode(cmd *cobra.Command) {
	if !fastMode {
		return
	}
	if !cmd.Flags().Changed("no-header") {
		noHeader = true
	}
	if !cmd.Flags().Changed("no-page-numbers") {
		noPageNumbers = true
	}
	if !cmd.Flags().Changed("no-outline") {
		noOutline = true
	}
	if !cmd.Flags().Changed("no-tagged-pdf") {
		noTagged = true
	}
}

func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	applyFastMode(cmd)

	cfg := config.Default()

	configPath := cfgFile
	if configPath == "" {
		found, err := config.FindConfig()
		if err != nil {
			return nil, err
		}
		configPath = found
	}

	if configPath != "" {
		loaded, err := config.Load(configPath)
		if err != nil {
			return nil, fmt.Errorf("loading config: %w", err)
		}
		cfg = loaded

		configDir := filepath.Dir(configPath)
		if cfg.CSS != "" && !filepath.IsAbs(cfg.CSS) {
			cfg.CSS = filepath.Join(configDir, cfg.CSS)
		}
		if cfg.Template != "" && !filepath.IsAbs(cfg.Template) {
			cfg.Template = filepath.Join(configDir, cfg.Template)
		}
		if cfg.Render.CoverImage != "" && !filepath.IsAbs(cfg.Render.CoverImage) {
			cfg.Render.CoverImage = filepath.Join(configDir, cfg.Render.CoverImage)
		}
	}

	if cmd.Flags().Changed("source") {
		cfg.Source = sourceDir
	}
	if cmd.Flags().Changed("out") {
		cfg.Output = outPath
	}
	if cmd.Flags().Changed("title") {
		cfg.Title = title
	}
	if cmd.Flags().Changed("subtitle") {
		cfg.Subtitle = subtitle
	}
	if cmd.Flags().Changed("author") {
		cfg.Author = author
	}
	if cmd.Flags().Changed("theme") {
		cfg.Theme = themeName
	}
	if cmd.Flags().Changed("css") {
		cfg.CSS = cssPath
		if cfg.CSS != "" && !filepath.IsAbs(cfg.CSS) {
			abs, err := filepath.Abs(cfg.CSS)
			if err == nil {
				cfg.CSS = abs
			}
		}
	}
	if cmd.Flags().Changed("template") {
		cfg.Template = tmplPath
		if cfg.Template != "" && !filepath.IsAbs(cfg.Template) {
			abs, err := filepath.Abs(cfg.Template)
			if err == nil {
				cfg.Template = abs
			}
		}
	}
	if cmd.Flags().Changed("timeout") {
		cfg.Render.Timeout = timeoutStr
	}
	if cmd.Flags().Changed("cover-image") {
		cfg.Render.CoverImage = coverImage
		if cfg.Render.CoverImage != "" && !filepath.IsAbs(cfg.Render.CoverImage) {
			abs, err := filepath.Abs(cfg.Render.CoverImage)
			if err == nil {
				cfg.Render.CoverImage = abs
			}
		}
	}

	if cmd.Flags().Changed("no-cover") {
		cfg.ThemeOptions.Sections.Cover = boolPtr(!noCover)
	}
	if cmd.Flags().Changed("no-toc") {
		cfg.ThemeOptions.Sections.TOC = boolPtr(!noTOC)
	}
	if cmd.Flags().Changed("no-page-numbers") || fastMode {
		cfg.ThemeOptions.Sections.PageNumbers = boolPtr(!noPageNumbers)
	}
	if cmd.Flags().Changed("no-header") || fastMode {
		cfg.ThemeOptions.Sections.Header = boolPtr(!noHeader)
	}
	if cmd.Flags().Changed("color-primary") {
		cfg.ThemeOptions.Colors.Primary = colorPrimary
	}
	if cmd.Flags().Changed("color-accent") {
		cfg.ThemeOptions.Colors.Accent = colorAccent
	}
	if cmd.Flags().Changed("color-text") {
		cfg.ThemeOptions.Colors.Text = colorText
	}
	if cmd.Flags().Changed("color-muted") {
		cfg.ThemeOptions.Colors.Muted = colorMuted
	}
	if cmd.Flags().Changed("color-bg") {
		cfg.ThemeOptions.Colors.Background = colorBg
	}
	if cmd.Flags().Changed("font-heading") {
		cfg.ThemeOptions.Fonts.Heading = fontHeading
	}
	if cmd.Flags().Changed("font-body") {
		cfg.ThemeOptions.Fonts.Body = fontBody
	}
	if cmd.Flags().Changed("font-code") {
		cfg.ThemeOptions.Fonts.Code = fontCode
	}
	if cmd.Flags().Changed("density") {
		cfg.ThemeOptions.Density = density
	}
	if cmd.Flags().Changed("allow-network-fonts") {
		cfg.ThemeOptions.AllowNetworkFonts = allowNetworkFonts
	}

	if verbose {
		if cfg.CSS != "" {
			if _, err := os.Stat(cfg.CSS); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: CSS file not found: %s\n", cfg.CSS)
			}
		}
		if cfg.Template != "" {
			if _, err := os.Stat(cfg.Template); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: template file not found: %s\n", cfg.Template)
			}
		}
		if cfg.Render.CoverImage != "" {
			if _, err := os.Stat(cfg.Render.CoverImage); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: cover image not found: %s\n", cfg.Render.CoverImage)
			}
		}
	}

	return cfg, nil
}

func buildOpts(cfg *config.Config, chromeExecPath, calibreExecPath string) []prettyconverter.Option {
	opts := []prettyconverter.Option{
		prettyconverter.WithVerbose(verbose),
		prettyconverter.WithFullConfig(cfg),
		prettyconverter.WithNetworkAccess(cfg.ThemeOptions.AllowNetworkFonts),
		prettyconverter.WithValidator(validatorFromConfig(cfg)),
		prettyconverter.WithChromeExecPath(chromeExecPath),
		prettyconverter.WithCalibreExecPath(calibreExecPath),
	}
	if noOutline {
		opts = append(opts, prettyconverter.WithGenerateDocumentOutline(false))
	}
	if noTagged {
		opts = append(opts, prettyconverter.WithGenerateTaggedPDF(false))
	}
	return opts
}

func boolPtr(b bool) *bool { return &b }

func validatorFromConfig(cfg *config.Config) *mdx.DefaultValidator {
	v := mdx.NewDefaultValidator()
	if len(cfg.Lint.RequireFrontmatter) > 0 {
		v.RequireFrontmatter = cfg.Lint.RequireFrontmatter
	}
	v.NoDuplicateIDs = cfg.Lint.NoDuplicateIDs
	v.MaxHeadingDepth = cfg.Lint.MaxHeadingDepth
	if v.MaxHeadingDepth == 0 {
		v.MaxHeadingDepth = 5
	}
	return v
}

func parserFromConfig(cfg *config.Config) *mdx.Parser {
	parserOpts := []mdx.ParserOption{}
	if len(cfg.Vars) > 0 {
		parserOpts = append(parserOpts, mdx.WithVars(cfg.Vars))
	}
	return mdx.NewParser(parserOpts...)
}

func cfgFileFound() bool {
	if cfgFile != "" {
		return true
	}
	path, _ := config.FindConfig()
	return path != ""
}

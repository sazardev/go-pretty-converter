package main

import (
	"embed"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sazardev/go-pretty-pdf/cmd/pretty-pdf/output"
	"github.com/sazardev/go-pretty-pdf/theme"
	"github.com/sazardev/go-pretty-pdf/version"
)

func themeNames() []string {
	names := make([]string, 0, len(theme.List()))
	for _, t := range theme.List() {
		names = append(names, t.Name)
	}
	return names
}

//go:embed initassets/*
var initAssets embed.FS

var (
	cfgFile    string
	sourceDir  string
	chromePath string
	outPath    string
	title      string
	subtitle   string
	author     string
	themeName  string
	cssPath    string
	tmplPath   string
	coverImage string
	timeoutStr string
	verbose    bool
	strict     bool
	noColor    bool
	quiet      bool
	jsonOutput bool
	initBare   bool
	servePort  int

	noCover           bool
	noTOC             bool
	noPageNumbers     bool
	noHeader          bool
	colorPrimary      string
	colorAccent       string
	colorText         string
	colorMuted        string
	colorBg           string
	fontHeading       string
	fontBody          string
	fontCode          string
	density           string
	allowNetworkFonts bool

	epubOutPath  string
	epubLanguage string

	formatStr     string
	buildLanguage string
)

var rootCmd = &cobra.Command{
	Use:   "pretty-pdf",
	Short: "Transform MDX files into beautiful, print-ready PDFs",
	Long: output.PrimaryStyle.Render(`
  go-pretty-pdf transforms a directory of MDX files into a print-ready PDF
  via headless Chrome. Documents are sorted by their [X.Y.Z] frontmatter ID.
  Supports custom components, themes, CSS overrides, and YAML configuration.
`) + "\n  " + output.MutedStyle.Render("https://github.com/sazardev/go-pretty-pdf"),
	SilenceUsage: true,
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build PDF and/or EPUB from MDX source files",
	Long: `Parse MDX files, validate them, compose HTML, and render to PDF and/or EPUB.

Use --format to pick output formats (comma-separated): pdf, epub, or pdf,epub.
When --out is a base name (no extension), the appropriate extension is appended
for each format. Chrome is only required when PDF is in the format list.

Pick a theme with --theme (see 'pretty-pdf theme list'), then customize it
without writing CSS via --color-*/--font-*/--density, or drop sections with
--no-cover/--no-toc/--no-page-numbers/--no-header.`,
	Example: `  pretty-pdf build --format pdf,epub --out mybook
  pretty-pdf build --format epub --out mybook.epub
  pretty-pdf build --theme corporate --color-primary "#0ea5e9"
  pretty-pdf build --theme dark --no-cover --no-page-numbers
  pretty-pdf build --theme my-custom-theme --density compact
  pretty-pdf build --css custom.css --template custom.html`,
	RunE: runBuild,
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate MDX files",
	Long:  "Parse and validate all MDX files in the source directory without building a PDF.",
	RunE:  runCheck,
}

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Scaffold a new book project",
	Long: `Scaffold a new book project with a sample MDX file, configuration, and directory structure.
Run 'pretty-pdf init my-book' to create a new project in the 'my-book' directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch for changes and rebuild automatically",
	Long:  "Watch the source directory for changes and rebuild the PDF on every file change.",
	RunE:  runWatch,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(output.PrimaryStyle.Render("go-pretty-pdf") + " " + output.MutedStyle.Render("v"+version.Version))
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Preview MDX as HTML in the browser",
	Long:  "Parse MDX files, compose HTML, and serve with live reload on file changes. No Chrome required.",
	RunE:  runServe,
}

var epubCmd = &cobra.Command{
	Use:   "epub",
	Short: "Build an EPUB from MDX source files",
	Long: `Parse MDX files, validate them, and write a single EPUB 3 file — no
Chrome/Chromium required, unlike 'build'. Each MDX document becomes its own
chapter, in the same order as the PDF's table of contents.`,
	Example: `  pretty-pdf epub --title "My Book" --author "Jane Doe"
  pretty-pdf epub --cover-image cover.png --out mybook.epub`,
	RunE: runEpub,
}

func init() {
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(themeCmd)
	rootCmd.AddCommand(epubCmd)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "path to config file")
	rootCmd.PersistentFlags().StringVar(&sourceDir, "source", "book", "source MDX directory")
	rootCmd.PersistentFlags().StringVar(&chromePath, "chrome-path", os.Getenv("PRETTY_PDF_CHROME_PATH"),
		"path to a Chrome/Chromium executable (skips auto-detection/download)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-error output")

	buildCmd.Flags().StringVar(&outPath, "out", "out.pdf", "output path (base name for multi-format, or single file path)")
	buildCmd.Flags().StringVar(&formatStr, "format", "pdf", "output format(s): pdf, epub, or pdf,epub")
	buildCmd.Flags().StringVar(&buildLanguage, "language", "en", "EPUB language (BCP-47 tag, e.g. en, es)")
	buildCmd.Flags().StringVar(&title, "title", "", "book title")
	buildCmd.Flags().StringVar(&subtitle, "subtitle", "", "book subtitle")
	buildCmd.Flags().StringVar(&author, "author", "", "book author")
	buildCmd.Flags().StringVar(&themeName, "theme", defaultTheme, fmt.Sprintf("book theme (%s, or a custom theme name/path)", strings.Join(themeNames(), ", ")))
	buildCmd.Flags().StringVar(&cssPath, "css", "", "custom CSS file path (overrides theme)")
	buildCmd.Flags().StringVar(&tmplPath, "template", "", "custom HTML template file path (overrides theme)")
	buildCmd.Flags().StringVar(&coverImage, "cover-image", "", "custom cover image (.png/.jpg/.jpeg/.svg/.webp); the cover page is sized to the image's own dimensions, replacing the text cover")
	buildCmd.Flags().StringVar(&timeoutStr, "timeout", "", "render timeout (e.g. 30s, 1m)")
	buildCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")

	buildCmd.Flags().BoolVar(&noCover, "no-cover", false, "omit the cover page")
	buildCmd.Flags().BoolVar(&noTOC, "no-toc", false, "omit the table of contents")
	buildCmd.Flags().BoolVar(&noPageNumbers, "no-page-numbers", false, "omit page numbers")
	buildCmd.Flags().BoolVar(&noHeader, "no-header", false, "omit the running page header")
	buildCmd.Flags().StringVar(&colorPrimary, "color-primary", "", "theme override: primary color (e.g. #1a56db)")
	buildCmd.Flags().StringVar(&colorAccent, "color-accent", "", "theme override: accent color")
	buildCmd.Flags().StringVar(&colorText, "color-text", "", "theme override: body text color")
	buildCmd.Flags().StringVar(&colorMuted, "color-muted", "", "theme override: muted/caption text color")
	buildCmd.Flags().StringVar(&colorBg, "color-bg", "", "theme override: page background color")
	buildCmd.Flags().StringVar(&fontHeading, "font-heading", "", "theme override: heading font family")
	buildCmd.Flags().StringVar(&fontBody, "font-body", "", "theme override: body font family")
	buildCmd.Flags().StringVar(&fontCode, "font-code", "", "theme override: code font family")
	buildCmd.Flags().StringVar(&density, "density", "", "spacing density: compact, normal, or relaxed")
	buildCmd.Flags().BoolVar(&allowNetworkFonts, "allow-network-fonts", false, "allow fetching Google Fonts declared by the theme (enables network access)")

	checkCmd.Flags().BoolVar(&strict, "strict", false, "treat content warnings as errors")

	initCmd.Flags().BoolVar(&initBare, "bare", false, "non-interactive init with flags")
	initCmd.Flags().StringVar(&title, "title", "My Book", "book title (for --bare)")
	initCmd.Flags().StringVar(&author, "author", "go-pretty-pdf", "book author (for --bare)")
	initCmd.Flags().StringVar(&themeName, "theme", defaultTheme, "book theme (for --bare)")
	initCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")

	serveCmd.Flags().IntVar(&servePort, "port", 8080, "HTTP server port")

	epubCmd.Flags().StringVar(&epubOutPath, "out", "out.epub", "output EPUB path")
	epubCmd.Flags().StringVar(&title, "title", "", "book title")
	epubCmd.Flags().StringVar(&subtitle, "subtitle", "", "book subtitle")
	epubCmd.Flags().StringVar(&author, "author", "", "book author")
	epubCmd.Flags().StringVar(&coverImage, "cover-image", "", "custom cover image (.png/.jpg/.jpeg/.svg/.webp), full-bleed as the first page")
	epubCmd.Flags().StringVar(&epubLanguage, "language", "en", "book language (BCP-47 tag, e.g. en, es)")

	epubCmd.Flags().StringVar(&themeName, "theme", defaultTheme, fmt.Sprintf("book theme (%s, or a custom theme name/path)", strings.Join(themeNames(), ", ")))
	epubCmd.Flags().StringVar(&cssPath, "css", "", "custom CSS file path (overrides theme)")
	epubCmd.Flags().StringVar(&colorPrimary, "color-primary", "", "theme override: primary color (e.g. #1a56db)")
	epubCmd.Flags().StringVar(&colorAccent, "color-accent", "", "theme override: accent color")
	epubCmd.Flags().StringVar(&colorText, "color-text", "", "theme override: body text color")
	epubCmd.Flags().StringVar(&colorMuted, "color-muted", "", "theme override: muted/caption text color")
	epubCmd.Flags().StringVar(&colorBg, "color-bg", "", "theme override: page background color")
	epubCmd.Flags().StringVar(&fontHeading, "font-heading", "", "theme override: heading font family")
	epubCmd.Flags().StringVar(&fontBody, "font-body", "", "theme override: body font family")
	epubCmd.Flags().StringVar(&fontCode, "font-code", "", "theme override: code font family")
	epubCmd.Flags().StringVar(&density, "density", "", "spacing density: compact, normal, or relaxed")
	epubCmd.Flags().BoolVar(&allowNetworkFonts, "allow-network-fonts", false, "allow fetching Google Fonts declared by the theme (enables network access)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

package main

import (
	"embed"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sazardev/go-pretty-converter/cmd/pretty-converter/output"
	"github.com/sazardev/go-pretty-converter/theme"
	"github.com/sazardev/go-pretty-converter/version"
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
	cfgFile     string
	sourceDir   string
	chromePath  string
	calibrePath string
	outPath     string
	title       string
	subtitle    string
	author      string
	themeName   string
	cssPath     string
	tmplPath    string
	coverImage  string
	timeoutStr  string
	verbose     bool
	strict      bool
	noColor     bool
	quiet       bool
	jsonOutput  bool
	initBare    bool
	servePort   int

	noCover           bool
	noTOC             bool
	noPageNumbers     bool
	noHeader          bool
	noOutline         bool
	noTagged          bool
	fastMode          bool
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

	kindleOutPath  string
	kindleLanguage string
	kindleNoVerify bool

	analyzeMaxTableColumns   int
	analyzeMaxCodeLineLength int
	analyzeMaxListDepth      int
	analyzeLongChapterWords  int

	formatStr     string
	buildLanguage string

	formatOutPath  string
	formatForce    bool
	formatNoVerify bool
)

var rootCmd = &cobra.Command{
	Use:   "pretty-converter",
	Short: "The Markdown book toolchain that audits itself before you ship",
	Long: output.PrimaryStyle.Render(`
  go-pretty-converter transforms a directory of Markdown/MDX files into a print-ready
  PDF, an EPUB 3, and a Kindle MOBI/AZW3 (via Calibre) — via headless Chrome,
  as a CLI or a Go library. What sets it apart: it checks its own output.

    • 'analyze' statically flags content that renders poorly across formats
      (dead links, wide tables, skipped heading levels, ...) before any build
    • every render runs an automatic quality audit too (overflow, low
      contrast, broken anchors, unloaded fonts, ...)
    • one Markdown source → PDF + EPUB + Kindle, no separate pipeline
    • 17 built-in themes over one shared stylesheet, tweakable without CSS
      via --color-*/--font-*/--density or theme_options in go-pretty-converter.yml
    • auto table of contents, PDF bookmarks, print-ready sizes (6x9in, A5, mm/in)
    • zero-install Chrome: a headless build is downloaded on first render
    • no LaTeX, no design tool — Markdown + a binary

  Documents are ordered by their [X.Y.Z] frontmatter id, not by filename.
  A missing frontmatter block is fine: id/title are derived from the filename.

  Quick start:
    pretty-converter init my-book
    pretty-converter analyze --source my-book
    pretty-converter build --source my-book --out my-book.pdf
`) + "\n  " + output.MutedStyle.Render("https://github.com/sazardev/go-pretty-converter · https://sazardev.github.io/go-pretty-converter/"),
	Example: `  # one-shot PDF from a docs folder
  pretty-converter build --source ./docs --out ./docs.pdf

  # PDF + EPUB + Kindle in a single pass
  pretty-converter build --format pdf,epub,kindle --out mybook

  # catch rendering issues before you build anything
  pretty-converter analyze --strict --source ./docs

  # Kindle only (needs Calibre's ebook-convert)
  pretty-converter kindle --source ./docs --out ./docs.mobi`,
	SilenceUsage: true,
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build PDF, EPUB, and/or Kindle from MDX source files",
	Long: `Parse MDX files, validate them, compose HTML, and render to PDF, EPUB,
and/or Kindle (MOBI/AZW3).

Use --format to pick output formats (comma-separated): pdf, epub, kindle, or
any combination (e.g. pdf,epub,kindle). When --out is a base name (no
extension), the appropriate extension is appended for each format. Chrome is
only required when PDF is in the format list; kindle requires Calibre's
ebook-convert on PATH (or --calibre-path / PRETTY_CONVERTER_CALIBRE_PATH).

Pick a theme with --theme (see 'pretty-converter theme list'), then customize it
without writing CSS via --color-*/--font-*/--density, or drop sections with
--no-cover/--no-toc/--no-page-numbers/--no-header.

After rendering, an automatic quality audit reports anything worth a second
look (overflow, low contrast, broken anchors, unloaded fonts, empty output, ...)
and the summary's Warnings count mirrors it. The audit is advisory: a
non-zero Warnings count does not fail the build.

Large books: page numbers/header (Chrome renders them per-page) are the
single most expensive part of the render, ahead of bookmarks and
accessibility tagging combined — pass --fast (shorthand for --no-header
--no-page-numbers --no-outline --no-tagged-pdf) to cut build time on very
big documents (see BENCHMARKS.md for measured numbers).`,
	Example: `  # one-shot PDF from a docs folder
  pretty-converter build --source ./docs --out ./docs.pdf

  # large book, optimized for speed (drops page numbers/header/bookmarks/tagging)
  pretty-converter build --source ./docs --out ./docs.pdf --fast

  # PDF + EPUB in a single pass (base name gets both extensions)
  pretty-converter build --format pdf,epub --out mybook

  # branded client report
  pretty-converter build --theme corporate --color-primary "#0ea5e9" --font-heading "Georgia, serif"

  # dark theme without cover or page numbers
  pretty-converter build --theme dark --no-cover --no-page-numbers

  # everything from a config file (title, author, vars, render settings)
  pretty-converter build --config ./go-pretty-converter.yml

  # machine-readable output for tooling/CI
  pretty-converter build --json --quiet`,
	RunE: runBuild,
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate MDX files",
	Long: `Parse and validate all MDX files without building anything. No Chrome required.

Checks: required frontmatter fields, [X.Y.Z] id format, duplicate ids, maximum
heading depth, and content-level warnings. Content warnings are reported but
do not fail the run; --strict promotes them to errors — the switch for CI gates.

Exit status: 0 when the source is valid, 1 when validation fails.`,
	Example: `  pretty-converter check --source ./docs
  pretty-converter check --strict --source ./docs   # fail on content warnings too
  pretty-converter check --quiet                    # only print errors`,
	RunE: runCheck,
}

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Scaffold a new book project",
	Long: `Scaffold a new book project: a sample MDX file, go-pretty-converter.yml, and the
directory structure, ready to 'pretty-converter build' immediately.

Interactive by default: a terminal form asks for title, author, theme, and
source directory. Pass --bare to skip the form and set everything with flags.`,
	Example: `  pretty-converter init my-book
  pretty-converter init my-book --bare --title "My Book" --author "Jane Doe" --theme corporate`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch for changes and rebuild automatically",
	Long: `Watch the source directory and rebuild the PDF whenever a .md/.mdx/.txt,
.yaml, or .yml file changes (changes are debounced by 300ms). It builds once
immediately so the output exists before you edit anything.

Rendering is a PDF build, so Chrome is required. Press Ctrl+C to stop — a
summary of successful builds and errors is printed on exit.`,
	Example: `  pretty-converter watch --source ./book --out ./book.pdf`,
	RunE:    runWatch,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		if noColor {
			output.NoColor()
		}
		fmt.Println(output.PrimaryStyle.Render("go-pretty-converter") + " " + output.MutedStyle.Render(version.Version))
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Preview MDX as HTML in the browser",
	Long: `Parse the source and serve a live HTML preview with automatic reload on file
changes, delivered via Server-Sent Events. No Chrome required — the fastest
way to iterate on content before the final print render.`,
	Example: `  pretty-converter serve --source ./book --port 8080`,
	RunE:    runServe,
}

var epubCmd = &cobra.Command{
	Use:   "epub",
	Short: "Build an EPUB from MDX source files",
	Long: `Parse MDX files, validate them, and write a single EPUB 3 file — no
Chrome/Chromium required, unlike 'build'. Each MDX document becomes its own
chapter, in the same order as the PDF's table of contents.

The theme is reused for EPUB (reflowable stylesheet: relative units, no
print-only rules). --cover-image (or render.cover_image) becomes a full-bleed
first page, and --language sets the BCP-47 language tag.`,
	Example: `  pretty-converter epub --source ./book --out ./book.epub
  pretty-converter epub --title "My Book" --author "Jane Doe" --language es
  pretty-converter epub --cover-image ./cover.png --out ./book.epub`,
	RunE: runEpub,
}

var kindleCmd = &cobra.Command{
	Use:   "kindle",
	Short: "Build a Kindle-ready ebook (MOBI/AZW3) from MDX source files",
	Long: `Parse MDX files, validate them, and convert them into a Kindle-ready
ebook file — no Chrome/Chromium required, unlike 'build'. Internally this
builds the same EPUB 'pretty-converter epub' would produce, then converts it with
Calibre's ebook-convert (Amazon retired its own KindleGen tool in 2022;
Calibre is the standard replacement) — so Calibre must be installed with
ebook-convert reachable on PATH, or pointed to explicitly with
--calibre-path / PRETTY_CONVERTER_CALIBRE_PATH.

The target format (MOBI, AZW3, ...) is inferred from --out's extension;
defaults to .mobi, the most broadly compatible Kindle format.`,
	Example: `  pretty-converter kindle --source ./book --out ./book.mobi
  pretty-converter kindle --title "My Book" --author "Jane Doe" --language es
  pretty-converter kindle --out ./book.azw3   # modern Kindle format (KF8)`,
	RunE: runKindle,
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze MDX content for cross-format rendering issues",
	Long: `Parse MDX files and statically analyze their content for patterns that
render poorly — or break outright — across PDF, EPUB, and Kindle output. No
Chrome or Calibre required: analyze works directly on parsed content, before
any build.

This is a different layer from 'check': 'check' validates frontmatter and
structure (required fields, id format, duplicate ids); 'analyze' looks at
content itself — heading hierarchy, tables, images, code blocks, lists, and
chapter length — with an eye specifically on how each renders across
formats. Findings are grouped into three severities:

  errors        content that will actually break in some format
                (a dead #anchor link, a missing local image file, ...)
  warnings      content that will render, but poorly, in some format
                (a table too wide for Kindle, a skipped heading level,
                deeply nested lists, an overlong code line, ...)
  improvements  suggestions worth a look, not necessarily wrong
                (no headings, no tags, an oversized image, a remote
                image URL that PDF's network lockdown will render blank)

Exit status: 0 unless errors are found; --strict also fails on warnings —
the switch for CI gates.`,
	Example: `  pretty-converter analyze --source ./docs
  pretty-converter analyze --strict --source ./docs    # fail on warnings too
  pretty-converter analyze --json --quiet              # machine-readable output
  pretty-converter analyze --max-table-columns 4 --long-chapter-words 5000`,
	RunE: runAnalyze,
}

var formatCmd = &cobra.Command{
	Use:   "format <input>",
	Short: "Convert raw .txt into structured .mdx files",
	Long: `Analyze raw, unstructured .txt (a single file or a directory) and turn it
into clean .mdx files with proper [X.Y.Z] frontmatter, headings, lists, and
fenced code blocks — plus a scaffolded go-pretty-converter.yml, ready to
'pretty-converter check'/'build' immediately.

This is heuristic and deterministic — no AI/LLM, no network access — and
deliberately conservative: it only promotes structure it's confident about,
rather than maximizing how much it finds. It never touches Chrome and never
produces a PDF itself; the output is meant to be reviewed, then run through
the existing check/analyze/build commands like any other source.

By default the generated output is re-parsed and run through the same
content analysis 'analyze' uses, purely as an informational summary — pass
--no-verify to skip it. Exit status only reflects I/O errors (a bad input
path, or an existing --out without --force); verify findings never fail
the run.`,
	Example: `  pretty-converter format notes.txt --out formatted
  pretty-converter format ./raw-notes --out formatted --title "My Book" --author "Jane Doe"
  pretty-converter format notes.txt --out formatted --force   # overwrite existing output
  pretty-converter format notes.txt --out formatted --json --quiet`,
	Args: cobra.ExactArgs(1),
	RunE: runFormat,
}

func init() {
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(themeCmd)
	rootCmd.AddCommand(epubCmd)
	rootCmd.AddCommand(kindleCmd)
	rootCmd.AddCommand(formatCmd)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "path to config file")
	rootCmd.PersistentFlags().StringVar(&sourceDir, "source", "book", "source MDX directory")
	rootCmd.PersistentFlags().StringVar(&chromePath, "chrome-path", os.Getenv("PRETTY_CONVERTER_CHROME_PATH"),
		"path to a Chrome/Chromium executable (skips auto-detection/download)")
	rootCmd.PersistentFlags().StringVar(&calibrePath, "calibre-path", os.Getenv("PRETTY_CONVERTER_CALIBRE_PATH"),
		"path to Calibre's ebook-convert executable (skips PATH auto-detection), used by --format kindle / the kindle command")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-error output")

	buildCmd.Flags().StringVar(&outPath, "out", "out.pdf", "output path (base name for multi-format, or single file path)")
	buildCmd.Flags().StringVar(&formatStr, "format", "pdf", "output format(s), comma-separated: pdf, epub, kindle (e.g. pdf,epub,kindle)")
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
	buildCmd.Flags().BoolVar(&noOutline, "no-outline", false, "skip PDF bookmarks/outline (faster on very large documents)")
	buildCmd.Flags().BoolVar(&noTagged, "no-tagged-pdf", false, "skip PDF accessibility tagging (faster on very large documents)")
	buildCmd.Flags().BoolVar(&fastMode, "fast", false, "shorthand for --no-header --no-page-numbers --no-outline --no-tagged-pdf (biggest speedup on very large documents); an explicit flag among these still wins")
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

	analyzeCmd.Flags().BoolVar(&strict, "strict", false, "treat warnings as blocking (in addition to errors)")
	analyzeCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	analyzeCmd.Flags().IntVar(&analyzeMaxTableColumns, "max-table-columns", 0, "flag tables wider than this many columns (default 6)")
	analyzeCmd.Flags().IntVar(&analyzeMaxCodeLineLength, "max-code-line-length", 0, "flag code lines longer than this many characters (default 100)")
	analyzeCmd.Flags().IntVar(&analyzeMaxListDepth, "max-list-depth", 0, "flag lists nested deeper than this (default 3)")
	analyzeCmd.Flags().IntVar(&analyzeLongChapterWords, "long-chapter-words", 0, "flag chapters at/above this word count with no subheadings (default 3000)")

	initCmd.Flags().BoolVar(&initBare, "bare", false, "non-interactive init with flags")
	initCmd.Flags().StringVar(&title, "title", "My Book", "book title (for --bare)")
	initCmd.Flags().StringVar(&author, "author", "go-pretty-converter", "book author (for --bare)")
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

	kindleCmd.Flags().StringVar(&kindleOutPath, "out", "out.mobi", "output Kindle ebook path (.mobi or .azw3)")
	kindleCmd.Flags().StringVar(&title, "title", "", "book title")
	kindleCmd.Flags().StringVar(&subtitle, "subtitle", "", "book subtitle")
	kindleCmd.Flags().StringVar(&author, "author", "", "book author")
	kindleCmd.Flags().StringVar(&coverImage, "cover-image", "", "custom cover image (.png/.jpg/.jpeg/.svg/.webp), full-bleed as the first page")
	kindleCmd.Flags().StringVar(&kindleLanguage, "language", "en", "book language (BCP-47 tag, e.g. en, es)")
	kindleCmd.Flags().BoolVar(&kindleNoVerify, "no-verify", false, "skip the post-conversion integrity check (extracts the file's text with Calibre and scans for leaked markup and missing chapters)")

	kindleCmd.Flags().StringVar(&themeName, "theme", defaultTheme, fmt.Sprintf("book theme (%s, or a custom theme name/path)", strings.Join(themeNames(), ", ")))
	kindleCmd.Flags().StringVar(&cssPath, "css", "", "custom CSS file path (overrides theme)")
	kindleCmd.Flags().StringVar(&colorPrimary, "color-primary", "", "theme override: primary color (e.g. #1a56db)")
	kindleCmd.Flags().StringVar(&colorAccent, "color-accent", "", "theme override: accent color")
	kindleCmd.Flags().StringVar(&colorText, "color-text", "", "theme override: body text color")
	kindleCmd.Flags().StringVar(&colorMuted, "color-muted", "", "theme override: muted/caption text color")
	kindleCmd.Flags().StringVar(&colorBg, "color-bg", "", "theme override: page background color")
	kindleCmd.Flags().StringVar(&fontHeading, "font-heading", "", "theme override: heading font family")
	kindleCmd.Flags().StringVar(&fontBody, "font-body", "", "theme override: body font family")
	kindleCmd.Flags().StringVar(&fontCode, "font-code", "", "theme override: code font family")
	kindleCmd.Flags().StringVar(&density, "density", "", "spacing density: compact, normal, or relaxed")
	kindleCmd.Flags().BoolVar(&allowNetworkFonts, "allow-network-fonts", false, "allow fetching Google Fonts declared by the theme (enables network access)")

	formatCmd.Flags().StringVar(&formatOutPath, "out", "formatted", "output directory for the generated .mdx files + go-pretty-converter.yml")
	formatCmd.Flags().BoolVar(&formatForce, "force", false, "overwrite an existing, non-empty --out directory")
	formatCmd.Flags().StringVar(&title, "title", "", "book title (for the scaffolded go-pretty-converter.yml)")
	formatCmd.Flags().StringVar(&author, "author", "", "book author (for the scaffolded go-pretty-converter.yml)")
	formatCmd.Flags().BoolVar(&formatNoVerify, "no-verify", false, "skip re-parsing/analyzing the generated output")
	formatCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

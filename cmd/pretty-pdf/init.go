package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/sazardev/go-pretty-pdf/cmd/pretty-pdf/output"
)

func runInit(cmd *cobra.Command, args []string) error {
	if noColor {
		output.NoColor()
	}

	targetDir := "."
	if len(args) > 0 {
		targetDir = args[0]
	}

	if initBare {
		t, a, th := initValues(cmd)
		return runInitBare(targetDir, t, a, th, sourceDir, jsonOutput)
	}

	if jsonOutput {
		t, a, th := initValues(cmd)
		return runInitBare(targetDir, t, a, th, sourceDir, true)
	}

	var (
		bookTitle   string
		authorName  string
		themeChoice string
		srcDir      string
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(output.HeadingStyle.Render("Book Title")).
				Description(output.MutedStyle.Render("The main title of your book")).
				Placeholder("My Book").
				Value(&bookTitle),

			huh.NewInput().
				Title(output.HeadingStyle.Render("Author")).
				Description(output.MutedStyle.Render("The author's name")).
				Placeholder("go-pretty-pdf").
				Value(&authorName),

			huh.NewSelect[string]().
				Title(output.HeadingStyle.Render("Theme")).
				Description(output.MutedStyle.Render("Visual theme for your PDF")).
				Options(
					huh.NewOption("Default — clean, professional look", defaultTheme),
					huh.NewOption("Minimal — stripped down, no extras", "minimal"),
				).
				Value(&themeChoice),

			huh.NewInput().
				Title(output.HeadingStyle.Render("Source Directory")).
				Description(output.MutedStyle.Render("Where your MDX files will live")).
				Placeholder("book").
				Value(&srcDir),

			huh.NewConfirm().
				Title(output.HeadingStyle.Render("Create Project?")).
				Description(output.MutedStyle.Render(fmt.Sprintf("Will create %s with go-pretty-pdf.yml", targetDir))).
				Affirmative("Create!").
				Negative("Cancel"),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return fmt.Errorf("form canceled: %w", err)
	}

	if bookTitle == "" {
		bookTitle = "My Book"
	}
	if authorName == "" {
		authorName = "go-pretty-pdf"
	}
	if themeChoice == "" {
		themeChoice = defaultTheme
	}
	if srcDir == "" {
		srcDir = "book"
	}

	if !quiet {
		fmt.Println()
		spinner := output.StartSpinner("Scaffolding project...")
		if err := scaffoldWithConfig(targetDir, bookTitle, authorName, themeChoice, srcDir); err != nil {
			spinner.Fail(err.Error())
			return err
		}
		spinner.Done("Project scaffolded!")
		fmt.Println()
	} else {
		if err := scaffoldWithConfig(targetDir, bookTitle, authorName, themeChoice, srcDir); err != nil {
			return err
		}
	}

	absTarget, _ := filepath.Abs(targetDir)
	fmt.Println(output.Success(fmt.Sprintf("Project created at %s", absTarget)))
	fmt.Println("  " + output.MutedStyle.Render("Run:") +
		" " + output.CodeStyle.Render(fmt.Sprintf("cd %s && pretty-pdf build", targetDir)))

	return nil
}

func runInitBare(targetDir, bookTitle, authorName, themeChoice, srcDir string, json bool) error {
	if err := scaffoldWithConfig(targetDir, bookTitle, authorName, themeChoice, srcDir); err != nil {
		return err
	}

	if json {
		fmt.Printf(`{"directory":"%s","book_title":"%s","author":"%s","theme":"%s","source":"%s"}`+"\n",
			targetDir, bookTitle, authorName, themeChoice, srcDir)
	} else {
		absTarget, _ := filepath.Abs(targetDir)
		fmt.Println(output.Success(fmt.Sprintf("Project created at %s", absTarget)))
		fmt.Println("  " + output.MutedStyle.Render("Run:") +
			" " + output.CodeStyle.Render(fmt.Sprintf("cd %s && pretty-pdf build", targetDir)))
	}
	return nil
}

// initValues resolves the default title/author/theme for `init`'s bare/JSON
// modes. --title/--author/--theme are registered on build and epub too, all
// binding to the same package globals — and pflag leaves the *last* flag
// definition's default as the variable's initial value — so `init --bare`
// without those flags would otherwise write an empty title/author into the
// generated config instead of the documented "My Book"/"go-pretty-pdf"
// defaults. cmd.Flags().Changed() distinguishes an explicit flag from the
// polluted global.
func initValues(cmd *cobra.Command) (t, a, th string) {
	// Package globals are named title/author/themeName; don't shadow them.
	t, a, th = title, author, themeName
	if !cmd.Flags().Changed("title") {
		t = "My Book"
	}
	if !cmd.Flags().Changed("author") {
		a = "go-pretty-pdf"
	}
	if !cmd.Flags().Changed("theme") {
		th = defaultTheme
	}
	return t, a, th
}

func scaffoldWithConfig(targetDir, bookTitle, authorName, themeChoice, sourceDir string) error {
	bookDir := filepath.Join(targetDir, sourceDir)
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		return fmt.Errorf("creating book directory: %w", err)
	}

	replacer := strings.NewReplacer(
		"{{BOOK_TITLE}}", bookTitle,
		"{{AUTHOR_NAME}}", authorName,
		"{{THEME}}", themeChoice,
		"{{SOURCE_DIR}}", sourceDir,
	)

	assets := []string{
		"initassets/[1.0.0]-introduction.mdx",
		"initassets/[1.1.0]-getting-started.mdx",
		"initassets/[1.1.1]-installation.mdx",
	}
	for _, asset := range assets {
		data, err := initAssets.ReadFile(asset)
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", asset, err)
		}
		content := replacer.Replace(string(data))
		destPath := filepath.Join(bookDir, filepath.Base(asset))
		if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", destPath, err)
		}
	}

	configData, err := initAssets.ReadFile("initassets/go-pretty-pdf.yml")
	if err != nil {
		return fmt.Errorf("reading embedded config: %w", err)
	}
	configContent := replacer.Replace(string(configData))
	configPath := filepath.Join(targetDir, "go-pretty-pdf.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

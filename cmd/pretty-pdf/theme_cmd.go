package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sazardev/go-pretty-pdf/cmd/pretty-pdf/output"
	"github.com/sazardev/go-pretty-pdf/theme"
)

var (
	themeFrom   string
	themeGlobal bool
	themeAs     string
)

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "List, inspect, and manage PDF themes",
	Long: fmt.Sprintf(`Manage go-pretty-pdf's built-in and custom themes: list what's available,
inspect resolved CSS, scaffold new custom themes, and import existing ones.

  %d built-in themes: %s

Themes are also customizable without writing CSS via 'pretty-pdf build' flags
(--color-*, --font-*, --density, --no-cover/--no-toc/--no-page-numbers/--no-header)
or the 'theme_options' block in go-pretty-pdf.yml.`,
		len(theme.List()), strings.Join(themeNames(), ", ")),
	Example: `  pretty-pdf theme list
  pretty-pdf theme show corporate
  pretty-pdf theme new my-report --from corporate
  pretty-pdf theme add ./some-theme.theme.yml`,
}

var themeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List built-in and custom themes",
	Long: `Print every built-in theme with its description, followed by any custom
themes discovered in ./themes (project-local) and the global themes
directory (~/.config/pretty-pdf/themes on Linux).`,
	RunE: runThemeList,
}

var themeShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Print the final resolved CSS for a theme",
	Long: `Resolve a theme by name — built-in, a custom theme discovered on disk, or a
direct path to a .theme.yml/.css file — with no customization applied, and
print its fully-assembled CSS to stdout.`,
	Example: `  pretty-pdf theme show dark
  pretty-pdf theme show my-report > my-report.css`,
	Args: cobra.ExactArgs(1),
	RunE: runThemeShow,
}

var themeNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Scaffold a new custom theme",
	Long: `Write a starter <name>.theme.yml you can hand-edit: colors, fonts, section
toggles, density, and a raw CSS escape hatch. Refuses to overwrite an
existing file.`,
	Example: `  pretty-pdf theme new my-report --from corporate
  pretty-pdf theme new my-report --from classic --global`,
	Args: cobra.ExactArgs(1),
	RunE: runThemeNew,
}

var themeAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Import an existing theme file (.theme.yml or .css) as a custom theme",
	Long: `Copy an existing .theme.yml file into the managed themes directory as-is, or
wrap a loose .css file into a minimal .theme.yml (extends: default, with the
file's content as its css: block).`,
	Example: `  pretty-pdf theme add ./some-theme.theme.yml
  pretty-pdf theme add ./brand.css --as my-report
  pretty-pdf theme add ./some-theme.theme.yml --global`,
	Args: cobra.ExactArgs(1),
	RunE: runThemeAdd,
}

func init() {
	themeCmd.AddCommand(themeListCmd)
	themeCmd.AddCommand(themeShowCmd)
	themeCmd.AddCommand(themeNewCmd)
	themeCmd.AddCommand(themeAddCmd)

	themeNewCmd.Flags().StringVar(&themeFrom, "from", "default", "builtin theme to base the scaffold on")
	themeNewCmd.Flags().BoolVar(&themeGlobal, "global", false, "write to the global themes directory instead of ./themes")

	themeAddCmd.Flags().StringVar(&themeAs, "as", "", "name to register the imported theme under (default: derived from the file name)")
	themeAddCmd.Flags().BoolVar(&themeGlobal, "global", false, "copy to the global themes directory instead of ./themes")
}

func runThemeList(cmd *cobra.Command, args []string) error {
	if noColor {
		output.NoColor()
	}

	fmt.Println()
	fmt.Println("  " + output.Heading("Built-in themes"))
	fmt.Println()
	for _, t := range theme.List() {
		fmt.Printf("  %s %s\n", padRight(t.Name, 12), t.Description)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	customThemes, err := theme.ListCustom(cwd)
	if err != nil {
		return fmt.Errorf("listing custom themes: %w", err)
	}

	fmt.Println()
	fmt.Println("  " + output.Heading("Custom themes"))
	fmt.Println()
	if len(customThemes) == 0 {
		fmt.Println("  " + output.MutedStyle.Render("none found — create one with `pretty-pdf theme new <name>`"))
	} else {
		for _, c := range customThemes {
			scope := "project"
			if c.Global {
				scope = "global"
			}
			fmt.Printf("  %s %s %s\n", padRight(c.Name, 12), output.MutedStyle.Render(padRight("("+scope+")", 11)), c.Path)
		}
	}
	fmt.Println()
	return nil
}

func runThemeShow(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	css, _, err := theme.ResolveByName(args[0], theme.Options{}, cwd)
	if err != nil {
		return err
	}
	fmt.Println(css)
	return nil
}

func runThemeNew(cmd *cobra.Command, args []string) error {
	name := args[0]

	base, ok := theme.Get(themeFrom)
	if !ok {
		return fmt.Errorf("unknown base theme %q (see `pretty-pdf theme list`)", themeFrom)
	}

	dir, err := themeTargetDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, name+theme.ThemeFileSuffix)

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("theme file already exists: %s", path)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating themes directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(theme.ScaffoldYAML(name, base)), 0644); err != nil {
		return fmt.Errorf("writing theme file: %w", err)
	}

	fmt.Println(output.Success(fmt.Sprintf("Created %s (extends %q)", path, themeFrom)))
	fmt.Println("  " + output.MutedStyle.Render(fmt.Sprintf("Edit it, then build with --theme %s", name)))
	return nil
}

func runThemeAdd(cmd *cobra.Command, args []string) error {
	srcPath := args[0]
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", srcPath, err)
	}

	name := themeAs
	base := filepath.Base(srcPath)
	if name == "" {
		switch {
		case strings.HasSuffix(base, theme.ThemeFileSuffix):
			name = strings.TrimSuffix(base, theme.ThemeFileSuffix)
		default:
			name = strings.TrimSuffix(base, filepath.Ext(base))
		}
	}
	if name == "" {
		return fmt.Errorf("could not derive a theme name from %s — pass --as <name>", srcPath)
	}

	var content string
	if strings.HasSuffix(base, ".theme.yml") || strings.HasSuffix(base, ".theme.yaml") {
		content = string(data)
	} else {
		content = fmt.Sprintf("name: %s\ndescription: \"Imported from %s\"\nextends: default\ncss: |\n%s\n",
			name, base, indentBlock(string(data), "  "))
	}

	dir, err := themeTargetDir()
	if err != nil {
		return err
	}
	destPath := filepath.Join(dir, name+theme.ThemeFileSuffix)

	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("theme file already exists: %s", destPath)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating themes directory: %w", err)
	}
	if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing theme file: %w", err)
	}

	fmt.Println(output.Success(fmt.Sprintf("Imported %s as %q -> %s", srcPath, name, destPath)))
	return nil
}

func themeTargetDir() (string, error) {
	if themeGlobal {
		return theme.UserThemesDir()
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	return theme.ProjectThemesDir(cwd), nil
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s + " "
	}
	return s + strings.Repeat(" ", n-len(s))
}

func indentBlock(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

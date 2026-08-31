package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ThemeFileSuffix is the required suffix for custom theme files.
const ThemeFileSuffix = ".theme.yml"

// UserThemesDir returns the global, cross-project directory custom themes
// can be installed into (e.g. ~/.config/pretty-converter/themes on Linux).
func UserThemesDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving user config dir: %w", err)
	}
	return filepath.Join(dir, "pretty-converter", "themes"), nil
}

// ProjectThemesDir returns the project-local themes directory, searched
// before the global one.
func ProjectThemesDir(cwd string) string {
	return filepath.Join(cwd, "themes")
}

// FindCustom looks for a "<name>.theme.yml" file, checking the project
// directory first and then the global one.
func FindCustom(cwd, name string) (string, bool) {
	candidates := []string{filepath.Join(ProjectThemesDir(cwd), name+ThemeFileSuffix)}
	if userDir, err := UserThemesDir(); err == nil {
		candidates = append(candidates, filepath.Join(userDir, name+ThemeFileSuffix))
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, true
		}
	}
	return "", false
}

// CustomThemeInfo describes a discovered custom theme file.
type CustomThemeInfo struct {
	Name   string
	Path   string
	Global bool
}

// ListCustom enumerates every custom theme visible from cwd: project-local
// themes first, then global ones (a project theme shadows a global theme
// of the same name).
func ListCustom(cwd string) ([]CustomThemeInfo, error) {
	dirs := []struct {
		path   string
		global bool
	}{
		{ProjectThemesDir(cwd), false},
	}
	if userDir, err := UserThemesDir(); err == nil {
		dirs = append(dirs, struct {
			path   string
			global bool
		}{userDir, true})
	}

	seen := make(map[string]bool)
	var out []CustomThemeInfo
	for _, d := range dirs {
		entries, err := os.ReadDir(d.path)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ThemeFileSuffix) {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ThemeFileSuffix)
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, CustomThemeInfo{
				Name:   name,
				Path:   filepath.Join(d.path, e.Name()),
				Global: d.global,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ScaffoldYAML returns a starter .theme.yml file for `theme new`, extending
// the given base theme with every field left blank (so it inherits the
// base's own values) and commented hints for what to fill in.
func ScaffoldYAML(name string, from Theme) string {
	base := from.Name
	if base == "" {
		base = NameDefault
	}
	return fmt.Sprintf(`# Custom go-pretty-converter theme.
# Generated with: pretty-converter theme new %s --from %s
name: %s
description: "My custom theme"
extends: %s

colors:
  primary: ""      # e.g. "#1a56db" - leave empty to inherit from extends
  accent: ""
  text: ""
  muted: ""
  background: ""

fonts:
  heading: ""        # e.g. "Georgia, serif"
  body: ""
  code: ""
  google_fonts: []    # only used with --allow-network-fonts, e.g. ["Inter:400,600"]

sections:
  cover: true
  toc: true
  page_numbers: true
  header: true

density: normal        # compact | normal | relaxed

# Raw CSS appended last — wins over everything above.
css: |
  /* .cover h1 { text-transform: uppercase; } */
`, name, base, name, base)
}

package theme

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// CustomTheme is the schema for a user-defined `<name>.theme.yml` file. It
// extends a builtin theme by name and overrides its colors, fonts, section
// toggles, and density, with an escape hatch for raw CSS appended last.
type CustomTheme struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Extends     string   `yaml:"extends"`
	Colors      Colors   `yaml:"colors"`
	Fonts       Fonts    `yaml:"fonts"`
	Sections    Sections `yaml:"sections"`
	Density     Density  `yaml:"density"`
	CSS         string   `yaml:"css"`
}

// LoadCustomTheme reads and parses a .theme.yml file.
func LoadCustomTheme(path string) (*CustomTheme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading theme file %s: %w", path, err)
	}
	var ct CustomTheme
	if err := yaml.Unmarshal(data, &ct); err != nil {
		return nil, fmt.Errorf("parsing theme file %s: %w", path, err)
	}
	return &ct, nil
}

// Resolve builds the final CSS for a custom theme. The theme's own YAML
// fields (colors, fonts, sections, density) act as defaults; opts (usually
// CLI flags or go-pretty-converter.yml's theme_options) take priority whenever a
// field is explicitly set. The theme's raw css field, if any, is appended
// last so it always wins.
func (c *CustomTheme) Resolve(opts Options) (string, ResolvedSections, error) {
	extends := c.Extends
	if extends == "" {
		extends = NameDefault
	}
	base, ok := Get(extends)
	if !ok {
		return "", ResolvedSections{}, fmt.Errorf("custom theme %q extends unknown base theme %q", c.Name, extends)
	}

	merged := Options{
		Colors:            mergeColors(c.Colors, opts.Colors),
		Fonts:             mergeFonts(c.Fonts, opts.Fonts),
		Sections:          mergeSectionOverrides(c.Sections, opts.Sections),
		Density:           opts.Density,
		AllowNetworkFonts: opts.AllowNetworkFonts,
	}
	if merged.Density == "" {
		merged.Density = c.Density
	}

	css, sections, err := Resolve(base, merged)
	if err != nil {
		return "", ResolvedSections{}, err
	}
	if c.CSS != "" {
		css += "\n" + c.CSS
	}
	return css, sections, nil
}

// ResolveForEPUB builds the final EPUB CSS for a custom theme. Same merge
// semantics as Resolve, but uses the EPUB structural skeleton instead of
// the PDF one and does not return ResolvedSections.
func (c *CustomTheme) ResolveForEPUB(opts Options) (string, error) {
	extends := c.Extends
	if extends == "" {
		extends = NameDefault
	}
	base, ok := Get(extends)
	if !ok {
		return "", fmt.Errorf("custom theme %q extends unknown base theme %q", c.Name, extends)
	}

	merged := Options{
		Colors:            mergeColors(c.Colors, opts.Colors),
		Fonts:             mergeFonts(c.Fonts, opts.Fonts),
		Sections:          mergeSectionOverrides(c.Sections, opts.Sections),
		Density:           opts.Density,
		AllowNetworkFonts: opts.AllowNetworkFonts,
	}
	if merged.Density == "" {
		merged.Density = c.Density
	}

	css, err := ResolveForEPUB(base, merged)
	if err != nil {
		return "", err
	}
	if c.CSS != "" {
		css += "\n" + c.CSS
	}
	return css, nil
}

func mergeColors(base, override Colors) Colors {
	return Colors{
		Primary:    firstNonEmpty(override.Primary, base.Primary),
		Accent:     firstNonEmpty(override.Accent, base.Accent),
		Text:       firstNonEmpty(override.Text, base.Text),
		Muted:      firstNonEmpty(override.Muted, base.Muted),
		Background: firstNonEmpty(override.Background, base.Background),
	}
}

func mergeFonts(base, override Fonts) Fonts {
	imports := override.GoogleImports
	if len(imports) == 0 {
		imports = base.GoogleImports
	}
	return Fonts{
		Heading:       firstNonEmpty(override.Heading, base.Heading),
		Body:          firstNonEmpty(override.Body, base.Body),
		Code:          firstNonEmpty(override.Code, base.Code),
		GoogleImports: imports,
	}
}

func mergeSectionOverrides(base, override Sections) Sections {
	return Sections{
		Cover:       firstNonNilBool(override.Cover, base.Cover),
		TOC:         firstNonNilBool(override.TOC, base.TOC),
		PageNumbers: firstNonNilBool(override.PageNumbers, base.PageNumbers),
		Header:      firstNonNilBool(override.Header, base.Header),
	}
}

func firstNonNilBool(vals ...*bool) *bool {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

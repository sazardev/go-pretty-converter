package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigFile = "go-pretty-converter.yml"
	defaultSource     = "book"
	defaultOutput     = "out.pdf"
)

type Config struct {
	Title        string             `yaml:"title"`
	Subtitle     string             `yaml:"subtitle"`
	Author       string             `yaml:"author"`
	Source       string             `yaml:"source"`
	Output       string             `yaml:"output"`
	Theme        string             `yaml:"theme"`
	CSS          string             `yaml:"css"`
	Template     string             `yaml:"template"`
	Vars         map[string]string  `yaml:"vars"`
	Lint         LintConfig         `yaml:"lint"`
	Render       RenderConfig       `yaml:"render"`
	ThemeOptions ThemeOptionsConfig `yaml:"theme_options"`
}

// ThemeOptionsConfig customizes the theme selected via Theme: colors,
// fonts, section toggles (cover/TOC/page numbers/header), density, and
// whether network-fetched Google Fonts are allowed.
type ThemeOptionsConfig struct {
	Colors            ColorsConfig   `yaml:"colors"`
	Fonts             FontsConfig    `yaml:"fonts"`
	Sections          SectionsConfig `yaml:"sections"`
	Density           string         `yaml:"density"`
	AllowNetworkFonts bool           `yaml:"allow_network_fonts"`
}

type ColorsConfig struct {
	Primary    string `yaml:"primary"`
	Accent     string `yaml:"accent"`
	Text       string `yaml:"text"`
	Muted      string `yaml:"muted"`
	Background string `yaml:"background"`
}

type FontsConfig struct {
	Heading     string   `yaml:"heading"`
	Body        string   `yaml:"body"`
	Code        string   `yaml:"code"`
	GoogleFonts []string `yaml:"google_fonts"`
}

type SectionsConfig struct {
	Cover       *bool `yaml:"cover"`
	TOC         *bool `yaml:"toc"`
	PageNumbers *bool `yaml:"page_numbers"`
	Header      *bool `yaml:"header"`
}

type LintConfig struct {
	RequireFrontmatter []string `yaml:"require_frontmatter"`
	NoDuplicateIDs     bool     `yaml:"no_duplicate_ids"`
	MaxHeadingDepth    int      `yaml:"max_heading_depth"`
}

type RenderConfig struct {
	Timeout     string `yaml:"timeout"`
	Paper       string `yaml:"paper"`
	MarginTop   string `yaml:"margin_top"`
	MarginBot   string `yaml:"margin_bottom"`
	MarginLeft  string `yaml:"margin_left"`
	MarginRight string `yaml:"margin_right"`
	HeaderTitle string `yaml:"header_title"`
	// CoverImage, when set, replaces the theme's text cover with a
	// full-bleed page built from this image (.png/.jpg/.jpeg). That page
	// is sized to the image's own pixel dimensions exactly — a square
	// image gets a square cover page — while every other page keeps the
	// configured Paper size untouched.
	CoverImage string `yaml:"cover_image"`
}

func Default() *Config {
	return &Config{
		Source: defaultSource,
		Output: defaultOutput,
		Title:  "Document",
		Author: "go-pretty-converter",
		Lint: LintConfig{
			RequireFrontmatter: []string{"id", "title"},
			NoDuplicateIDs:     true,
			MaxHeadingDepth:    5,
		},
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return cfg, nil
}

func FindConfig() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(dir, DefaultConfigFile)
		if _, err := os.Stat(path); err == nil {
			return filepath.Abs(path)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", nil
}

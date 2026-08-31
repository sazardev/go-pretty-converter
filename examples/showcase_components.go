package main

import (
	"fmt"
	"strconv"
	"strings"

	prettyconverter "github.com/sazardev/go-pretty-converter"
	"github.com/sazardev/go-pretty-converter/mdx"
)

// The showcase demonstrates that a component is nothing more than a Go
// function: attrs (title only — see the component page in the book) and
// inner content in, an HTML string out. Colors match compose/assets/print.css
// so custom components sit visually next to the built-in ones.

func calloutHandler(attrs map[string]string, inner string) string {
	level := strings.ToLower(strings.TrimSpace(attrs["title"]))
	color, bg, icon := "#4a6cf7", "#f0f4ff", "i"
	switch level {
	case "success":
		color, bg, icon = "#4caf50", "#f0faf0", "OK"
	case "warning":
		color, bg, icon = "#f7a84a", "#fff8e6", "!"
	case "danger":
		color, bg, icon = "#e5484d", "#fff0f0", "x"
	default:
		level = "info"
	}
	return fmt.Sprintf(`<div style="background:%s;border-left:4px solid %s;padding:12px 16px;margin:16px 0;border-radius:0 4px 4px 0;">`+
		`<strong style="color:%s;text-transform:uppercase;font-size:9pt;letter-spacing:.5px;">%s CALLOUT: %s</strong>`+
		`<div style="margin-top:6px;font-size:10pt;">%s</div></div>`,
		bg, color, color, icon, strings.ToUpper(level), strings.TrimSpace(inner))
}

func badgeHandler(attrs map[string]string, inner string) string {
	variant := strings.ToLower(strings.TrimSpace(attrs["title"]))
	color, bg := "#4a6cf7", "#eef1ff"
	switch variant {
	case "stable":
		color, bg = "#4caf50", "#eaf7ea"
	case "deprecated":
		color, bg = "#e5484d", "#fdecec"
	case "beta":
		color, bg = "#f7a84a", "#fff3e0"
	}
	return fmt.Sprintf(`<span style="display:inline-block;background:%s;color:%s;font-size:8pt;font-weight:700;`+
		`letter-spacing:.5px;padding:2px 8px;border-radius:10px;text-transform:uppercase;margin:0 4px 4px 0;">%s</span>`,
		bg, color, strings.TrimSpace(inner))
}

func stepsHandler(_ map[string]string, inner string) string {
	var b strings.Builder
	b.WriteString(`<div style="margin:16px 0;">`)
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(inner), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		n++
		fmt.Fprintf(&b, `<div style="display:flex;gap:10px;margin-bottom:8px;align-items:flex-start;">`+
			`<span style="flex:0 0 auto;width:20px;height:20px;border-radius:50%%;background:#4a6cf7;`+
			`color:#fff;font-size:9pt;font-weight:700;text-align:center;line-height:20px;">%d</span>`+
			`<span style="font-size:10pt;padding-top:1px;">%s</span></div>`, n, line)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func cardHandler(attrs map[string]string, inner string) string {
	title := strings.TrimSpace(attrs["title"])
	return fmt.Sprintf(`<div style="border:1px solid #ddd;border-radius:6px;overflow:hidden;margin:16px 0;">`+
		`<div style="background:#1a1a1a;color:#fff;padding:8px 14px;font-weight:600;font-size:10pt;">%s</div>`+
		`<div style="padding:12px 14px;font-size:10pt;background:#fafafa;">%s</div></div>`,
		title, strings.TrimSpace(inner))
}

func statHandler(attrs map[string]string, inner string) string {
	label := strings.TrimSpace(attrs["title"])
	return fmt.Sprintf(`<div style="display:inline-block;width:30%%;margin:0 2%% 12px 0;text-align:center;vertical-align:top;">`+
		`<div style="font-size:20pt;font-weight:700;color:#4a6cf7;">%s</div>`+
		`<div style="font-size:8.5pt;color:#666;text-transform:uppercase;letter-spacing:.5px;">%s</div></div>`,
		strings.TrimSpace(inner), label)
}

func timelineHandler(attrs map[string]string, inner string) string {
	date := strings.TrimSpace(attrs["title"])
	return fmt.Sprintf(`<div style="border-left:2px solid #4a6cf7;padding-left:14px;margin:0 0 14px 6px;position:relative;">`+
		`<div style="position:absolute;left:-6px;top:2px;width:10px;height:10px;border-radius:50%%;background:#4a6cf7;"></div>`+
		`<div style="font-weight:700;font-size:9.5pt;color:#1a1a1a;">%s</div>`+
		`<div style="font-size:10pt;color:#444;margin-top:2px;">%s</div></div>`,
		date, strings.TrimSpace(inner))
}

func quoteHandler(attrs map[string]string, inner string) string {
	author := strings.TrimSpace(attrs["title"])
	return fmt.Sprintf(`<div style="margin:16px 0;padding:14px 18px;background:#f7f7f9;border-radius:6px;`+
		`font-style:italic;font-size:11pt;color:#333;">&ldquo;%s&rdquo;`+
		`<div style="margin-top:8px;font-style:normal;font-size:9pt;font-weight:600;color:#666;">— %s</div></div>`,
		strings.TrimSpace(inner), author)
}

func progressHandler(attrs map[string]string, inner string) string {
	pct, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(attrs["title"]), "%"))
	if err != nil || pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return fmt.Sprintf(`<div style="margin:12px 0;">`+
		`<div style="display:flex;justify-content:space-between;font-size:9pt;margin-bottom:4px;">`+
		`<span>%s</span><span style="font-weight:700;">%d%%</span></div>`+
		`<div style="background:#eee;border-radius:8px;height:10px;overflow:hidden;">`+
		`<div style="background:#4a6cf7;height:100%%;width:%d%%;"></div></div></div>`,
		strings.TrimSpace(inner), pct, pct)
}

// showcaseComponents registers all eight custom components used by the
// showcase book. They only take effect when the book is built through
// this Go code (WithComponent registers in memory) — a bare
// `pretty-converter build` from the CLI has no way to load them from YAML.
func showcaseComponents() []prettyconverter.Option {
	return []prettyconverter.Option{
		prettyconverter.WithComponent("Callout", calloutHandler),
		prettyconverter.WithComponent("Badge", badgeHandler),
		prettyconverter.WithComponent("Steps", stepsHandler),
		prettyconverter.WithComponent("Card", cardHandler),
		prettyconverter.WithComponent("Stat", statHandler),
		prettyconverter.WithComponent("Timeline", timelineHandler),
		prettyconverter.WithComponent("Quote", quoteHandler),
		prettyconverter.WithComponent("Progress", progressHandler),
	}
}

// showcaseOptions returns the full option set for building the showcase
// book: metadata, variable substitution, linting, and all custom
// components. Shared by both the compose-only test and the full-render
// test so they stay in sync.
func showcaseOptions() []prettyconverter.Option {
	opts := []prettyconverter.Option{
		prettyconverter.WithSourceDir("showcase"),
		prettyconverter.WithTitle("go-pretty-converter Showcase"),
		prettyconverter.WithSubtitle("Every feature, one book"),
		prettyconverter.WithAuthor("go-pretty-converter"),
		prettyconverter.WithHeaderTitle("go-pretty-converter v1.0 — Showcase"),
		prettyconverter.WithVars(map[string]string{
			"product": "go-pretty-converter",
			"version": "1.0",
		}),
		prettyconverter.WithValidator(showcaseValidator()),
	}
	return append(opts, showcaseComponents()...)
}

// showcaseValidator allows headings up to h5 since the typography page
// deliberately demonstrates every heading level the theme supports.
func showcaseValidator() *mdx.DefaultValidator {
	v := mdx.NewDefaultValidator()
	v.MaxHeadingDepth = 5
	return v
}

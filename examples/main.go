package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/sazardev/go-pretty-converter"
)

func main() {
	pdf, err := prettyconverter.New(
		prettyconverter.WithSourceDir("./examples/docs"),
		prettyconverter.WithOutputFile("./examples/output/example-output.pdf"),
		prettyconverter.WithTitle("go-pretty-converter — Complete Example"),
		prettyconverter.WithSubtitle("Every feature demonstrated with real MDX files"),
		prettyconverter.WithAuthor("go-pretty-converter Demo"),
		prettyconverter.WithComponent("Callout", func(attrs map[string]string, inner string) string {
			level := attrs["title"]
			if level == "" {
				level = "info"
			}
			return fmt.Sprintf(
				`<div class="callout callout-%s"><strong>%s:</strong> %s</div>`,
				level, strings.ToUpper(level), inner,
			)
		}),
		prettyconverter.WithComponent("Steps", func(attrs map[string]string, inner string) string {
			lines := strings.Split(strings.TrimSpace(inner), "\n")
			var items []string
			for i, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				items = append(items, fmt.Sprintf(
					`<div class="step-item"><span class="step-number">%d</span><span class="step-text">%s</span></div>`,
					i+1, line,
				))
			}
			return `<div class="steps-container">` + strings.Join(items, "\n") + `</div>`
		}),
		prettyconverter.WithComponent("Card", func(attrs map[string]string, inner string) string {
			title := attrs["title"]
			icon := attrs["icon"]
			return fmt.Sprintf(
				`<div class="custom-card"><div class="card-header">%s <strong>%s</strong></div><div class="card-body">%s</div></div>`,
				icon, title, inner,
			)
		}),
	)
	if err != nil {
		log.Fatalf("Error creating PDF: %v", err)
	}

	fmt.Println("Building PDF from examples/docs/...")
	if err := pdf.Build(context.Background()); err != nil {
		log.Fatalf("Error building PDF: %v", err)
	}
	fmt.Println("✅ PDF generated: examples/output/example-output.pdf")
}

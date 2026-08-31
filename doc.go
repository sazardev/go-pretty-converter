/*
Package prettyconverter transforms MDX source files into print-ready PDFs via headless Chrome.

It is both a composable Go library and a CLI tool. The library exposes a pipeline
with step-by-step methods (ParseDir, ValidateDoc, ComposeHTML, Render) and 18 functional
options for full customization.

# Quick start (library)

	pdf, err := prettyconverter.New(
		prettyconverter.WithSourceDir("./docs"),
		prettyconverter.WithOutputFile("output.pdf"),
		prettyconverter.WithTitle("My Book"),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := pdf.Build(context.Background()); err != nil {
		log.Fatal(err)
	}

# Quick start (CLI)

	go install github.com/sazardev/go-pretty-converter/cmd/pretty-converter@latest
	pretty-converter build --source ./docs --out out.pdf

MDX files require frontmatter with an `id` field in [X.Y.Z] format (e.g. `[1.0.0]`).
Documents are sorted by ID, not filename. Variables in {{key}} syntax are substituted
before parsing.

Built-in custom components: <DeepDive>, <Warning>, <Axiom>. Additional components
can be registered via WithComponent().

# Trust model

MDX is parsed with raw HTML passthrough enabled, and component transpilation
does not escape inner content. Only build PDFs from MDX you trust — see
SECURITY.md for details. Network access during rendering is blocked by
default (see WithNetworkAccess).
*/
package prettyconverter

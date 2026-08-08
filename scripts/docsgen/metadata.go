package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sazardev/go-pretty-pdf/theme"
	"github.com/sazardev/go-pretty-pdf/version"
)

// versionJSONName is a machine-readable build manifest consumed by
// tooling, release scripts, or monitoring that wants facts (version,
// build time, counts) without scraping HTML.
const versionJSONName = "version.json"

// siteMetadata is the shape of version.json.
type siteMetadata struct {
	Project    string   `json:"project"`
	Version    string   `json:"version"`
	License    string   `json:"license"`
	Homepage   string   `json:"homepage"`
	Repository string   `json:"repository"`
	GoPackage  string   `json:"go_package"`
	BuiltAt    string   `json:"built_at"`
	BuiltAtUTC string   `json:"built_at_utc"`
	Themes     int      `json:"themes"`
	ThemeNames []string `json:"theme_names"`
	DocsPages  int      `json:"docs_pages"`
	Formats    []string `json:"formats"`
	Language   string   `json:"language"`
	LastAudit  string   `json:"last_audit"`
}

// buildMetadata assembles version.json from live package data so it never
// drifts from the actual release.
func buildMetadata() siteMetadata {
	names := make([]string, 0, len(theme.List()))
	for _, t := range theme.List() {
		names = append(names, t.Name)
	}
	now := time.Now()
	return siteMetadata{
		Project:    siteName,
		Version:    version.Version,
		License:    "MIT",
		Homepage:   siteBaseURL,
		Repository: siteRepoURL,
		GoPackage:  "github.com/sazardev/go-pretty-pdf",
		BuiltAt:    now.Format("2006-01-02 15:04:05 MST"),
		BuiltAtUTC: now.UTC().Format(time.RFC3339),
		Themes:     len(names),
		ThemeNames: names,
		DocsPages:  2,
		Formats:    []string{groupPDF, groupEPUB},
		Language:   "en",
		LastAudit:  "PDF quality audit runs on every theme build; see docs.html#cli-reference",
	}
}

// writeMetadata writes version.json to outDir.
func writeMetadata(outDir string, log *buildLogger) {
	data, err := json.MarshalIndent(buildMetadata(), "", "  ")
	if err != nil {
		log.Warnf("could not marshal version.json: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join(outDir, versionJSONName), data, 0644); err != nil {
		log.Warnf("could not write version.json: %v", err)
		return
	}
	log.Vf("  ✓ %-24s %s", versionJSONName, formatBytes(len(data)))
}

// sitemapTXTName is a plain-text sitemap alternative to sitemap.xml for
// robots/agents that find XML awkward — a bare URL per line, matching the
// XML sitemap's URL set exactly.
const sitemapTXTName = "sitemap.txt"

// writeSitemapTXT writes the same URLs as sitemapXML() as plain lines.
func writeSitemapTXT(outDir string, log *buildLogger) {
	var b strings.Builder
	lines := make([]string, 0, 8+2*len(theme.List()))
	lines = append(lines,
		siteBaseURL,
		siteBaseURL+"docs.html",
		siteBaseURL+llmsFileName(),
		siteBaseURL+"llms-full.txt",
		siteBaseURL+"humans.txt",
		siteBaseURL+"docs.md",
		siteBaseURL+"docs-search.json",
		siteBaseURL+"report.json",
		siteBaseURL+docsPDFDefault,
		siteBaseURL+docsEPUBDefault,
	)
	for _, t := range theme.List() {
		lines = append(lines, siteBaseURL+docsPDFFilename(t.Name))
		lines = append(lines, siteBaseURL+docsEPUBFilename(t.Name))
	}
	for _, u := range lines {
		fmt.Fprintf(&b, "%s\n", u)
	}
	data := []byte(b.String())
	if err := os.WriteFile(filepath.Join(outDir, sitemapTXTName), data, 0644); err != nil {
		log.Warnf("could not write sitemap.txt: %v", err)
		return
	}
	log.Vf("  ✓ %-24s %s (%d URLs)", sitemapTXTName, formatBytes(len(data)), len(lines))
}

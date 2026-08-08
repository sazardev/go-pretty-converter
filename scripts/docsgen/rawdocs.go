package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// docsRawName is the single-file concatenated markdown of all docs sources,
// served as plain text so LLM agents and plain `curl` consumers can grab the
// full reference without HTML rendering or the compact llms.txt summary.
const docsRawName = "docs.md"

// buildRawDocs exports README.md, docs/cli.md, and CHANGELOG.md as plain
// markdown files under the site root. Unlike llms.txt (a dense summary for
// token-conscious agents), these are the exact source files, useful for
// mirroring, diffing, or feeding a model that wants the un-rendered text.
func buildRawDocs(outDir, root string, readme, cli, changelog []byte, log *buildLogger) {
	assets := []struct {
		name string
		data []byte
	}{
		{"README.md", readme},
		{docsRawName, concatDocsMarkdown(readme, cli, changelog)},
		{"CHANGELOG.md", changelog},
	}
	for _, a := range assets {
		if err := os.WriteFile(filepath.Join(outDir, a.name), a.data, 0644); err != nil {
			log.Warnf("could not write %s: %v", a.name, err)
			continue
		}
		log.Vf("  ✓ %-24s %s", a.name, formatBytes(len(a.data)))
	}
	_ = root
}

// concatDocsMarkdown merges the three docs sources into one logical
// reference file with clear section dividers, in reading order.
func concatDocsMarkdown(readme, cli, changelog []byte) []byte {
	var b strings.Builder
	b.WriteString("# go-pretty-pdf — Full Documentation (source)\n\n")
	b.WriteString("> Generated from README.md + docs/cli.md + CHANGELOG.md.\n\n")
	b.WriteString("<!-- README.md -->\n\n")
	b.Write(readme)
	b.WriteString("\n\n---\n\n<!-- docs/cli.md -->\n\n")
	b.Write(cli)
	b.WriteString("\n\n---\n\n<!-- CHANGELOG.md -->\n\n")
	b.Write(changelog)
	return []byte(b.String())
}

// searchIndexEntry is one row of docs-search.json: a documented section
// with its anchor and first sentences of plain text, enough for a
// client-side search box or an agent to locate a topic without parsing the
// whole page.
type searchIndexEntry struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Href  string `json:"href"`
	Text  string `json:"text"`
}

// buildSearchIndex writes docs-search.json from the already-composed
// sections. Text is stripped of HTML and truncated so the index stays
// lean; id/href mirror the on-page anchors so a search result can deep-link.
func buildSearchIndex(outDir string, sections []Section, log *buildLogger) {
	entries := make([]searchIndexEntry, 0, len(sections))
	for _, s := range sections {
		if s.ID == heroSectionID || s.Title == "" {
			continue
		}
		text := stripHTMLTags(s.Content)
		text = htmlEntityDecode(text)
		text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
		text = strings.TrimSpace(text)
		if len(text) > 240 {
			text = text[:240] + "…"
		}
		entries = append(entries, searchIndexEntry{
			ID:    s.ID,
			Title: htmlEntityDecode(stripHTMLTags(s.Title)),
			Href:  "docs.html#" + s.ID,
			Text:  text,
		})
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		log.Warnf("could not marshal search index: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join(outDir, "docs-search.json"), data, 0644); err != nil {
		log.Warnf("could not write docs-search.json: %v", err)
		return
	}
	log.Vf("  ✓ %-24s %s (%d entries)", "docs-search.json", formatBytes(len(data)), len(entries))
}

// htmlEntityDecode decodes the handful of HTML entities docsgen emits, so
// search text and raw files don't leak "&mdash;" where "—" belongs.
func htmlEntityDecode(s string) string {
	repl := strings.NewReplacer(
		"&mdash;", "—",
		"&ndash;", "–",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&hellip;", "…",
		"&middot;", "·",
	)
	return repl.Replace(s)
}

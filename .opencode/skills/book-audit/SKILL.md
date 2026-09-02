---
name: book-audit
description: Total audit of a go-pretty-pdf book (MDX source, no Chrome needed). Hunts content leaks, dead references, orphaned files, broken TOC/links, unregistered components, frontmatter errors, layout leak risks, and editorial incoherence. Use when user asks to "analizar el libro", "book audit", "fugas", "leaks", "revisa el libro", "integrity", "contenido huérfano", or wants to validate the whole docs/book before building the PDF.
---

# Book Audit

Source-level audit of the whole book (the MDX files under `book/` or `examples/docs/`). Needs no Chrome, no render — everything here is checked against the source and the parser/compose rules.

Grounding facts (verify against AGENTS.md / mdx/parser.go, don't assume):

- Documents are sorted by frontmatter `id` `[X.Y.Z]` (regex `^\[(\d+)\.(\d+)\.(\d+)\]$`), NOT by filename.
- Frontmatter is optional. If the `---` block is entirely absent, `id`/`title` are generated from the filename (`02-getting-started.mdx` → `[2.0.0]`, "Getting Started"). A `---` block present but with invalid YAML is a **hard error**.
- `.txt` files are accepted: no frontmatter, literal text, HTML-escaped.
- Built-in custom components: `<DeepDive>`, `<Warning>`, `<Axiom>`. Others need `WithComponent()`. An unregistered component does NOT fail the build — it renders as raw HTML/`<component>`, a silent content leak.
- Frontmatter extras: `subtitle`, `tags`, `difficulty`, `status`, `completeness` (0–100), `depends_on` (list of `[X.Y.Z]` ids).

## Steps

1. **Inventario.** List every file in the book dir: `.mdx` and `.txt`. Group by frontmatter-vs-generated id. Build the full id table.
2. **Caza de fugas de contenido** (the core — find every one, don't stop at the first):
   - Orphaned/leaked documents: file whose `id` never appears in the final ordered list (regex mismatch, duplicate id silently collapsed by sort).
   - Duplicate `[X.Y.Z]` ids across files (sort/pagination leaks: both render, TOC collides, cross-refs resolve ambiguously).
   - Malformed id: not matching `^\[X.Y.Z\]$` (letters, negatives, `1.2` two-part, spaces). Files with auto-generated ids mixed with explicit ones.
   - Missing/incomplete title: no title in frontmatter and filename unparseable into one.
   - TOC leaks: sections whose id is missing from the composed TOC, or TOC order that disagrees with id order.
   - Dead cross-references: links/anchors/`depends_on` pointing to an id that doesn't exist, or anchor targets (AnchorID) that don't match any rendered heading.
   - Unregistered components: `<DeepDive>`/`<Warning>`/`<Axiom>` are fine; any OTHER `<CustomX>` used without a matching registration = silent leak. Flag every occurrence.
   - Leaked code fences: unbalanced ``` in a file (the rest of the doc is swallowed into the fence).
   - `.txt` stray content and files that don't sort where intended.
3. **Frontmatter integrity.**
   - `---` block present but invalid YAML (hard error at parse) — flag even though it fails loudly.
   - `id` type wrong (string vs number), `completeness` not 0–100 numeric, `depends_on` with non-existent or self-referencing ids, `tags`/`difficulty` inconsistent across the book.
4. **Fugas de layout (estático, sin render).** Flag content that will clip/overflow in print:
   - Code lines / tables / images wider than the printable area (long unbroken tokens, wide inline tables, images with explicit px width).
   - Headings at the very end of a document (orphan risk on page break).
   - Very long unbreakable blocks (single paragraph, giant code fence) that can't split across pages.
   - Missing page-break hints where sections clearly should start fresh.
5. **Coherencia editorial ("etc.").**
   - `status: Draft`, `completeness: 0`/low — leaked unfinished sections shipping in the book.
   - Placeholders: TODO, TBD, lorem ipsum, "coming soon", empty sections, stub text.
   - Duplicate titles across documents; wrong sort that produces non-monotonic chapter progression.
   - `subtitle`/`difficulty`/`tags` conventions broken for some docs and not others.

## Report format

```markdown
# Book audit: <dir>
**Archivos:** n (n mdx + n txt) · ids explícitos: n · generados: n

## Fugas de contenido 🔴 (cada una con file:line)
## Frontmatter 🟡
## Layout risk 🔵
## Coherencia 🔵/❓
## Veredicto
n fugas 🔴 · n frontmatter 🟡 · n layout 🔵 — <el libro está listo para build | hay fugas que corregir antes de compilar>
```

## Rules

- Read the actual files; don't infer from filenames alone.
- Every finding cites `file:line` and quotes the exact id/tag involved.
- Severity: 🔴 real leak (content missing/duplicated/unreachable) · 🟡 fragile (frontmatter, sort edge) · 🔵 polish.
- Distinguish "hard error" (build fails, e.g. invalid YAML) from "silent leak" (build succeeds, content lost) — silent leaks are the priority, they're what a user notices as "fugas".
- End with: the single most damaging leak, and whether the book is safe to build as-is.

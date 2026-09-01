# Benchmarks

Measured performance of `go-pretty-converter` rendering real books, from a quick
validation pass to a 3,533-page PDF. Every number below comes from a single
end-to-end CLI invocation — parse, validate, transpile, compose, render
(headless Chrome), and the automatic quality audit are all included in the
wall time.

## Latest results

| Source docs | `check` | `epub` | PDF pages | PDF size | PDF wall time | PDF throughput | `--fast` size | `--fast` wall time | `--fast` throughput |
|------------:|--------:|-------:|----------:|---------:|--------------:|----------------:|--------------:|-------------------:|---------------------:|
| 100         | 55 ms   | 64 ms  | 121       | 262 KiB  | 0.55 s        | 220 pages/s     | 136 KiB       | 0.43 s              | 279 pages/s          |
| 500         | 82 ms   | 106 ms | 591       | 1.2 MiB  | 1.27 s        | 466 pages/s     | 593 KiB       | 0.76 s              | 779 pages/s          |
| 1,000       | 112 ms  | 156 ms | 1,179     | 2.4 MiB  | 2.46 s        | 479 pages/s     | 1.1 MiB       | 1.37 s              | 861 pages/s          |
| 2,000       | 168 ms  | 256 ms | 2,356     | 4.8 MiB  | 6.05 s        | 390 pages/s     | 2.3 MiB       | 3.55 s              | 664 pages/s          |
| 3,000       | 221 ms  | 362 ms | 3,533     | 7.3 MiB  | 12.19 s       | 290 pages/s     | 3.4 MiB       | 6.86 s              | 515 pages/s          |

Machine-readable snapshot of this run: `benchmark-report.json` (produced by
`scripts/benchmark`, not checked into the repo — see Reproduce below).

The Go-side pipeline (parse + validate + compose) stays well under a quarter
second even at 3,000 docs — `check` on 3,000 documents takes 221 ms. PDF wall
time is dominated entirely by headless Chrome's own print/pagination engine,
not by Go, and that cost grows faster than linearly with page count (roughly
2,000 docs ≈ 3.3x the wall time of 1,000, not 2x) — a documented Chromium
print-pipeline characteristic for long single-flow documents, not something
this project's own code controls.

## `--fast`: the biggest lever for large books

`--fast` (shorthand for `--no-header --no-page-numbers --no-outline
--no-tagged-pdf`) cuts wall time by **40-44%** on books over ~500 documents,
and ~20% even on a 100-doc book — consistently, across every size measured
above. Phase-level timing (`PRETTY_CONVERTER_DEBUG_TIMING=1`, see below)
attributes this precisely, measured on a 3,000-doc/3,533-page book:

| Chrome feature (on top of bare pagination) | Added time | Share of default render |
|---|---:|---:|
| Page numbers + running header (native Chrome header/footer templates) | +3.0 s | ~25% |
| PDF bookmarks/outline | +0.05 s (negligible) | <1% |
| PDF/UA accessibility tagging | +1.4-2.1 s | ~15-18% |
| *(bare pagination, no extras)* | 6.9 s | — |

**Page numbers cost more than bookmarks and accessibility tagging combined.**
Bookmarks are effectively free — a document's heading structure is trivial for
Chrome to walk — so `--no-outline` alone buys almost nothing on its own;
the real cost is Chrome laying out and injecting a header/footer template into
every single page, and separately building a full accessibility tree during
`--tagged-pdf` generation. This previously undocumented breakdown is why
`--fast` exists as a single flag: remembering "disable outline and tagging"
alone (the two flags this project documented before) captured less than half
of the available speedup.

`--fast` also roughly halves the output file size (7.3 MiB → 3.4 MiB at
3,000 docs) — bookmarks and the accessibility tag tree are themselves real
PDF content, not free metadata.

Use `--fast` for CI artifacts, drafts, and any render where page numbers,
running headers, PDF bookmarks, or PDF/UA tagging aren't required. An
individual flag among the four still overrides `--fast` (e.g. `pretty-converter
build --fast --no-page-numbers=false` keeps page numbers while dropping the
other three).

## Methodology

### Hardware and toolchain (2026-08-31 run)

- CPU: AMD Ryzen 5 3600 (6 cores / 12 threads)
- RAM: 15 GiB
- OS: Linux 7.2.2 (CachyOS)
- Chrome: Google Chrome 152.0.7977.64
- Go: 1.26.5

### Procedure

1. `scripts/benchmark` builds the real CLI binary and generates a synthetic
   book of N documents. Each document has one-page body, valid `[X.Y.Z]`
   frontmatter, and default options — the same code path as a real project.
2. For each book size it runs real invocations, sampling the whole process
   tree (CLI + Chrome children) for peak RSS and CPU every ~90 ms:
   `pretty-converter check`, `pretty-converter epub`, `pretty-converter build`
   (default flags), and `pretty-converter build --fast`.
3. Wall time is measured process-wide; PDF pages are counted from the
   produced file; throughput is pages per wall second.

### Phase-level timing

Set `PRETTY_CONVERTER_DEBUG_TIMING=1` on any `build` invocation to print a
per-phase breakdown to stderr — navigate, DOM audit, `PrintToPDF`, output
write, and PDF-byte audit — useful for judging which of `--no-header`,
`--no-page-numbers`, `--no-outline`, or `--no-tagged-pdf` is worth it for
*your* document instead of guessing from these aggregate numbers.

## Reproduce

```bash
# system Chrome is auto-detected; pass --chrome-path if needed
PRETTY_CONVERTER_CHROME_PATH=/usr/bin/chromium go run ./scripts/benchmark --color=false
```

Flags: `--sizes 100,500,1000` (default `100,500,1000,2000,max`), `--max 3000`
(default), `--out DIR` (artifacts + `benchmark-report.json`, default a temp
dir). Each size runs `check`, `epub`, `build` (`pdf`), and `build --fast`
(`pdf-fast`) — the JSON report lists every one.

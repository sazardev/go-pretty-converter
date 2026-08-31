# Benchmarks

Measured performance of `go-pretty-converter` rendering real books, from a quick
validation pass to a 3,535-page PDF. Every number below comes from a single
end-to-end CLI invocation — parse, validate, transpile, compose, render
(headless Chrome), and the automatic quality audit are all included in the
wall time.

## Latest results

| Source docs | `check` | `epub` | PDF pages | PDF size | PDF wall time | PDF throughput |
|------------:|--------:|-------:|----------:|---------:|--------------:|---------------:|
| 100         | 30 ms   | 27 ms  | 121       | 244 KiB  | 0.57 s        | 211 pages/s    |
| 500         | 44 ms   | 56 ms  | 592       | 1.1 MiB  | 1.51 s        | 391 pages/s    |
| 1,000       | 62 ms   | 88 ms  | 1,180     | 2.2 MiB  | 2.99 s        | 394 pages/s    |
| 2,000       | 95 ms   | 180 ms | 2,358     | 4.4 MiB  | 11.52 s       | 205 pages/s    |
| 3,000       | 142 ms  | 248 ms | 3,535     | 6.6 MiB  | 23.02 s       | 154 pages/s    |

Machine-readable snapshot of this run: `benchmarks/benchmark-report.json`.

The Go-side pipeline (parse + validate + compose) stays sub-second even at
3,000 docs — `check` on 3,000 documents takes 142 ms. PDF wall time is
dominated by headless Chrome's print engine and the quality audit, not by Go.
Throughput peaks at ~390 pages/s around 500–1,000 docs; single renders past
~2,000 docs pay a Chrome-side cost as the output file grows, so very large
books are best split into per-chapter PDFs.

## Methodology

### Hardware and toolchain (2026-08-12 run)

- CPU: Intel Core i5-13500 (13th Gen, 18 threads)
- RAM: 23 GiB
- OS: Linux 6.6 (WSL2)
- Chrome: Chromium 150.0.7871.114
- Go: 1.26.5

### Procedure

1. `scripts/benchmark` builds the real CLI binary and generates a synthetic
   book of N documents. Each document has one-page body, valid `[X.Y.Z]`
   frontmatter, and default options — the same code path as a real project.
2. For each book size it runs three real invocations, sampling the whole
   process tree (CLI + Chrome children) for peak RSS and CPU every ~80 ms:
   `pretty-converter check`, `pretty-converter epub`, and `pretty-converter build`.
3. Wall time is measured process-wide; PDF pages are counted from the
   produced file; throughput is pages per wall second.

## Reproduce

```bash
# system Chrome is auto-detected; pass --chrome-path if needed
PRETTY_CONVERTER_CHROME_PATH=/usr/bin/chromium go run ./scripts/benchmark --color=false
```

Flags: `--sizes 100,500,1000` (default `100,500,1000,2000,max`), `--max 3000`
(default), `--out DIR` (artifacts + `benchmark-report.json`). To publish a
fresh snapshot, re-run and replace `benchmarks/benchmark-report.json`.

# Contributing to go-pretty-pdf

Thanks for your interest in contributing!

## Development setup

Requires Go 1.26+. Chrome/Chromium is optional — `pretty-pdf` auto-downloads
a headless build if none is found (see `chromemgr`).

```bash
git clone https://github.com/sazardev/go-pretty-pdf.git
cd go-pretty-pdf
go mod download
```

## Running tests

```bash
make test          # all tests with race detector
make test-cover    # with HTML coverage report
```

## Linting

```bash
make lint
```

Requires [golangci-lint](https://golangci-lint.run/usage/install/).

## Building

```bash
make build         # dev build to bin/
make build-release # stripped build
```

## Adding a builtin theme

Adding a theme to the CLI/library requires exactly one file and one
registry entry — nothing else to keep in sync:

1. Create `theme/assets/<name>.css`. Set the `--pdf-*` custom properties
   (see the contract documented at the top of `theme/assets/base.css`) plus
   any structural CSS deltas (e.g. a bordered `.cover`).
2. In `theme/builtin.go`: add a `//go:embed` var, a `Name<Foo>` constant, an
   entry in the `registry` map (set `Accented: true` if the theme uses its
   accent color as a bold structural element — a cover border, an
   accent-colored blockquote — rather than just for links), and append the
   constant to `order`.
3. Run `go test ./theme/... ./scripts/docsgen/...`.

That's it. The docs website (`scripts/docsgen`) reads colors, fonts, and
the accent treatment straight out of `theme.List()` and each theme's own
CSS at build time — the theme switcher, its swatch colors, and a
downloadable "docs as a PDF" rendered in that theme all appear
automatically on the next `go run ./scripts/docsgen`. There is no
site-side file to hand-edit or duplicate a palette into.

## Code conventions

- MDX frontmatter `id` field must use `[X.Y.Z]` format
- Documents sorted by ID, not filename
- Components registered via `WithComponent()` — never overwrite the parser
- Config file paths resolved relative to config file directory
- Pre-flight checks: Chrome availability checked first (hard failure), then source/output
- Partial parsing: per-file errors collected, never abort the whole parse

## Commit style

Use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` — new feature
- `fix:` — bug fix
- `docs:` — documentation
- `refactor:` — code restructuring
- `test:` — adding or updating tests
- `ci:` — CI/CD changes
- `chore:` — maintenance

## Pull requests

1. Fork and branch from `main`
2. Add tests for new functionality
3. Run `make lint` and `make test` before submitting
4. Keep PRs focused — one feature or fix per PR

## Releasing (versioning, bump, changelog)

SemVer (`MAJOR.MINOR.PATCH`); release tags carry a `v` prefix (`v0.10.0`).
The single source of truth is `version/version.go`; every build path reads it
or injects it at build time:

- plain `go build`: uses `version/version.go` directly
- `make build` / `make install`: ldflags from `git describe --tags`
- goreleaser: `-X .../version.Version={{ .Version }}` (the tag, `v`-prefixed)

To cut a release:

1. **Move the changelog**: rename `## [Unreleased]` in `CHANGELOG.md` to a
   dated `## [X.Y.Z] - YYYY-MM-DD` and start a fresh `[Unreleased]` on top.
   `make bump-*` never touches `CHANGELOG.md`, so do this first — the entry
   ships in the tag.
2. **Bump**: `make bump-patch` (or `bump-minor` / `bump-major`). This reads
   `version/version.go`, increments it, commits `chore: bump version to
   X.Y.Z`, and creates the annotated tag `vX.Y.Z`.
3. **Push**: `git push && git push --tags`. Tag pushes trigger
   `.github/workflows/release.yml` — tests on 3 OSes, then goreleaser builds
   the binaries and drafts the GitHub release.

Checks before pushing the tag:

- The tag is `v`-prefixed (the workflow only reacts to `v*`) and equals what
  `pretty-pdf version` prints.
- The `[X.Y.Z]` changelog date is the release day, and the section exists
  (the changelog's own header links `[X.Y.Z]` against `vX.Y.Z`).
- The next docs build picks the version up into `_site/version.json`
  automatically — nothing to hand-edit.

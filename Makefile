GOCMD      = go
MAIN       = ./cmd/pretty-converter
BINARY     = pretty-converter
BUILD_DIR  = bin
GOOS       = $(shell $(GOCMD) env GOOS)
VERSION    = $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS    = -s -w -X github.com/sazardev/go-pretty-converter/version.Version=$(VERSION)
LDFLAGS_NO_STRIP = -X github.com/sazardev/go-pretty-converter/version.Version=$(VERSION)

.PHONY: help lint test test-verbose test-cover build build-release install clean fmt tidy vulncheck \
        bump-patch bump-minor bump-major release-dry-run version-info

.DEFAULT_GOAL := help

##@ Development

help: ## Show this help
	@echo "go-pretty-converter Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@perl -nle '$$_=~/^([a-zA-Z_-]+):.*?##\s*(.*)/ and printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2' $(MAKEFILE_LIST)

lint: ## Run golangci-lint
	@golangci-lint run --timeout=5m

fmt: ## Format Go code
	@$(GOCMD) fmt ./...

tidy: ## Tidy module dependencies
	@$(GOCMD) mod tidy

vulncheck: ## Run govulncheck
	@go run golang.org/x/vuln/cmd/govulncheck@latest ./...

test: ## Run tests with race detector
	@$(GOCMD) test -race ./...

test-verbose: ## Run tests with verbose output
	@$(GOCMD) test -race -v ./...

test-cover: ## Run tests with coverage report
	@$(GOCMD) test -race -coverprofile=coverage.out ./...
	@$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@$(GOCMD) tool cover -func=coverage.out
	@echo ""
	@echo "Coverage report: coverage.html"

bench: ## Run the automated performance benchmark (renders books up to 3000 docs)
	@$(GOCMD) run ./scripts/benchmark

##@ Build

build: ## Build binary (dev)
	@mkdir -p $(BUILD_DIR)
	$(GOCMD) build -ldflags="$(LDFLAGS)" -o "$(BUILD_DIR)/$(BINARY)$(if $(filter windows,$(GOOS)),.exe)" $(MAIN)
	@echo "Built: $(BUILD_DIR)/$(BINARY)$(if $(filter windows,$(GOOS)),.exe) ($(VERSION))"

build-release: ## Build stripped binary
	@mkdir -p $(BUILD_DIR)
	$(GOCMD) build -ldflags="$(LDFLAGS)" -o "$(BUILD_DIR)/$(BINARY)$(if $(filter windows,$(GOOS)),.exe)" $(MAIN)
	@echo "Release build: $(BUILD_DIR)/$(BINARY)$(if $(filter windows,$(GOOS)),.exe) ($(VERSION))"

install: ## Install binary to GOPATH/bin
	@$(GOCMD) install -ldflags="$(LDFLAGS)" $(MAIN)
	@echo "Installed: $(shell go env GOPATH)/bin/$(BINARY)"

##@ Versioning

version-info: ## Print current version info
	@echo "Version: $(VERSION)"
	@echo "Module:  github.com/sazardev/go-pretty-converter"
	@echo "Source:  version/version.go"

bump-patch: ## Bump patch version (x.y.Z)
	@$(eval NEW_VER := $(shell $(GOCMD) run ./scripts/bump/bump.go patch))
	@echo "Bumped to $(NEW_VER)"
	@git add version/version.go
	@git commit -m "chore: bump version to $(NEW_VER)"
	@git tag -a "v$(NEW_VER)" -m "v$(NEW_VER)"
	@echo "Tagged v$(NEW_VER). Run 'git push --tags' to trigger release."
	@echo "Note: 'make bump-*' does NOT touch CHANGELOG.md — move the [Unreleased]"
	@echo "      section into [$(NEW_VER)] before pushing the tag (see CONTRIBUTING.md)."

bump-minor: ## Bump minor version (x.Y.z)
	@$(eval NEW_VER := $(shell $(GOCMD) run ./scripts/bump/bump.go minor))
	@echo "Bumped to $(NEW_VER)"
	@git add version/version.go
	@git commit -m "chore: bump version to $(NEW_VER)"
	@git tag -a "v$(NEW_VER)" -m "v$(NEW_VER)"
	@echo "Tagged v$(NEW_VER). Run 'git push --tags' to trigger release."
	@echo "Note: 'make bump-*' does NOT touch CHANGELOG.md — move the [Unreleased]"
	@echo "      section into [$(NEW_VER)] before pushing the tag (see CONTRIBUTING.md)."

bump-major: ## Bump major version (X.y.z)
	@$(eval NEW_VER := $(shell $(GOCMD) run ./scripts/bump/bump.go major))
	@echo "Bumped to $(NEW_VER)"
	@git add version/version.go
	@git commit -m "chore: bump version to $(NEW_VER)"
	@git tag -a "v$(NEW_VER)" -m "v$(NEW_VER)"
	@echo "Tagged v$(NEW_VER). Run 'git push --tags' to trigger release."
	@echo "Note: 'make bump-*' does NOT touch CHANGELOG.md — move the [Unreleased]"
	@echo "      section into [$(NEW_VER)] before pushing the tag (see CONTRIBUTING.md)."

##@ Release

release-dry-run: ## Test goreleaser without publishing
	@goreleaser release --clean --skip=publish --snapshot

##@ Cleanup

clean: ## Remove build artifacts and coverage
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html out.pdf
	@echo "Cleaned."

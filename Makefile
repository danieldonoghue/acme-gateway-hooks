GO ?= go
PKG := ./...
BIN_DIR := dist/bin
LOCAL_BIN_DIR := dist/bin-local
IMAGE ?= ghcr.io/danieldonoghue/acme-gateway-hooks
TAG ?= dev
PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: help build build-local test lint security docker-build release-artifacts clean

help: ## Show this help message
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

# -- Build --------------------------------------------------------------------

build: ## Build linux/amd64 and linux/arm64 static hook binaries
	@set -eu; \
	for arch in amd64 arm64; do \
		out_dir="$(BIN_DIR)/$$arch"; \
		mkdir -p "$$out_dir"; \
		for cmd_dir in cmd/*; do \
			[ -d "$$cmd_dir" ] || continue; \
			name=$${cmd_dir##*/}; \
			CGO_ENABLED=0 GOOS=linux GOARCH=$$arch $(GO) build -trimpath -ldflags="-s -w" -o "$$out_dir/$$name" "./$$cmd_dir"; \
		done; \
	done

build-local: ## Build local OS/arch hook binaries for development and e2e tests
	@set -eu; \
	mkdir -p "$(LOCAL_BIN_DIR)"; \
	for cmd_dir in cmd/*; do \
		[ -d "$$cmd_dir" ] || continue; \
		name=$${cmd_dir##*/}; \
		CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o "$(LOCAL_BIN_DIR)/$$name" "./$$cmd_dir"; \
	done

# -- Quality ------------------------------------------------------------------

test: ## Run tests with race detector and coverage
	$(GO) test $(PKG) -race -cover

lint: ## Run formatting and vet checks
	$(GO) fmt $(PKG)
	$(GO) vet $(PKG)

security: ## Run govulncheck (auto-installs if missing)
	@command -v govulncheck >/dev/null 2>&1 || $(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck $(PKG)

# -- Release ------------------------------------------------------------------

docker-build: ## Build a local container image with current TAG and latest tags
	docker build --build-arg VERSION=$(TAG) -t $(IMAGE):$(TAG) -t $(IMAGE):latest .

release-artifacts: build ## Create release tarball from built binaries
	mkdir -p dist/release
	@set -eu; \
	for arch in amd64 arm64; do \
		dir="$(BIN_DIR)/$$arch"; \
		out="dist/release/acme-gateway-hooks_linux_$$arch.tar.gz"; \
		files=""; \
		for f in "$$dir"/*; do \
			[ -f "$$f" ] || continue; \
			[ -x "$$f" ] || continue; \
			files="$$files $${f##*/}"; \
		done; \
		[ -n "$$files" ] || { echo "No executable files found in $$dir" >&2; exit 1; }; \
		tar -C "$$dir" -czf "$$out" $$files; \
	done

clean: ## Clean build artifacts
	rm -rf $(BIN_DIR) $(LOCAL_BIN_DIR) dist/release
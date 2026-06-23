GO ?= go
PKG := ./...
BIN_DIR := dist/bin
IMAGE ?= ghcr.io/danieldonoghue/acme-gateway-hooks
TAG ?= dev

.PHONY: help build test lint security docker-build release-artifacts

help: ## Show this help message
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

# -- Build --------------------------------------------------------------------

build: ## Build linux/amd64 static hook binaries
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/excedo-dns-deploy ./cmd/excedo-dns-deploy
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/excedo-dns-cleanup ./cmd/excedo-dns-cleanup

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

docker-build: ## Build container image with current TAG and latest tags
	docker build --build-arg VERSION=$(TAG) -t $(IMAGE):$(TAG) -t $(IMAGE):latest .

release-artifacts: build ## Create release tarball from built binaries
	mkdir -p dist/release
	tar -C $(BIN_DIR) -czf dist/release/acme-gateway-hooks_linux_amd64.tar.gz excedo-dns-deploy excedo-dns-cleanup

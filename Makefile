GO ?= go
PKG := ./...
BIN_DIR := dist/bin
LOCAL_BIN_DIR := dist/bin-local
IMAGE ?= ghcr.io/danieldonoghue/acme-gateway-hooks
TAG ?= dev

.PHONY: help build build-local test lint security docker-build release-artifacts

help: ## Show this help message
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

# -- Build --------------------------------------------------------------------

build: ## Build linux/amd64 static hook binaries
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/bind-dns-deploy ./cmd/bind-dns-deploy
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/bind-dns-cleanup ./cmd/bind-dns-cleanup
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/excedo-dns-deploy ./cmd/excedo-dns-deploy
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/excedo-dns-cleanup ./cmd/excedo-dns-cleanup

build-local: ## Build local OS/arch hook binaries for development and e2e tests
	mkdir -p $(LOCAL_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(LOCAL_BIN_DIR)/bind-dns-deploy ./cmd/bind-dns-deploy
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(LOCAL_BIN_DIR)/bind-dns-cleanup ./cmd/bind-dns-cleanup
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(LOCAL_BIN_DIR)/excedo-dns-deploy ./cmd/excedo-dns-deploy
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(LOCAL_BIN_DIR)/excedo-dns-cleanup ./cmd/excedo-dns-cleanup

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
	tar -C $(BIN_DIR) -czf dist/release/acme-gateway-hooks_linux_amd64.tar.gz bind-dns-deploy bind-dns-cleanup excedo-dns-deploy excedo-dns-cleanup

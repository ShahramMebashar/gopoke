SHELL := /bin/bash

GO ?= go
NPM ?= npm
GOCACHE ?= $(CURDIR)/.gocache

FRONTEND_DIR := cmd/gopad/frontend
APP_PKG := ./cmd/gopad
APP_BIN := gopad
WAILS_TAGS := wails,desktop,production

.DEFAULT_GOAL := help

.PHONY: help frontend-install frontend-build frontend-dev frontend-clean fmt fmt-check test test-wails vet run build check bench-warm-run bench-nfr stress-run-cancel release-macos clean clean-cache

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_.-]+:.*## / {printf "%-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

frontend-install: ## Install frontend dependencies
	cd $(FRONTEND_DIR) && $(NPM) install

frontend-build: ## Build frontend production assets to frontend/dist
	cd $(FRONTEND_DIR) && $(NPM) run build

frontend-dev: ## Start frontend dev server (Vite)
	cd $(FRONTEND_DIR) && $(NPM) run dev

frontend-clean: ## Remove generated frontend dist assets
	rm -rf $(FRONTEND_DIR)/dist

fmt: ## Format all Go files
	$(GO) fmt ./...

fmt-check: ## Fail if Go files are not formatted
	@files="$$(gofmt -l $$(find . -name '*.go' -type f | sort))"; \
	if [ -n "$$files" ]; then \
		echo "These files need gofmt:"; \
		echo "$$files"; \
		exit 1; \
	fi

test: ## Run Go tests
	GOCACHE=$(GOCACHE) $(GO) test ./...

test-wails: frontend-build ## Run tests for Wails-tagged app package
	GOCACHE=$(GOCACHE) $(GO) test -tags wails $(APP_PKG)

vet: ## Run go vet
	GOCACHE=$(GOCACHE) $(GO) vet ./...

run: frontend-build ## Run desktop app with Wails production desktop tags
	GOCACHE=$(GOCACHE) $(GO) run -tags "$(WAILS_TAGS)" $(APP_PKG)

build: frontend-build ## Build desktop binary
	GOCACHE=$(GOCACHE) $(GO) build -tags "$(WAILS_TAGS)" -o $(APP_BIN) $(APP_PKG)

check: fmt-check vet test test-wails ## Run local verification checks

bench-warm-run: ## Run warm-run latency benchmark harness (GP-022)
	GOCACHE=$(GOCACHE) $(GO) test -run '^$$' -bench BenchmarkWarmRunLatency -benchtime=10x ./internal/execution

bench-nfr: ## Run GP-039 NFR benchmark suite and generate report artifacts
	./scripts/run-nfr-benchmarks.sh artifacts

stress-run-cancel: ## Run GP-040 run/cancel reliability stress suite and report
	./scripts/run-run-cancel-stress.sh artifacts

release-macos: ## Build/package macOS app bundle + zip (optional sign/notarize via env)
	./scripts/release-macos.sh

clean: ## Remove built desktop binary
	rm -f $(APP_BIN)

clean-cache: ## Remove local Go build cache used by Make targets
	rm -rf $(GOCACHE)

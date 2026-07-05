# ffgo — developer tasks

BINARY      := ffgo
PKG         := github.com/arbazkhan971/ffgo
BUILDINFO   := $(PKG)/internal/buildinfo
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w \
	-X $(BUILDINFO).Version=$(VERSION) \
	-X $(BUILDINFO).Commit=$(COMMIT) \
	-X $(BUILDINFO).Date=$(DATE)

GO          ?= go
GOBIN       ?= $(shell $(GO) env GOPATH)/bin

.DEFAULT_GOAL := build

.PHONY: build
build: ## Build the ffgo binary
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) .

.PHONY: install
install: ## Install ffgo into GOBIN
	$(GO) install -trimpath -ldflags '$(LDFLAGS)' .

.PHONY: test
test: ## Run unit tests
	$(GO) test ./... -count=1

.PHONY: test-race
test-race: ## Run tests with the race detector
	$(GO) test -race ./... -count=1

.PHONY: cover
cover: ## Run tests and open an HTML coverage report
	$(GO) test ./... -coverprofile=coverage.txt -covermode=atomic
	$(GO) tool cover -html=coverage.txt -o coverage.html
	@echo "coverage.html written"

.PHONY: bench
bench: ## Run benchmarks
	$(GO) test -run=^$$ -bench=. -benchmem ./...

.PHONY: fmt
fmt: ## Format all Go files
	$(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: lint
lint: ## Run golangci-lint (install: https://golangci-lint.run)
	golangci-lint run ./...

.PHONY: tidy
tidy: ## Tidy go.mod/go.sum
	$(GO) mod tidy

.PHONY: check
check: fmt vet test ## Format, vet and test

.PHONY: snapshot
snapshot: ## Build a local multi-platform snapshot with goreleaser
	goreleaser release --snapshot --clean

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BINARY) $(BINARY).exe dist/ coverage.txt coverage.html

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

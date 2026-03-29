VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
REVISION := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X cderun/internal/version.Version=$(VERSION) \
           -X cderun/internal/version.Revision=$(REVISION) \
           -X cderun/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: test
test:
	@echo "Running all unit and integration tests..."
	@go test -v ./...

.PHONY: test-runtime
test-runtime:
	@echo "Running tests that require a container runtime (Docker/Podman)..."
	@go test -v -tags=runtime ./...

.PHONY: lint
lint: lint-go lint-md lint-actions link-check

.PHONY: pin-actions
pin-actions:
	@echo "Pinning GitHub Actions..."
	@go install github.com/suzuki-shunsuke/pinact/v3/cmd/pinact@v3.9.0
	@$(shell go env GOPATH)/bin/pinact run

.PHONY: lint-actions
lint-actions:
	@echo "Checking GitHub Actions pinning..."
	@go install github.com/suzuki-shunsuke/pinact/v3/cmd/pinact@v3.9.0
	@$(shell go env GOPATH)/bin/pinact run --verify --check

.PHONY: lint-go
lint-go:
	@echo "Running golangci-lint..."
	@golangci-lint run

.PHONY: lint-md
lint-md:
	@echo "Running markdownlint..."
	@if command -v markdownlint >/dev/null 2>&1; then \
		markdownlint "**/*.md"; \
	else \
		npx markdownlint-cli "**/*.md"; \
	fi

.PHONY: link-check
link-check:
	@echo "Checking Markdown links..."
	@./scripts/check-links.sh

.PHONY: coverage
coverage:
	@echo "Generating coverage report..."
	@go test ./... -cover -coverprofile=coverage.out
	@echo "Done. To view HTML report, run: go tool cover -html=coverage.out"

.PHONY: coverage-html
coverage-html: coverage
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Generated coverage.html"

.PHONY: build
build:
	@echo "Building cderun..."
	@go build -ldflags "$(LDFLAGS)" -o cderun main.go

.PHONY: clean
clean:
	@echo "Cleaning up..."
	@rm -f cderun coverage.out coverage.html

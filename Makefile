.PHONY: test
test:
	@echo "Running all unit and integration tests..."
	@go test -v ./...

.PHONY: test-e2e
test-e2e:
	@echo "Running E2E tests (requires Docker/Podman)..."
	@go test -v -tags=e2e ./...

.PHONY: lint
lint: lint-go lint-md link-check

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
	@go test ./... -cover -coverprofile=coverage.txt
	@echo "Done. To view HTML report, run: go tool cover -html=coverage.out"

.PHONY: coverage-html
coverage-html: coverage
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Generated coverage.html"

.PHONY: build
build:
	@echo "Building cderun..."
	@go build -o cderun main.go

.PHONY: clean
clean:
	@echo "Cleaning up..."
	@rm -f cderun coverage.out coverage.html

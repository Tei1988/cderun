.PHONY: test
test:
	@echo "Running all tests..."
	@go test -v ./...

.PHONY: lint
lint: lint-go lint-md

.PHONY: lint-go
lint-go:
	@echo "Running golangci-lint..."
	@golangci-lint run

.PHONY: lint-md
lint-md:
	@echo "Running markdownlint..."
	@markdownlint "**/*.md"

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
	@go build -o cderun main.go

.PHONY: clean
clean:
	@echo "Cleaning up..."
	@rm -f cderun coverage.out coverage.html

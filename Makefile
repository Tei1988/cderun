.PHONY: test
test:
	@echo "Running all tests..."
	@go test -v ./...

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

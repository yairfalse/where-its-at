.PHONY: fmt test build verify

fmt:
	@echo "Formatting code..."
	@gofmt -w .
	@echo "Code formatted successfully"

test:
	@echo "Running tests..."
	@go test ./...

build:
	@echo "Building all packages..."
	@go build ./...

verify: fmt
	@echo "Verifying code formatting..."
	@test -z "$$(gofmt -l . | grep -v vendor)" || (echo "Code is not formatted. Run 'make fmt'" && exit 1)
	@echo "Building all packages..."
	@go build ./...
	@echo "Running tests..."
	@go test ./...
	@echo "Verifying modules..."
	@go mod verify
	@echo "All verification passed!"

coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"
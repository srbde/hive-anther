.PHONY: fmt lint test build run-example clean docs

fmt:
	go fmt ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint is not installed. Please install it to run linting."; \
		exit 1; \
	fi

test:
	go test -v -race ./...

build:
	go build -v ./...

docs:
	gomarkdoc ./... > docs/API.md

run-example:
	go run main.go

clean:
	go clean

.PHONY: build test clean install lint

BINARY_NAME=impact
BUILD_DIR=bin
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
	@echo "Building $(BINARY_NAME) version $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) .

install:
	@echo "Installing $(BINARY_NAME)..."
	go install -ldflags="-X main.version=$(VERSION)" .

test:
	@echo "Running tests..."
	go test -v ./...

update-golden:
	@echo "Updating golden baselines..."
	UPDATE_GOLDEN=1 go test ./internal/orchestrator/ -run TestGoldenBaselines
	@echo "Golden baselines updated in testdata/golden/"

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed" && exit 1)
	golangci-lint run ./...

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	rm -rf .impact/cache

run-analyze:
	go run . analyze --diff HEAD~1 --verbose

run-test:
	go run . test --diff HEAD~1 --verbose

run-graph:
	go run . graph --file main.go --format mermaid

deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

.DEFAULT_GOAL := build

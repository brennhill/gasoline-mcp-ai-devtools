# Gasoline Build Makefile

VERSION := 3.0.0
BINARY_NAME := gasoline
BUILD_DIR := dist

# Build targets
PLATFORMS := \
	darwin-amd64 \
	darwin-arm64 \
	linux-amd64 \
	linux-arm64 \
	windows-amd64

.PHONY: all clean build test dev run checksums \
	lint lint-go lint-js format format-fix typecheck check ci \
	$(PLATFORMS)

all: clean build

clean:
	rm -rf $(BUILD_DIR)

test:
	CGO_ENABLED=0 go test -v ./cmd/dev-console/...

build: $(PLATFORMS)

darwin-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-x64 ./cmd/dev-console

darwin-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/dev-console

linux-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-x64 ./cmd/dev-console

linux-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/dev-console

windows-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-win32-x64.exe ./cmd/dev-console

# Build for current platform only (for development)
dev:
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/dev-console

# Run the server locally
run:
	CGO_ENABLED=0 go run ./cmd/dev-console

# Create checksums
checksums:
	cd $(BUILD_DIR) && shasum -a 256 * > checksums.txt

# --- Code Quality ---

lint: lint-go lint-js

lint-go:
	go vet ./cmd/dev-console/
	golangci-lint run ./cmd/dev-console/

lint-js:
	npx eslint extension/ extension-tests/

format:
	@echo "Checking Go formatting..."
	@test -z "$$(gofmt -l ./cmd/dev-console/)" || (gofmt -l ./cmd/dev-console/ && exit 1)
	npx prettier --check .

format-fix:
	gofmt -w ./cmd/dev-console/
	npx prettier --write .

typecheck:
	npx tsc --noEmit

check: lint format typecheck

ci: check test
	node --test extension-tests/*.test.js

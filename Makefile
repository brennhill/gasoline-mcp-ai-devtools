# Gasoline Build Makefile

VERSION := 4.6.0
BINARY_NAME := gasoline
BUILD_DIR := dist

# Build targets
PLATFORMS := \
	darwin-amd64 \
	darwin-arm64 \
	linux-amd64 \
	linux-arm64 \
	windows-amd64

.PHONY: all clean build test test-race test-cover test-bench test-fuzz \
	dev run checksums verify-zero-deps verify-imports verify-size \
	lint lint-go lint-js format format-fix typecheck check ci \
	$(PLATFORMS)

all: clean build

clean:
	rm -rf $(BUILD_DIR)

test:
	CGO_ENABLED=0 go test -v ./cmd/dev-console/...

test-race:
	go test -race -v ./cmd/dev-console/...

test-cover:
	go test -coverprofile=coverage.out ./cmd/dev-console/...
	@go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//' | \
		awk '{if ($$1 < 60) {print "FAIL: Coverage " $$1 "% is below 60% threshold"; exit 1} else {print "OK: Coverage " $$1 "%"}}'

test-bench:
	go test -bench=. -benchmem -count=3 ./cmd/dev-console/...

test-fuzz:
	go test -fuzz=. -fuzztime=10s ./cmd/dev-console/...

verify-zero-deps:
	@if grep -q '^require' go.mod; then echo "FAIL: go.mod contains external dependencies"; exit 1; fi
	@if [ -f go.sum ]; then echo "FAIL: go.sum exists (implies external dependencies)"; exit 1; fi
	@echo "OK: Zero external dependencies verified"

verify-imports:
	@VIOLATIONS=$$(go list -f '{{range .Imports}}{{.}} {{end}}' ./cmd/dev-console/ | tr ' ' '\n' | grep -v '^$$' | grep -v '^[a-z]' | grep -v '^github.com/dev-console/dev-console'); \
	if [ -n "$$VIOLATIONS" ]; then echo "FAIL: Non-stdlib imports found:"; echo "$$VIOLATIONS"; exit 1; fi
	@echo "OK: All imports are stdlib or internal"

verify-size:
	@make dev 2>/dev/null
	@SIZE=$$(wc -c < dist/gasoline | tr -d ' '); \
	MAX=15000000; \
	if [ $$SIZE -gt $$MAX ]; then echo "FAIL: Binary size $${SIZE} bytes exceeds $${MAX} byte limit"; exit 1; \
	else echo "OK: Binary size $${SIZE} bytes (limit: $${MAX})"; fi

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

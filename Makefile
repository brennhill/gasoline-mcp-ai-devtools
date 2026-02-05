# Gasoline Build Makefile

VERSION := 5.6.6
BINARY_NAME := gasoline
BUILD_DIR := dist
LDFLAGS := -s -w -X main.version=$(VERSION)

# Build targets
PLATFORMS := \
	darwin-amd64 \
	darwin-arm64 \
	linux-amd64 \
	linux-arm64 \
	windows-amd64

.PHONY: all clean build test test-js test-fast test-all test-race test-cover test-bench test-fuzz \
	dev run checksums verify-zero-deps verify-imports verify-size check-file-length \
	lint lint-go lint-js format format-fix typecheck check ci \
	ci-local ci-go ci-js ci-security ci-e2e ci-bench ci-fuzz \
	release-check install-hooks bench-baseline sync-version \
	pypi-binaries pypi-build pypi-publish pypi-test-publish pypi-clean \
	security-check pre-commit verify-all \
	$(PLATFORMS)

all: clean build

clean:
	rm -rf $(BUILD_DIR)

# Compile TypeScript to JavaScript (REQUIRED before tests)
compile-ts:
	@echo "=== Compiling TypeScript ==="
	@npx tsc
	@if [ ! -f extension/background/index.js ]; then \
		echo "❌ ERROR: TypeScript compilation failed - extension/background/index.js not found"; \
		exit 1; \
	fi
	@./scripts/fix-esm-imports.sh
	@echo "=== Bundling extension scripts ==="
	@node scripts/bundle-content.js
	@if [ ! -f extension/content.bundled.js ]; then \
		echo "❌ ERROR: Content script bundling failed"; \
		exit 1; \
	fi
	@if [ ! -f extension/inject.bundled.js ]; then \
		echo "❌ ERROR: Inject script bundling failed"; \
		exit 1; \
	fi
	@echo "✅ TypeScript compilation successful"

test:
	CGO_ENABLED=0 go test -v ./cmd/dev-console/...

test-js:
	node --test --test-force-exit --test-timeout=15000 --test-concurrency=4 --test-reporter=dot tests/extension/*.test.js

test-fast:
	go vet ./cmd/dev-console/
	node --test --test-force-exit --test-timeout=15000 --test-concurrency=4 --test-reporter=dot tests/extension/*.test.js

test-all: test test-js

test-race:
	go test -race -v ./cmd/dev-console/...

test-cover:
	go test -coverprofile=coverage.out ./cmd/dev-console/...
	@go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//' | \
		awk '{if ($$1 < 89) {print "FAIL: Coverage " $$1 "% is below 89% threshold"; exit 1} else {print "OK: Coverage " $$1 "%"}}'

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

# Check file line limits (800 lines soft limit)
check-file-length:
	@bash scripts/check-file-length.sh

build: $(PLATFORMS)

darwin-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-x64 ./cmd/dev-console

darwin-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/dev-console

linux-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-x64 ./cmd/dev-console

linux-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/dev-console

windows-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-win32-x64.exe ./cmd/dev-console

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
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./cmd/dev-console/ || echo "golangci-lint not installed (optional)"

lint-js:
	npx eslint extension/ tests/extension/

format:
	@echo "Checking Go formatting..."
	@test -z "$$(gofmt -l ./cmd/dev-console/)" || (gofmt -l ./cmd/dev-console/ && exit 1)
	npx prettier --check .

format-fix:
	gofmt -w ./cmd/dev-console/
	npx prettier --write .

typecheck:
	npx tsc --noEmit

check: lint format typecheck check-invariants

check-invariants:
	@./scripts/check-sync-invariants.sh

ci: check test test-js

# --- Local CI (mirrors GitHub Actions) ---

ci-local: ci-go ci-js ci-security
	@echo "All CI checks passed locally"

ci-e2e:
	cd tests/e2e && npm ci && npx playwright install chromium --with-deps && npx playwright test

extension-zip:
	@mkdir -p $(BUILD_DIR)
	@rm -f $(BUILD_DIR)/gasoline-extension-v$(VERSION).zip
	cd extension && zip -r ../$(BUILD_DIR)/gasoline-extension-v$(VERSION).zip \
		manifest.json background.js content.js inject.js \
		popup.html popup.js options.html options.js \
		icons/ lib/ \
		-x "*.DS_Store" "package.json"
	@echo "Built $(BUILD_DIR)/gasoline-extension-v$(VERSION).zip"
	@ls -lh $(BUILD_DIR)/gasoline-extension-v$(VERSION).zip

release-check: ci-local ci-e2e
	@echo "All release checks passed (CI + E2E)"

ci-go:
	go vet ./cmd/dev-console/
	make test-race
	make test-cover
	make build
	make verify-zero-deps

ci-js:
	npm ci
	npx eslint extension/ tests/extension/
	npx tsc --noEmit
	node --test --test-force-exit --test-timeout=20000 --test-concurrency=4 --test-reporter=dot tests/extension/*.test.js

ci-security:
	@command -v gosec >/dev/null 2>&1 && gosec -exclude=G104,G114,G204,G301,G304,G306 ./cmd/dev-console/ || echo "gosec not installed (optional - GitHub Actions will verify)"

ci-bench:
	@command -v benchstat >/dev/null 2>&1 || { echo "benchstat not found. Install: go install golang.org/x/perf/cmd/benchstat@latest"; exit 1; }
	@test -f docs/benchmarks/baseline.txt || { echo "FAIL: No baseline. Run 'make bench-baseline' first."; exit 1; }
	go test -bench=. -benchmem -count=6 -run=^$$ ./cmd/dev-console/ > /tmp/gasoline-bench-current.txt
	benchstat docs/benchmarks/baseline.txt /tmp/gasoline-bench-current.txt

ci-fuzz:
	go test -fuzz=FuzzPostLogs -fuzztime=30s ./cmd/dev-console/
	go test -fuzz=FuzzMCPRequest -fuzztime=30s ./cmd/dev-console/
	go test -fuzz=FuzzNetworkBodies -fuzztime=30s ./cmd/dev-console/
	go test -fuzz=FuzzWebSocketEvents -fuzztime=30s ./cmd/dev-console/
	go test -fuzz=FuzzEnhancedActions -fuzztime=30s ./cmd/dev-console/
	go test -fuzz=FuzzValidateLogEntry -fuzztime=30s ./cmd/dev-console/
	go test -fuzz=FuzzScreenshotEndpoint -fuzztime=30s ./cmd/dev-console/

bench-baseline:
	@mkdir -p benchmarks
	go test -bench=. -benchmem -count=6 -run=^$$ ./cmd/dev-console/ > docs/benchmarks/baseline.txt
	@echo "Baseline saved to docs/benchmarks/baseline.txt"

install-hooks:
	@cp scripts/hooks/pre-commit .git/hooks/pre-commit
	@cp scripts/hooks/pre-push .git/hooks/pre-push
	@chmod +x .git/hooks/pre-commit .git/hooks/pre-push
	@echo "Git hooks installed (pre-commit and pre-push)."

# --- Quality Gates ---

# Run all security checks (gosec for Go, ESLint security rules for JS)
security-check:
	@echo "Running security checks..."
	@command -v gosec >/dev/null 2>&1 || { echo "gosec not found. Install: go install github.com/securego/gosec/v2/cmd/gosec@latest"; exit 1; }
	gosec -exclude=G104,G114,G204,G301,G304,G306 -severity=high ./cmd/dev-console/
	npx eslint extension/ tests/extension/
	@echo "All security checks passed"

# Pre-commit quality gate (lint + security, no tests)
pre-commit: lint security-check
	@echo "Pre-commit checks passed"

# Full verification (lint + security + tests with coverage)
verify-all: lint security-check test-cover test-js
	@echo "All verification checks passed"

# Quality gate for top 1% standards (comprehensive)
quality-gate: check-file-length lint typecheck security-check test test-js
	@echo ""
	@echo "═══════════════════════════════════════════"
	@echo "✅ QUALITY GATE PASSED - Top 1% Standards"
	@echo "═══════════════════════════════════════════"
	@echo "  ✓ File length limits enforced"
	@echo "  ✓ Linting passed (ESLint + go vet)"
	@echo "  ✓ Type safety verified (TypeScript)"
	@echo "  ✓ Security checks passed"
	@echo "  ✓ All Go tests passed"
	@echo "  ✓ All TypeScript tests passed"
	@echo "═══════════════════════════════════════════"

# Update all version references to match VERSION (single source of truth)
sync-version:
	@echo "Syncing version to $(VERSION)..."
	@# JSON "version" fields
	@perl -pi -e 's/"version": "[0-9]+\.[0-9]+\.[0-9]+"/"version": "$(VERSION)"/g' \
		extension/manifest.json extension/package.json server/package.json \
		npm/gasoline-mcp/package.json npm/darwin-x64/package.json \
		npm/darwin-arm64/package.json npm/linux-x64/package.json \
		npm/linux-arm64/package.json npm/win32-x64/package.json \
		cmd/dev-console/testdata/mcp-initialize.golden.json
	@# NPM optionalDependencies versions
	@perl -pi -e 's/("@brennhill\/gasoline-[^"]+": ")[0-9]+\.[0-9]+\.[0-9]+(")/$${1}$(VERSION)$$2/g' \
		npm/gasoline-mcp/package.json
	@# PyPI version fields in pyproject.toml
	@perl -pi -e 's/^version = "[0-9]+\.[0-9]+\.[0-9]+"/version = "$(VERSION)"/' \
		pypi/gasoline-mcp/pyproject.toml \
		pypi/gasoline-mcp-darwin-arm64/pyproject.toml \
		pypi/gasoline-mcp-darwin-x64/pyproject.toml \
		pypi/gasoline-mcp-linux-arm64/pyproject.toml \
		pypi/gasoline-mcp-linux-x64/pyproject.toml \
		pypi/gasoline-mcp-win32-x64/pyproject.toml
	@# PyPI optional dependencies versions
	@perl -pi -e 's/(gasoline-mcp-[^"]+==)[0-9]+\.[0-9]+\.[0-9]+/$${1}$(VERSION)/g' \
		pypi/gasoline-mcp/pyproject.toml
	@# PyPI __init__.py versions
	@perl -pi -e 's/__version__ = "[0-9]+\.[0-9]+\.[0-9]+"/__version__ = "$(VERSION)"/' \
		pypi/gasoline-mcp/gasoline_mcp/__init__.py \
		pypi/gasoline-mcp-darwin-arm64/gasoline_mcp_darwin_arm64/__init__.py \
		pypi/gasoline-mcp-darwin-x64/gasoline_mcp_darwin_x64/__init__.py \
		pypi/gasoline-mcp-linux-arm64/gasoline_mcp_linux_arm64/__init__.py \
		pypi/gasoline-mcp-linux-x64/gasoline_mcp_linux_x64/__init__.py \
		pypi/gasoline-mcp-win32-x64/gasoline_mcp_win32_x64/__init__.py
	@# JS version strings
	@perl -pi -e "s/version: '[0-9]+\.[0-9]+\.[0-9]+'/version: '$(VERSION)'/g" \
		extension/inject.js tests/extension/popup.test.js
	@perl -pi -e "s/(parsed\.version, )'[0-9]+\.[0-9]+\.[0-9]+'/\$$1'$(VERSION)'/g" \
		tests/extension/background.test.js
	@perl -pi -e "s/VERSION = '[0-9]+\.[0-9]+\.[0-9]+'/VERSION = '$(VERSION)'/g" \
		server/scripts/install.js
	@# Go version fallback
	@perl -pi -e 's/var version = "[0-9]+\.[0-9]+\.[0-9]+"/var version = "$(VERSION)"/' \
		cmd/dev-console/main.go
	@# README badge and benchmark
	@perl -pi -e 's/version-[0-9]+\.[0-9]+\.[0-9]+-green/version-$(VERSION)-green/' README.md
	@perl -pi -e 's/\(v[0-9]+\.[0-9]+\.[0-9]+\)/(v$(VERSION))/' README.md
	@# Docs and benchmarks
	@perl -pi -e 's/Gasoline v[0-9]+\.[0-9]+\.[0-9]+/Gasoline v$(VERSION)/g' docs/getting-started.md
	@perl -pi -e 's/"version": "[0-9]+\.[0-9]+\.[0-9]+"/"version": "$(VERSION)"/g' docs/har-export.md
	@perl -pi -e 's/\*\*Version:\*\* [0-9]+\.[0-9]+\.[0-9]+/**Version:** $(VERSION)/' docs/benchmarks/latest-benchmark.md
	@echo "All files synced to $(VERSION)"

context-size:
	@echo "=== Claude Code Initial Context Size ==="
	@echo ""
	@total=0; \
	for f in CLAUDE.md $$(find .claude/docs -name "*.md" -type f 2>/dev/null) $$(find .claude -maxdepth 1 -name "*.md" -type f 2>/dev/null); do \
		chars=$$(wc -c < "$$f"); \
		tokens=$$((chars / 4)); \
		total=$$((total + chars)); \
		printf "%6d tokens  %s\n" "$$tokens" "$$f"; \
	done; \
	echo ""; \
	printf "%6d tokens  TOTAL (always loaded - startup)\n" "$$((total / 4))"; \
	echo ""; \
	ref_total=0; \
	for f in $$(find .claude/refs -name "*.md" -type f 2>/dev/null); do \
		chars=$$(wc -c < "$$f"); \
		ref_total=$$((ref_total + chars)); \
	done; \
	printf "%6d tokens  References (loaded on-demand)\n" "$$((ref_total / 4))"; \
	echo ""; \
	cmd_total=0; \
	for f in $$(find .claude/commands -name "*.md" -type f 2>/dev/null); do \
		chars=$$(wc -c < "$$f"); \
		cmd_total=$$((cmd_total + chars)); \
	done; \
	printf "%6d tokens  Commands (loaded on invoke only)\n" "$$((cmd_total / 4))"; \
	echo ""; \
	echo "Budget: <5K lean | 5-15K normal | 15-30K heavy | >30K too large"

# --- PyPI Distribution ---

pypi-binaries: build
	@echo "Copying binaries to PyPI platform packages..."
	@cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 pypi/gasoline-mcp-darwin-arm64/gasoline_mcp_darwin_arm64/
	@cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-x64 pypi/gasoline-mcp-darwin-x64/gasoline_mcp_darwin_x64/
	@cp $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 pypi/gasoline-mcp-linux-arm64/gasoline_mcp_linux_arm64/
	@cp $(BUILD_DIR)/$(BINARY_NAME)-linux-x64 pypi/gasoline-mcp-linux-x64/gasoline_mcp_linux_x64/
	@cp $(BUILD_DIR)/$(BINARY_NAME)-win32-x64.exe pypi/gasoline-mcp-win32-x64/gasoline_mcp_win32_x64/
	@echo "Binaries copied successfully"

pypi-build: pypi-binaries
	@echo "Building PyPI wheels..."
	@for pkg in pypi/gasoline-mcp-*/; do \
		echo "Building $$pkg..."; \
		cd $$pkg && python3 -m build && cd ../..; \
	done
	@echo "Building main package..."
	@cd pypi/gasoline-mcp && python3 -m build
	@echo "All PyPI packages built successfully"
	@echo ""
	@echo "Wheels created:"
	@find pypi -name "*.whl" -type f

pypi-test-publish: pypi-build
	@echo "Publishing to Test PyPI..."
	@echo "NOTE: Requires TWINE_USERNAME and TWINE_PASSWORD environment variables"
	@for pkg in pypi/gasoline-mcp-*/; do \
		echo "Uploading $$pkg..."; \
		cd $$pkg && python3 -m twine upload --repository testpypi dist/* && cd ../..; \
	done
	@echo "Uploading main package..."
	@cd pypi/gasoline-mcp && python3 -m twine upload --repository testpypi dist/*
	@echo "All packages published to Test PyPI"
	@echo "Test installation: pip install --index-url https://test.pypi.org/simple/ gasoline-mcp"

pypi-publish: pypi-build
	@echo "Publishing to PyPI..."
	@echo "NOTE: Requires TWINE_USERNAME and TWINE_PASSWORD environment variables"
	@echo "Press Ctrl+C to cancel, or Enter to continue..."
	@read dummy
	@for pkg in pypi/gasoline-mcp-*/; do \
		echo "Uploading $$pkg..."; \
		cd $$pkg && python3 -m twine upload dist/* && cd ../..; \
	done
	@echo "Uploading main package..."
	@cd pypi/gasoline-mcp && python3 -m twine upload dist/*
	@echo "All packages published to PyPI"
	@echo "Installation: pip install gasoline-mcp"

pypi-clean:
	@echo "Cleaning PyPI build artifacts..."
	@find pypi -type d -name "build" -exec rm -rf {} + 2>/dev/null || true
	@find pypi -type d -name "dist" -exec rm -rf {} + 2>/dev/null || true
	@find pypi -type d -name "*.egg-info" -exec rm -rf {} + 2>/dev/null || true
	@find pypi -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	@echo "PyPI artifacts cleaned"

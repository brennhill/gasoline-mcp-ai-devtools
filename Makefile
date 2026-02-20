# Gasoline Build Makefile

VERSION := $(shell cat VERSION)
BINARY_NAME := gasoline
BUILD_DIR := dist
LDFLAGS := -s -w -X main.version=$(VERSION) -X github.com/dev-console/dev-console/internal/export.version=$(VERSION)
CMD_PKG ?= ./cmd/dev-console
CMD_DIR ?= $(patsubst ./%,%,$(CMD_PKG))

# Build targets
PLATFORMS := \
	darwin-amd64 \
	darwin-arm64 \
	linux-amd64 \
	linux-arm64 \
	windows-amd64

.PHONY: all clean build test test-js test-fast test-all test-go-quick test-go-long test-go-sharded test-race test-cover test-cover-integration test-cover-all test-bench test-fuzz \
	dev run checksums verify-zero-deps verify-imports verify-size check-file-length \
	lint lint-go lint-js format format-fix typecheck check check-wire-drift ci \
	ci-local ci-go ci-js ci-security ci-e2e ci-bench ci-fuzz \
	release-check install-hooks bench-baseline sync-version \
	pypi-binaries pypi-build pypi-publish pypi-test-publish pypi-clean \
	security-check pre-commit verify-all npm-binaries validate-semver \
	test-upgrade-guards release-gate clean-test-daemons \
	generate-wire-types generate-dom-primitives \
	$(PLATFORMS)

GO_TEST_SHARDS ?= 4
GO_TEST_COUNT ?= 1
GO_TEST_PARALLEL ?= 16
GO_TEST_P ?= 8
GO_TEST_STATE_DIR ?= /tmp/gasoline-state-test
GO_TEST_TOOLCHAIN ?= auto
GO_TEST_CACHE_DIR ?= /tmp/go-build-cache

all: validate-semver clean build

clean:
	rm -rf $(BUILD_DIR)

# Generate TypeScript wire types from Go source of truth
generate-wire-types:
	@node scripts/generate-wire-types.js

generate-dom-primitives:
	@node scripts/generate-dom-primitives.js

# Compile TypeScript to JavaScript (REQUIRED before tests)
compile-ts: generate-wire-types generate-dom-primitives
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
	@if [ ! -f extension/early-patch.bundled.js ]; then \
		echo "❌ ERROR: Early-patch script bundling failed"; \
		exit 1; \
	fi
	@echo "✅ TypeScript compilation successful"

test: check-file-length
	$(MAKE) test-go-quick

test-long:
	$(MAKE) test-go-long

test-js: compile-ts
	./scripts/test-js-sharded.sh

test-fast:
	go vet $(CMD_PKG)/
	$(MAKE) test-go-quick
	./scripts/test-js-sharded.sh

test-all: test test-js

test-go-quick:
	@set -e; trap 'bash ./scripts/cleanup-test-daemons.sh --quiet >/dev/null 2>&1 || true' EXIT; \
	CGO_ENABLED=0 GOTOOLCHAIN=$(GO_TEST_TOOLCHAIN) GOCACHE=$(GO_TEST_CACHE_DIR) GASOLINE_STATE_DIR=$(GO_TEST_STATE_DIR) go test -short -count=$(GO_TEST_COUNT) -p $(GO_TEST_P) -parallel $(GO_TEST_PARALLEL) ./internal/...; \
	CGO_ENABLED=0 GOTOOLCHAIN=$(GO_TEST_TOOLCHAIN) GOCACHE=$(GO_TEST_CACHE_DIR) GASOLINE_STATE_DIR=$(GO_TEST_STATE_DIR) GO_TEST_SHARDS=$(GO_TEST_SHARDS) GO_TEST_COUNT=$(GO_TEST_COUNT) GASOLINE_CMD_PKG=$(CMD_PKG) ./scripts/test-go-sharded.sh --package $(CMD_PKG) --short -- -parallel $(GO_TEST_PARALLEL)

test-go-long:
	@set -e; trap 'bash ./scripts/cleanup-test-daemons.sh --quiet >/dev/null 2>&1 || true' EXIT; \
	CGO_ENABLED=0 GOTOOLCHAIN=$(GO_TEST_TOOLCHAIN) GOCACHE=$(GO_TEST_CACHE_DIR) GASOLINE_STATE_DIR=$(GO_TEST_STATE_DIR) go test -count=$(GO_TEST_COUNT) -p $(GO_TEST_P) -parallel $(GO_TEST_PARALLEL) ./internal/...; \
	CGO_ENABLED=0 GOTOOLCHAIN=$(GO_TEST_TOOLCHAIN) GOCACHE=$(GO_TEST_CACHE_DIR) GASOLINE_STATE_DIR=$(GO_TEST_STATE_DIR) GO_TEST_SHARDS=$(GO_TEST_SHARDS) GO_TEST_COUNT=$(GO_TEST_COUNT) GASOLINE_CMD_PKG=$(CMD_PKG) ./scripts/test-go-sharded.sh --package $(CMD_PKG) -- -parallel $(GO_TEST_PARALLEL)

test-go-sharded:
	@set -e; trap 'bash ./scripts/cleanup-test-daemons.sh --quiet >/dev/null 2>&1 || true' EXIT; \
	CGO_ENABLED=0 GOTOOLCHAIN=$(GO_TEST_TOOLCHAIN) GOCACHE=$(GO_TEST_CACHE_DIR) GASOLINE_STATE_DIR=$(GO_TEST_STATE_DIR) GO_TEST_SHARDS=$(GO_TEST_SHARDS) GO_TEST_COUNT=$(GO_TEST_COUNT) GASOLINE_CMD_PKG=$(CMD_PKG) ./scripts/test-go-sharded.sh --package $(CMD_PKG) -- -parallel $(GO_TEST_PARALLEL)

test-race:
	go test -race -v ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//' | \
		awk '{if ($$1 < 89) {print "FAIL: Coverage " $$1 "% is below 89% threshold"; exit 1} else {print "OK: Coverage " $$1 "%"}}'

test-cover-integration:
	@mkdir -p coverage/integration
	GOCOVERDIR=coverage/integration go test -cover -timeout 120s $(CMD_PKG)/ -count=1
	@go tool covdata percent -i=coverage/integration

test-cover-all:
	@mkdir -p coverage/unit coverage/integration coverage/merged
	GOCOVERDIR=coverage/unit go test -cover ./internal/...
	GOCOVERDIR=coverage/integration go test -cover -timeout 120s $(CMD_PKG)/ -count=1
	go tool covdata merge -i=coverage/unit,coverage/integration -o=coverage/merged
	go tool covdata textfmt -i=coverage/merged -o=coverage/coverage.txt
	@go tool cover -func=coverage/coverage.txt | grep total
	@echo "HTML report: go tool cover -html=coverage/coverage.txt"

test-bench:
	go test -bench=. -benchmem -count=3 $(CMD_PKG)/...

test-fuzz:
	go test -fuzz=. -fuzztime=10s $(CMD_PKG)/...

clean-test-daemons:
	bash ./scripts/cleanup-test-daemons.sh

verify-zero-deps:
	@if grep -q '^require' go.mod; then echo "FAIL: go.mod contains external dependencies"; exit 1; fi
	@if [ -f go.sum ]; then echo "FAIL: go.sum exists (implies external dependencies)"; exit 1; fi
	@echo "OK: Zero external dependencies verified"

verify-imports:
	@VIOLATIONS=$$(go list -f '{{range .Imports}}{{.}} {{end}}' $(CMD_PKG)/ | tr ' ' '\n' | grep -v '^$$' | grep -v '^[a-z]' | grep -v '^github.com/dev-console/dev-console'); \
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

# Validate strict semver (X.Y.Z format, no pre-release)
validate-semver:
	@bash scripts/validate-semver.sh

# Validate optionalDependencies match package version
validate-deps-versions:
	@node npm/gasoline-mcp/lib/validate-versions.js

build: $(PLATFORMS)

darwin-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-x64 $(CMD_PKG)

darwin-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PKG)

linux-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-x64 $(CMD_PKG)

linux-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PKG)

windows-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-win32-x64.exe $(CMD_PKG)

# Build and copy binaries to NPM package directories (for releases)
npm-binaries: build
	@echo "=== Copying binaries to NPM packages ==="
	cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 npm/darwin-arm64/bin/gasoline
	cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-x64 npm/darwin-x64/bin/gasoline
	cp $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 npm/linux-arm64/bin/gasoline
	cp $(BUILD_DIR)/$(BINARY_NAME)-linux-x64 npm/linux-x64/bin/gasoline
	cp $(BUILD_DIR)/$(BINARY_NAME)-win32-x64.exe npm/win32-x64/bin/gasoline.exe
	@echo "=== Verifying embedded versions ==="
	@EMBEDDED=$$(npm/darwin-arm64/bin/gasoline --version 2>&1 | grep -o '[0-9]\+\.[0-9]\+\.[0-9]\+'); \
	EXPECTED=$$(cat VERSION); \
	if [ "$$EMBEDDED" != "$$EXPECTED" ]; then \
		echo "❌ ERROR: Embedded version $$EMBEDDED does not match VERSION file $$EXPECTED"; \
		exit 1; \
	fi
	@echo "✅ NPM binaries ready with version $(VERSION)"

# Build for current platform only (for development)
dev:
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PKG)

# Run the server locally
run:
	CGO_ENABLED=0 go run $(CMD_PKG)

# Create checksums
checksums:
	cd $(BUILD_DIR) && shasum -a 256 * > checksums.txt

# --- Code Quality ---

lint: lint-go lint-js

lint-go:
	go vet $(CMD_PKG)/
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run $(CMD_PKG)/... ./internal/... || echo "golangci-lint not installed (optional)"

lint-hardening:
	@./scripts/lint-hardening.sh

lint-js:
	npx eslint extension/ tests/extension/

format:
	@echo "Checking Go formatting..."
	@test -z "$$(gofmt -l $(CMD_DIR)/)" || (gofmt -l $(CMD_DIR)/ && exit 1)
	npx prettier --check .

format-fix:
	gofmt -w $(CMD_DIR)/
	npx prettier --write .

typecheck:
	npx tsc --noEmit

check: check-file-length lint format typecheck check-invariants

check-wire-drift:
	@node scripts/generate-wire-types.js --check

check-invariants: check-wire-drift
	@./scripts/check-sync-invariants.sh
	@./scripts/validate-codex-skills.sh

ci: check test test-js validate-deps-versions

# --- Local CI (mirrors GitHub Actions) ---

ci-local: ci-go ci-js ci-security
	@echo "All CI checks passed locally"

ci-e2e:
	cd tests/e2e && npm ci && npx playwright install chromium --with-deps && npx playwright test

extension-zip:
	@mkdir -p $(BUILD_DIR)
	@rm -f $(BUILD_DIR)/gasoline-extension-v$(VERSION).zip
	cd extension && zip -r ../$(BUILD_DIR)/gasoline-extension-v$(VERSION).zip \
		manifest.json background.js content.js inject.js early-patch.js \
		early-patch.bundled.js content.bundled.js inject.bundled.js \
		popup.html popup.js options.html options.js \
		icons/ lib/ \
		-x "*.DS_Store" "package.json"
	@echo "Built $(BUILD_DIR)/gasoline-extension-v$(VERSION).zip"
	@ls -lh $(BUILD_DIR)/gasoline-extension-v$(VERSION).zip

extension-crx:
	@node scripts/build-crx.js

release-check: ci-local ci-e2e
	@echo "All release checks passed (CI + E2E)"

ci-go:
	go vet $(CMD_PKG)/
	make test-race
	make test-cover
	make build
	make verify-zero-deps

ci-js:
	npm ci
	npx eslint extension/ tests/extension/
	npx tsc --noEmit
	JS_TEST_TIMEOUT=20000 ./scripts/test-js-sharded.sh

ci-security:
	@command -v gosec >/dev/null 2>&1 && gosec -exclude=G104,G114,G204,G301,G304,G306 $(CMD_PKG)/ || echo "gosec not installed (optional - GitHub Actions will verify)"

ci-bench:
	@command -v benchstat >/dev/null 2>&1 || { echo "benchstat not found. Install: go install golang.org/x/perf/cmd/benchstat@latest"; exit 1; }
	@test -f docs/benchmarks/baseline.txt || { echo "FAIL: No baseline. Run 'make bench-baseline' first."; exit 1; }
	go test -bench=. -benchmem -count=6 -run=^$$ $(CMD_PKG)/ > /tmp/gasoline-bench-current.txt
	benchstat docs/benchmarks/baseline.txt /tmp/gasoline-bench-current.txt

ci-fuzz:
	go test -fuzz=FuzzPostLogs -fuzztime=30s $(CMD_PKG)/
	go test -fuzz=FuzzMCPRequest -fuzztime=30s $(CMD_PKG)/
	go test -fuzz=FuzzNetworkBodies -fuzztime=30s $(CMD_PKG)/
	go test -fuzz=FuzzWebSocketEvents -fuzztime=30s $(CMD_PKG)/
	go test -fuzz=FuzzEnhancedActions -fuzztime=30s $(CMD_PKG)/
	go test -fuzz=FuzzValidateLogEntry -fuzztime=30s $(CMD_PKG)/
	go test -fuzz=FuzzScreenshotEndpoint -fuzztime=30s $(CMD_PKG)/

bench-baseline:
	@mkdir -p benchmarks
	go test -bench=. -benchmem -count=6 -run=^$$ $(CMD_PKG)/ > docs/benchmarks/baseline.txt
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
	gosec -exclude=G104,G114,G204,G301,G304,G306 -severity=high $(CMD_PKG)/
	npx eslint extension/ tests/extension/
	@echo "All security checks passed"

# Pre-commit quality gate (lint + security, no tests)
pre-commit: lint security-check
	@echo "Pre-commit checks passed"

# Full verification (lint + security + tests with coverage)
verify-all: lint security-check test-cover test-js
	@echo "All verification checks passed"

# Quality gate for top 1% standards (comprehensive)
quality-gate: check-file-length lint lint-hardening typecheck security-check test test-js validate-deps-versions
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
	@echo "  ✓ Version consistency verified"
	@echo "═══════════════════════════════════════════"

# Upgrade/install guardrail suite: prevents stale daemons from surviving release upgrades.
test-upgrade-guards:
	go test ./cmd/dev-console -run 'TestConnectWithRetriesRejectsVersionMismatch' -count=1
	node --test scripts/install-upgrade-regression.contract.test.mjs
	node --test npm/gasoline-mcp/lib/kill-daemon.test.js
	python3 -m unittest discover -s pypi/gasoline-mcp/tests -p 'test_*.py'
	node scripts/install-upgrade-regression.mjs

# Release gate for daemon cleanup/version safety.
release-gate: quality-gate test-upgrade-guards
	@echo "✅ release-gate passed"

# Update all version references to match VERSION (single source of truth)
sync-version:
	@echo "Syncing version to $(VERSION)..."
	@# JSON "version" fields
	@perl -pi -e 's/"version": "[0-9]+\.[0-9]+\.[0-9]+"/"version": "$(VERSION)"/g' \
			extension/manifest.json extension/package.json server/package.json \
			npm/gasoline-mcp/package.json npm/darwin-x64/package.json \
			npm/darwin-arm64/package.json npm/linux-x64/package.json \
			npm/linux-arm64/package.json npm/win32-x64/package.json \
			$(CMD_DIR)/testdata/mcp-initialize.golden.json
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
		$(CMD_DIR)/main.go
	@# Shell wrapper version
	@perl -pi -e 's/GASOLINE_VERSION="[0-9]+\.[0-9]+\.[0-9]+"/GASOLINE_VERSION="$(VERSION)"/' \
		npm/gasoline-mcp/bin/gasoline-mcp
	@# README badge and benchmark
	@perl -pi -e 's/version-[0-9]+\.[0-9]+\.[0-9]+-green/version-$(VERSION)-green/' README.md
	@perl -pi -e 's/\(v[0-9]+\.[0-9]+\.[0-9]+\)/(v$(VERSION))/' README.md
	@# Docs and benchmarks
	@perl -pi -e 's/Gasoline v[0-9]+\.[0-9]+\.[0-9]+/Gasoline v$(VERSION)/g' docs/getting-started.md
	@perl -pi -e 's/\[gasoline\] v[0-9]+\.[0-9]+\.[0-9]+/[gasoline] v$(VERSION)/g' docs/getting-started.md
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
	@cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 pypi/gasoline-mcp-darwin-arm64/gasoline_mcp_darwin_arm64/gasoline
	@cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-x64 pypi/gasoline-mcp-darwin-x64/gasoline_mcp_darwin_x64/gasoline
	@cp $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 pypi/gasoline-mcp-linux-arm64/gasoline_mcp_linux_arm64/gasoline
	@cp $(BUILD_DIR)/$(BINARY_NAME)-linux-x64 pypi/gasoline-mcp-linux-x64/gasoline_mcp_linux_x64/gasoline
	@cp $(BUILD_DIR)/$(BINARY_NAME)-win32-x64.exe pypi/gasoline-mcp-win32-x64/gasoline_mcp_win32_x64/gasoline.exe
	@echo "Binaries copied successfully"

pypi-preflight:
	@python3 -c 'import importlib.util,sys; missing=[n for n in ("build","setuptools","wheel") if importlib.util.find_spec(n) is None]; \
	print("Python build modules available: build, setuptools, wheel") if not missing else ( \
	print("ERROR: Missing Python build modules: " + ", ".join(missing)), \
	print("System Python may be externally managed. Use a virtual environment:") if sys.prefix==sys.base_prefix else print("Install with: python -m pip install --upgrade build setuptools wheel"), \
	print("  python3 -m venv .venv-build") if sys.prefix==sys.base_prefix else None, \
	print("  source .venv-build/bin/activate") if sys.prefix==sys.base_prefix else None, \
	print("  python -m pip install --upgrade pip build setuptools wheel") if sys.prefix==sys.base_prefix else None, \
	sys.exit(1))'

pypi-schema-check:
	@echo "Checking PyPI main pyproject normalization..."
	@node scripts/normalize-pypi-main-pyproject.js --check --validate
	@python3 -c 'import sys,tomllib; p="pypi/gasoline-mcp/pyproject.toml"; d=tomllib.load(open(p,"rb")); project=d.get("project", {}); scripts=project.get("scripts", {}); \
	(isinstance(scripts, dict) or (print("ERROR: [project.scripts] must be a TOML table"), sys.exit(1))); \
	("dependencies" not in scripts or (print("ERROR: project.scripts.dependencies must not exist"), sys.exit(1))); \
	print("PyPI schema check ok"); print("project keys:", sorted(project.keys())); print("project.scripts keys:", sorted(scripts.keys())); print("project.dependencies count:", len(project.get("dependencies", [])))'

pypi-build: pypi-preflight pypi-schema-check
	@$(MAKE) pypi-binaries
	@echo "Normalizing PyPI main pyproject metadata..."
	@node scripts/normalize-pypi-main-pyproject.js
	@echo "Validating PyPI main pyproject metadata..."
	@node scripts/normalize-pypi-main-pyproject.js --validate
	@echo "Building PyPI wheels..."
	@for pkg in pypi/gasoline-mcp-*/; do \
		echo "Building $$pkg..."; \
		(cd "$$pkg" && python3 -m build); \
	done
	@echo "Building main package..."
	@(cd pypi/gasoline-mcp && python3 -m build)
	@echo "All PyPI packages built successfully"
	@echo ""
	@echo "Wheels created:"
	@find pypi -name "*.whl" -type f

pypi-test-publish: pypi-build
	@echo "Publishing to Test PyPI..."
	@echo "NOTE: Requires TWINE_USERNAME and TWINE_PASSWORD environment variables"
	@for pkg in pypi/gasoline-mcp-*/; do \
		echo "Uploading $$pkg..."; \
		(cd "$$pkg" && python3 -m twine upload --repository testpypi dist/*); \
	done
	@echo "Uploading main package..."
	@(cd pypi/gasoline-mcp && python3 -m twine upload --repository testpypi dist/*)
	@echo "All packages published to Test PyPI"
	@echo "Test installation: pip install --index-url https://test.pypi.org/simple/ gasoline-mcp"

pypi-publish: pypi-build
	@echo "Publishing to PyPI..."
	@echo "NOTE: Requires TWINE_USERNAME and TWINE_PASSWORD environment variables"
	@echo "Press Ctrl+C to cancel, or Enter to continue..."
	@read dummy
	@for pkg in pypi/gasoline-mcp-*/; do \
		echo "Uploading $$pkg..."; \
		(cd "$$pkg" && python3 -m twine upload dist/*); \
	done
	@echo "Uploading main package..."
	@(cd pypi/gasoline-mcp && python3 -m twine upload dist/*)
	@echo "All packages published to PyPI"
	@echo "Installation: pip install gasoline-mcp"

pypi-clean:
	@echo "Cleaning PyPI build artifacts..."
	@find pypi -type d -name "build" -exec rm -rf {} + 2>/dev/null || true
	@find pypi -type d -name "dist" -exec rm -rf {} + 2>/dev/null || true
	@find pypi -type d -name "*.egg-info" -exec rm -rf {} + 2>/dev/null || true
	@find pypi -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	@echo "PyPI artifacts cleaned"

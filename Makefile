# Copyright (C) 2026 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# SPDX-License-Identifier: Apache-2.0

.PHONY: help build test test-coverage fmt vet clean install check-fmt check-vet ci-checks all test-mutate quality-report

# Binary name
BINARY_NAME=kortex-cli
# Build output directory
BUILD_DIR=.
# Go command
GO=go
# Go files
GOFILES=$(shell find . -type f -name '*.go' -not -path "./vendor/*")

# Default target
all: build

help: ## Display this help message
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the kortex-cli binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kortex-cli

install: ## Install the binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	$(GO) install ./cmd/kortex-cli

test: ## Run all tests
	@echo "Running tests..."
	$(GO) test -v -race ./...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	$(GO) test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

fmt: ## Format code with gofmt
	@echo "Formatting code..."
	@gofmt -w $(GOFILES)

vet: ## Run go vet
	@echo "Running go vet..."
	$(GO) vet ./...

check-fmt: ## Check if code is formatted (for CI)
	@echo "Checking code formatting..."
	@unformatted=$$(gofmt -l $(GOFILES)); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not formatted:"; \
		echo "$$unformatted"; \
		echo "Run 'make fmt' to format the code."; \
		exit 1; \
	fi
	@echo "All files are properly formatted."

check-vet: ## Run go vet and fail on issues (for CI)
	@echo "Running go vet..."
	@$(GO) vet ./... || (echo "go vet found issues. Please fix them."; exit 1)

ci-checks: check-fmt check-vet test ## Run all CI checks
	@echo "All CI checks passed!"

clean: ## Remove build artifacts
	@echo "Cleaning build artifacts..."
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@rm -f *.test
	@echo "Clean complete."

test-mutate: ## Run mutation testing with gremlins
	@echo "Running mutation testing..."
	@command -v gremlins >/dev/null 2>&1 || (echo "Install gremlins: go install github.com/go-gremlins/gremlins/cmd/gremlins@latest" && exit 1)
	gremlins unleash --tags="" ./pkg/...

# ── Quality Report ────────────────────────────────────────────────────────────

quality-report: ## Run coverage gap analysis and mutation testing
	@echo "=== Quality Report ==="
	@echo ""
	@echo "── Coverage ──"
	@$(GO) test -coverprofile=coverage.out -covermode=atomic ./... >/dev/null 2>&1
	@$(GO) tool cover -func=coverage.out > func-coverage.txt
	@TOTAL=$$(grep 'total:' func-coverage.txt | awk '{print $$NF}'); \
	UNCOVERED=$$(grep '0.0%' func-coverage.txt 2>/dev/null | wc -l | tr -d ' '); \
	PARTIAL=$$(awk -F'\t' '{ pct = $$NF; gsub(/%/, "", pct); if (pct+0 < 80 && pct+0 > 0) print $$0 }' func-coverage.txt 2>/dev/null | wc -l | tr -d ' '); \
	echo "  Total coverage: $$TOTAL"; \
	echo "  Uncovered functions (0%): $$UNCOVERED"; \
	echo "  Partially covered (< 80%): $$PARTIAL"
	@echo ""
	@if command -v gremlins >/dev/null 2>&1; then \
		echo "── Mutation Testing ──"; \
		gremlins unleash --tags="" ./pkg/... 2>&1 | tee mutation-report.txt || true; \
		KILLED=$$(grep -c 'KILLED' mutation-report.txt 2>/dev/null || echo "0"); \
		SURVIVED=$$(grep -c 'SURVIVED' mutation-report.txt 2>/dev/null || echo "0"); \
		TOTAL_MUT=$$((KILLED + SURVIVED)); \
		if [ "$$TOTAL_MUT" -gt 0 ]; then \
			SCORE=$$((KILLED * 100 / TOTAL_MUT)); \
		else \
			SCORE=0; \
		fi; \
		echo ""; \
		echo "  Mutation score: $$SCORE% ($$KILLED killed, $$SURVIVED survived)"; \
	else \
		echo "── Mutation Testing ──"; \
		echo "  Skipped (install gremlins: go install github.com/go-gremlins/gremlins/cmd/gremlins@latest)"; \
	fi
	@echo ""
	@echo "=== Report complete ==="
	@rm -f func-coverage.txt

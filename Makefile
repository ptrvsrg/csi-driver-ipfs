# Copyright 2026 ptrvsrg.
#
# Licensed under the Apache License, Version 2.0 (the License);
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an 'AS IS' BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

PROJECT_NAME := csi-driver-ipfs
REGISTRY ?= ghcr.io/ptrvsrg
IMAGE_NAME := $(REGISTRY)/$(PROJECT_NAME)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

## Location to install dependencies to
GO_BIN ?= $(shell pwd)/bin
YARN_BIN ?= $(shell pwd)/bin

$(GOBIN):
	mkdir -p $(GOBIN)

$(YARNBIN):
	mkdir -p $(YARN_BIN)

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } /^[^[:space:]]+:.*##/ { gsub(/^[ \t]+|[ \t]+$$/, "", $$2); printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2 } ' $(MAKEFILE_LIST)

.PHONY: help/all
help/all: help ## Show help for root and nested Makefiles.
	@echo ""
	@echo "\033[1mGitHub Makefile (.github)\033[0m"
	@$(MAKE) -C .github help
	@echo ""
	@echo "\033[1mCharts Makefile (charts)\033[0m"
	@$(MAKE) -C charts help
	@echo ""
	@echo "\033[1mE2E Makefile (test/e2e)\033[0m"
	@$(MAKE) -C test/e2e help
	@echo ""
	@echo "\033[1mDocs Makefile (docs)\033[0m"
	@$(MAKE) -C docs help

##@ Dependency

GOLANGCI_LINT_VERSION ?= 2.10.1
.PHONY: deps/golangci-lint
deps/golangci-lint: ## Install golangci-lint into $(GOBIN) if missing.
	$(call go_install_if_not_exists,golangci-lint,github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(GOLANGCI_LINT_VERSION))

MOCKERY_VERSION ?= 2.53.6
.PHONY: deps/mockery
deps/mockery: ## Install mockery into $(GOBIN) if missing.
	$(call go_install_if_not_exists,mockery,github.com/vektra/mockery/v2@v$(MOCKERY_VERSION))

MARKDOWN_LINT_VERSION ?= 0.22.0
.PHONY: deps/markdownlint
deps/markdownlint: ## Install markdownlint-cli2 into $(YARN_BIN) if missing.
	$(call yarn_install_if_not_exists,markdownlint-cli2,markdownlint-cli2@$(MARKDOWN_LINT_VERSION))

EDITORCONFIG_CHECKER_VERSION ?= 6.1.1
.PHONY: deps/editorconfig-checker
deps/editorconfig-checker: ## Install editorconfig-checker into $(YARN_BIN) if missing.
	$(call yarn_install_if_not_exists,editorconfig-checker,editorconfig-checker@$(EDITORCONFIG_CHECKER_VERSION))

.PHONY: deps/mod-tidy
deps/mod-tidy: ## Run go mod tidy.
	go mod tidy

.PHONY: deps/mod-download
deps/mod-download: ## Download go module dependencies.
	go mod download

.PHONY: deps/all
deps/all: deps/mod-download deps/editorconfig-checker deps/markdownlint deps/mockery deps/golangci-lint ## Download all dependencies.

##@ Generating

.PHONY: gen/license-header
gen/license-header: ## Add copyright to source files.
	$(PWD)/hack/generate-license-header.sh

.PHONY: gen/mocks
gen/mocks: deps/mockery ## Generate Go mocks using .mockery.yaml.
	$(GOBIN)/mockery --config .mockery.yaml

##@ Testing

.PHONY: test/unit
test/unit: ## Run tests.
	go test -v -count=1 ./...

.PHONY: test/unit-race
test/unit-race: ## Run tests with race detector.
	go test -v -race -count=1 ./...

.PHONY: test/unit-coverage
test/unit-coverage: ## Run tests with coverage report.
	go test -v -count=1 -coverprofile=coverage.out -covermode=atomic ./...
	@echo "Coverage report: coverage.out"
	@go tool cover -func=coverage.out | tail -1

.PHONY: test/unit-coverage-html
test/unit-coverage-html: test/unit-coverage ## Generate HTML coverage report.
	go tool cover -html=coverage.out -o coverage.html
	@echo "HTML report: coverage.html"

##@ Build

.PHONY: build/golang
build/golang: $(GOBIN) verify/fmt verify/vet ## Build binary file.
	CGO_ENABLED=0 go build \
		-ldflags "-s -w \
		-X main.driverVersion=$(VERSION) \
		-X main.gitCommit=$(GIT_COMMIT) \
		-X main.buildDate=$(BUILD_DATE)" \
		-o bin/$(PROJECT_NAME) \
		./cmd/$(PROJECT_NAME)/

.PHONY: build/docker
build/docker: ## Build docker image (single platform, visible in docker images).
	docker build \
	--build-arg VERSION=$(VERSION) \
	--build-arg GIT_COMMIT=$(GIT_COMMIT) \
	--build-arg BUILD_DATE=$(BUILD_DATE) \
	--network=host \
	-t $(IMAGE_NAME):$(VERSION) \
	-t $(IMAGE_NAME):latest \
	.

.PHONY: build/docker-buildx
build/docker-buildx: ## Build and push multi-platform image; result in registry only (not in docker images).
	docker buildx build \
	--build-arg VERSION=$(VERSION) \
	--build-arg GIT_COMMIT=$(GIT_COMMIT) \
	--build-arg BUILD_DATE=$(BUILD_DATE) \
	--platform="linux/amd64,linux/arm64" \
	--network=host \
	-t $(IMAGE_NAME):$(VERSION) \
	-t $(IMAGE_NAME):latest \
	.

##@ Verification

.PHONY: verify/fmt
verify/fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: verify/editorconfig
verify/editorconfig:  deps/editorconfig-checker ## Verify files comply with .editorconfig.
	$(YARN_BIN)/editorconfig-checker

.PHONY: verify/vet
verify/vet: ## Run go vet against code.
	go vet ./...

.PHONY: verify/lint
verify/lint: deps/golangci-lint ## Run golangci-lint against code (uses .golangci.yaml).
	$(GO_BIN)/golangci-lint run --timeout 10m

.PHONY: verify/lint-fix
verify/lint-fix: deps/golangci-lint ## Run golangci-lint with --fix to auto-fix issues.
	$(GO_BIN)/golangci-lint run --fix --timeout 10m

.PHONY: verify/license-header
verify/license-header: ## Validate license headers in source files.
	$(PWD)/hack/validate-license-header.sh

.PHONY: verify/markdownlint
verify/markdownlint: deps/markdownlint ## Lint Markdown (globs/ignores in .markdownlint-cli2.jsonc; rules in .markdownlint.json).
	$(YARN_BIN)/markdownlint-cli2

.PHONY: verify/markdownlint-fix
verify/markdownlint-fix: deps/markdownlint ## Lint Markdown (globs/ignores in .markdownlint-cli2.jsonc; rules in .markdownlint.json).
	$(YARN_BIN)/markdownlint-cli2 --fix

.PHONY: verify/go-mod
verify/go-mod: ## Verify go module dependencies.
	go mod verify

.PHONY: verify/all
verify/all: verify/go-mod verify/fmt verify/vet verify/editorconfig verify/lint verify/license-header verify/markdownlint ## Verify modules and run all Go/static checks and Markdown.

##@ Cleaning

.PHONY: clean/all
clean/all: ## Remove build artifacts and generated files.
	rm -rf bin/
	rm -f coverage.out coverage.html

.DEFAULT_GOAL := help

define go_install_if_not_exists
	[ -e $(GO_BIN)/$(1) ] || GOBIN=$(GO_BIN) go install $(2)
endef

define yarn_install_if_not_exists
	[ -e $(YARN_BIN)/$(1) ] || yarn global add $(2) --prefix $(shell dirname $(YARN_BIN))
endef

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
VERSION_RAW ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
VERSION ?= $(patsubst driver/%,%,$(VERSION_RAW))
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

## Location to install dependencies to
GO_BIN ?= $(shell pwd)/bin
YARN_BIN ?= $(shell pwd)/bin
TRIVY_BIN := $(GO_BIN)/trivy
TRIVY_SEVERITY ?= HIGH,CRITICAL
TRIVY_IMAGE_IGNORE_FILE ?= .trivyignore.image.yaml
SHELLCHECK_BIN ?= shellcheck
SHELL_SCRIPTS := $(shell git ls-files '*.sh')
DOCKER_SCAN_IMAGE_NAME ?= $(IMAGE_NAME)
DOCKER_SCAN_IMAGE_TAG ?= $(subst /,-,$(VERSION))
HELM_BIN ?= helm
K8S_DEPLOY_APP_VERSION ?= $(shell sed -n 's/^appVersion: "\(.*\)"/\1/p' charts/csi-driver-ipfs/Chart.yaml | sed -n '1p')
K8S_DEPLOY_DIR ?= deploy/k8s/v$(K8S_DEPLOY_APP_VERSION)

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

GOSEC_VERSION ?= 2.25.0
.PHONY: deps/gosec
deps/gosec: ## Install gosec into $(GOBIN) if missing.
	$(call go_install_if_not_exists,gosec,github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION))

.PHONY: deps/govulncheck
deps/govulncheck: ## Install govulncheck into $(GOBIN) if missing.
	$(call go_install_if_not_exists,govulncheck,golang.org/x/vuln/cmd/govulncheck@latest)

.PHONY: deps/trivy
deps/trivy: ## Install trivy into $(GO_BIN) if missing.
	@if [ ! -x "$(TRIVY_BIN)" ]; then \
		curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b "$(GO_BIN)"; \
	fi

.PHONY: deps/shellcheck
deps/shellcheck: ## Verify shellcheck is available in PATH.
	@command -v "$(SHELLCHECK_BIN)" >/dev/null 2>&1 || { \
		echo "shellcheck is required but was not found in PATH."; \
		echo "Install it locally (for example: brew install shellcheck or apt-get install shellcheck)."; \
		exit 1; \
	}

.PHONY: deps/helm
deps/helm: ## Verify helm is available in PATH.
	@command -v "$(HELM_BIN)" >/dev/null 2>&1 || { \
		echo "helm is required but was not found in PATH."; \
		echo "Install it locally (for example: brew install helm)."; \
		exit 1; \
	}

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

.PHONY: deps/security
deps/security: deps/gosec deps/govulncheck deps/trivy deps/shellcheck ## Install local security scanners used by Make/CI.

##@ Generating

.PHONY: gen/license-header
gen/license-header: ## Add copyright to source files.
	$(PWD)/hack/generate-license-header.sh

.PHONY: gen/mocks
gen/mocks: deps/mockery ## Generate Go mocks using .mockery.yaml.
	$(GOBIN)/mockery --config .mockery.yaml

.PHONY: gen/k8s-manifests
gen/k8s-manifests: deps/helm ## Render Helm charts into deploy/k8s/v<appVersion>.
	@mkdir -p "$(K8S_DEPLOY_DIR)"
	$(HELM_BIN) dependency update charts/csi-driver-ipfs
	$(HELM_BIN) dependency update charts/ipfs-cluster
	$(HELM_BIN) template csi-driver-ipfs charts/csi-driver-ipfs --namespace csi-ipfs > "$(K8S_DEPLOY_DIR)/csi-driver-ipfs.yaml"
	$(HELM_BIN) template ipfs-cluster charts/ipfs-cluster --namespace ipfs > "$(K8S_DEPLOY_DIR)/ipfs-cluster.yaml"
	@echo "Rendered manifests to $(K8S_DEPLOY_DIR)"

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

##@ Security

.PHONY: security/golang-code-scan
security/golang-code-scan: deps/gosec ## Run gosec against Go source code.
	$(GO_BIN)/gosec -exclude-generated ./...

.PHONY: security/golang-deps-scan
security/golang-deps-scan: deps/trivy ## Run Trivy against Go module dependencies.
	$(TRIVY_BIN) fs --quiet --scanners vuln --severity $(TRIVY_SEVERITY) --exit-code 1 --skip-dirs docs .

.PHONY: security/docs-deps-scan
security/docs-deps-scan: deps/trivy ## Run Trivy against docs/yarn dependencies.
	$(TRIVY_BIN) fs --quiet --scanners vuln --severity $(TRIVY_SEVERITY) --exit-code 1 docs

.PHONY: security/dockerfile-scan
security/dockerfile-scan: deps/trivy ## Run Trivy config checks against the Dockerfile.
	$(TRIVY_BIN) config --quiet --severity $(TRIVY_SEVERITY) --exit-code 1 Dockerfile

.PHONY: security/docker-image-scan
security/docker-image-scan: deps/trivy ## Build and scan the local Docker image with Trivy.
	$(MAKE) build/docker IMAGE_NAME=$(DOCKER_SCAN_IMAGE_NAME) VERSION=$(DOCKER_SCAN_IMAGE_TAG)
	$(TRIVY_BIN) image --quiet --severity $(TRIVY_SEVERITY) --ignorefile $(TRIVY_IMAGE_IGNORE_FILE) --exit-code 1 $(DOCKER_SCAN_IMAGE_NAME):$(DOCKER_SCAN_IMAGE_TAG)

.PHONY: security/shell-scripts-scan
security/shell-scripts-scan: deps/shellcheck ## Run shellcheck against tracked shell scripts.
	@if [ -z "$(SHELL_SCRIPTS)" ]; then \
		echo "No tracked shell scripts found."; \
	else \
		"$(SHELLCHECK_BIN)" -x $(SHELL_SCRIPTS); \
	fi

.PHONY: security/all
security/all: security/golang-code-scan security/golang-deps-scan security/docs-deps-scan security/dockerfile-scan security/docker-image-scan security/shell-scripts-scan ## Run all configured security checks.

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

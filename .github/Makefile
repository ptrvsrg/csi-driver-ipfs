# Copyright 2026 ptrvsrg.
#
# Licensed under the Apache License, Version 2.0 (the License);
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GITHUB_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
BIN := $(GITHUB_DIR)/bin

WORKFLOWS := $(wildcard $(GITHUB_DIR)/workflows/*.yml)

.DEFAULT_GOAL: help

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\n"} /^[^[:space:]]+:.*?##/ { printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Dependency

.deps/make-bin-dir:
	mkdir -p $(BIN)

ACTIONLINT_VERSION ?= v1.7.7
.PHONY: deps/actionlint
deps/actionlint: .deps/make-bin-dir ## Install actionlint into .github/bin.
	GOBIN=$(BIN) go install github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)

##@ Verification

.PHONY: verify/actionlint
verify/actionlint: deps/actionlint ## Lint GitHub Actions workflow YAML files.
	$(BIN)/actionlint -color $(WORKFLOWS)

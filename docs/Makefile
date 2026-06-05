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

.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Show docs Makefile help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make -C docs \033[36m<target>\033[0m\n"} /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } /^[^[:space:]]+:.*##/ { gsub(/^[ \t]+|[ \t]+$$/, "", $$2); printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## Remove generated docs artifacts.
	rm -rf build .docusaurus node_modules

##@ Dependency

.PHONY: deps/install
deps/install: ## Install Docusaurus dependencies.
	yarn install --frozen-lockfile

##@ Development

.PHONY: dev/start
dev/start: deps/install ## Run local Docusaurus dev server.
	yarn run start

.PHONY: dev/build
dev/build: deps/install ## Build static docs site to docs/build.
	yarn run build

##@ Security

.PHONY: security/audit
security/audit: deps/install ## Audit docs site runtime dependencies for known vulnerabilities.
	yarn npm audit

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

# Charts list — add a name here; per-chart targets are generated below (see CHART_RULES_*).
CHARTS := csi-driver-ipfs ipfs-cluster

CHART_DIR := $(CURDIR)
DIST_DIR := $(CHART_DIR)/dist
BIN_DIR := $(CHART_DIR)/bin
CR_CONFIG := $(CHART_DIR)/cr.yaml

# Descriptions for per-chart targets (shown in `make help`; rules are generated via CHART_RULES_*).
CHART_DESC_TEST := helm unittest (this chart only)
CHART_DESC_LINT := helm lint (this chart only)
CHART_DESC_HELM_SECURITY := trivy config scan for chart sources (this chart only)
CHART_DESC_MANIFEST_SECURITY := trivy config scan for rendered manifests (this chart only)
CHART_DESC_PACKAGE := cr package into dist/ (this chart only)
CHART_DESC_RELEASE := package then cr upload (all *.tgz in dist/)

define CHART_RULES_TEST
.PHONY: test/$(1)
test/$(1): deps/unittest
	helm unittest $(CHART_DIR)/$(1)

.PHONY: lint/$(1)
lint/$(1):
	helm lint $(CHART_DIR)/$(1)
endef

define CHART_RULES_RELEASE
.PHONY: release/package/$(1)
release/package/$(1): deps/cr
	@mkdir -p $(DIST_DIR)
	$(BIN_DIR)/cr package -p $(DIST_DIR) $(CHART_DIR)/$(1)

.PHONY: release/upload/$(1)
release/upload/$(1): release/package/$(1)
	@if [ -z "$$CR_TOKEN" ] && [ -z "$$GITHUB_TOKEN" ]; then \
		echo 'Error: set CR_TOKEN or GITHUB_TOKEN in the environment to upload releases.'; \
		echo 'Note: cr upload sends all *.tgz in $(DIST_DIR)/'; \
		exit 1; \
	fi
	$(BIN_DIR)/cr upload --config $(CR_CONFIG) --skip-existing
endef

define CHART_RULES_SECURITY
.PHONY: security/helm/$(1)
security/helm/$(1): deps/trivy
	$(BIN_DIR)/trivy config --quiet --severity HIGH,CRITICAL --exit-code 1 --ignorefile $(CHART_DIR)/$(1)/.trivyignore "$(CHART_DIR)/$(1)"

.PHONY: security/manifests/$(1)
security/manifests/$(1): deps/trivy
	@mkdir -p $(DIST_DIR)
	helm dependency update "$(CHART_DIR)/$(1)"
	helm template "$(1)" "$(CHART_DIR)/$(1)" > "$(DIST_DIR)/$(1)-rendered.yaml"
	$(BIN_DIR)/trivy config --quiet --severity HIGH,CRITICAL --exit-code 1 --ignorefile $(CHART_DIR)/$(1)/.trivyignore "$(DIST_DIR)/$(1)-rendered.yaml"
endef

.DEFAULT_GOAL := help

##@ General

# awk only sees static lines; generated rules use $$(1) — print per-chart lines here (same text as CHART_DESC_*).
.PHONY: help/per-chart-test
help/per-chart-test:
	@for c in $(CHARTS); do \
		printf '  \033[36m%-28s\033[0m  [%s] $(CHART_DESC_TEST)\n' "test/$$c" "$$c"; \
		printf '  \033[36m%-28s\033[0m  [%s] $(CHART_DESC_LINT)\n' "lint/$$c" "$$c"; \
	done

.PHONY: help/per-chart-security
help/per-chart-security:
	@for c in $(CHARTS); do \
		printf '  \033[36m%-28s\033[0m  [%s] $(CHART_DESC_HELM_SECURITY)\n' "security/helm/$$c" "$$c"; \
		printf '  \033[36m%-28s\033[0m  [%s] $(CHART_DESC_MANIFEST_SECURITY)\n' "security/manifests/$$c" "$$c"; \
	done

.PHONY: help/per-chart-release
help/per-chart-release:
	@for c in $(CHARTS); do \
		printf '  \033[36m%-28s\033[0m  [%s] $(CHART_DESC_PACKAGE)\n' "release/package/$$c" "$$c"; \
		printf '  \033[36m%-28s\033[0m  [%s] $(CHART_DESC_RELEASE)\n' "release/upload/$$c" "$$c"; \
	done

.PHONY: help
help: ## Show this help (Test / Release; per-chart targets are listed under each section).
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^##@ Release/ { exit } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } /^[^[:space:]]+:.*?##/ { printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@$(MAKE) -s help/per-chart-test
	@echo ""
	@echo "\033[1mSecurity\033[0m"
	@$(MAKE) -s help/per-chart-security
	@awk 'BEGIN {FS = ":.*##";} /^##@ Release/ { f=1; printf "\n\033[1m%s\033[0m\n", substr($$0, 5); next } f && /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5); next } f && /^[^[:space:]]+:.*?##/ { printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@$(MAKE) -s help/per-chart-release
	@echo ''
	@echo "CHARTS: $(CHARTS)"

clean: ## Remove $(DIST_DIR) and downloaded $(CR_BIN).
	rm -rf $(DIST_DIR)
	rm -f $(CR_BIN)

##@ Dependency

.deps/make-bin-dir:
	mkdir -p $(BIN_DIR)

.PHONY: deps/unittest
deps/unittest: ## Install helm-unittest plugin if missing.
	@if ! helm plugin list 2>/dev/null | awk 'NR>1 {print $$1}' | grep -qx 'unittest'; then \
		helm plugin install 'https://github.com/helm-unittest/helm-unittest.git' --verify=false; \
	fi

CR_VERSION ?= 1.8.1
CR_OS ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
CR_ARCH ?= $(shell uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/' -e 's/arm64/arm64/')
CR_URL := https://github.com/helm/chart-releaser/releases/download/v$(CR_VERSION)/chart-releaser_$(CR_VERSION)_$(CR_OS)_$(CR_ARCH).tar.gz
.PHONY: deps/cr
deps/cr: .deps/make-bin-dir ## Ensure chart-releaser (cr) is available (PATH or download into $(BIN_DIR)).
	@if [ ! -x "$(BIN_DIR)/cr" ]; then \
		curl -sSL -o $(BIN_DIR)/cr.tgz '$(CR_URL)'; \
		tar -xzf $(BIN_DIR)/cr.tgz -C $(BIN_DIR) cr; \
		rm -f $(BIN_DIR)/cr.tgz; \
		chmod +x $(BIN_DIR)/cr; \
	fi

.PHONY: deps/trivy
deps/trivy: .deps/make-bin-dir ## Ensure trivy is available in $(BIN_DIR).
	@if [ ! -x "$(BIN_DIR)/trivy" ]; then \
		curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b "$(BIN_DIR)"; \
	fi

##@ Test

$(foreach c,$(CHARTS),$(eval $(call CHART_RULES_TEST,$(c))))
$(foreach c,$(CHARTS),$(eval $(call CHART_RULES_SECURITY,$(c))))

.PHONY: test/all
test/all: $(addprefix test/,$(CHARTS)) ## Run helm unittest for every chart.

.PHONY: lint/all
lint/all: $(addprefix lint/,$(CHARTS)) ## Run helm lint for every chart.

##@ Security

.PHONY: security/helm/all
security/helm/all: $(addprefix security/helm/,$(CHARTS)) ## Run Trivy config scan for every chart source.

.PHONY: security/manifests/all
security/manifests/all: $(addprefix security/manifests/,$(CHARTS)) ## Run Trivy config scan for every rendered chart manifest.

##@ Release

$(foreach c,$(CHARTS),$(eval $(call CHART_RULES_RELEASE,$(c))))

.PHONY: release/package/all
release/package/all: $(addprefix release/package/,$(CHARTS)) ## Package all charts to dist/.

.PHONY: release/upload/all
release/upload/all: release/package/all ## Upload all *.tgz in $(DIST_DIR) to GitHub Releases.
	@if [ -z "$$CR_TOKEN" ] && [ -z "$$GITHUB_TOKEN" ]; then \
		echo 'Error: set CR_TOKEN or GITHUB_TOKEN in the environment to upload releases.'; \
		echo 'Example: CR_TOKEN=$$(gh auth token) make release'; \
		exit 1; \
	fi
	$(BIN_DIR)/cr upload --config $(CR_CONFIG) --skip-existing
	@echo 'Upload complete. Run "make index" to refresh Helm repo index (gh-pages) if you use that flow.'

release/index: deps/cr release/package/all ## Push Helm index to gh-pages (cr index --push; needs token).
	@if [ -z "$$CR_TOKEN" ] && [ -z "$$GITHUB_TOKEN" ]; then \
		echo 'Error: set CR_TOKEN or GITHUB_TOKEN for cr index --push.'; \
		exit 1; \
	fi
	$(BIN_DIR)/cr index --config $(CR_CONFIG) --package-path $(DIST_DIR) --push

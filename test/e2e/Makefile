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

# Run from this directory or via `make -C test/e2e <target>` from repo root.
REPO_ROOT := $(abspath $(CURDIR)/../..)
CHARTS_DIR := $(REPO_ROOT)/charts
E2E_ENV_DIR := $(CURDIR)/env
BIN_DIR := $(REPO_ROOT)/bin
E2E_MOD_DIR := $(CURDIR)

PROJECT_NAME := csi-driver-ipfs
REGISTRY ?= ghcr.io/ptrvsrg
IMAGE_NAME := $(REGISTRY)/$(PROJECT_NAME)
IMAGE_GIT_COMMIT := $(shell git -C $(REPO_ROOT) rev-parse --short HEAD 2>/dev/null || echo "unknown")
IPFS_KUBO_IMAGE_NAME := ipfs/kubo:v0.40.1
IPFS_CLUSTER_IMAGE_NAME := ipfs/ipfs-cluster:v1.1.5

KIND_CLUSTER ?= csi-ipfs-e2e
KIND_CONTEXT ?= kind-$(KIND_CLUSTER)
E2E_NAMESPACE ?= default
SNAPSHOTTER_CRDS_VERSION ?= v8.5.0
SNAPSHOTTER_CRDS_BASE_URL ?= https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$(SNAPSHOTTER_CRDS_VERSION)/client/config/crd
SNAPSHOTTER_DEPLOY_BASE_URL ?= https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$(SNAPSHOTTER_CRDS_VERSION)/deploy/kubernetes
GINKGO_VERSION ?= v2.28.1
KUTTL_VERSION ?= 0.25.0
KUTTL_TIMEOUT ?= 240
KUTTL_ENABLED ?= 1
GINKGO_FOCUS ?=

.DEFAULT_GOAL := help

##@ Help

.PHONY: help
help: ## Show e2e-related targets (from repo root: make -C test/e2e <target>).
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m (from test/e2e or: make -C test/e2e <target>)\n"} /^[^[:space:]]+:.*?##/ { printf "  \033[36m%-30s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Dependency

.PHONY: deps/helm
deps/helm: ## Ensure helm is installed (https://helm.sh/).
	@command -v helm >/dev/null 2>&1 || { echo 'helm is required for e2e-env targets'; exit 1; }

.PHONY: deps/kind
deps/kind: ## Ensure kind is installed (https://kind.sigs.k8s.io/).
	@command -v kind >/dev/null 2>&1 || { echo 'kind is required for e2e-env-up'; exit 1; }

.PHONY: deps/ginkgo
deps/ginkgo: ## Install ginkgo CLI into repository bin/.
	mkdir -p $(BIN_DIR)
	cd $(E2E_MOD_DIR) && GOBIN=$(BIN_DIR) go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

.PHONY: deps/kuttl
deps/kuttl: ## Install kubectl-kuttl plugin into repository bin/.
	mkdir -p $(BIN_DIR)
	cd $(E2E_MOD_DIR) && GOBIN=$(BIN_DIR) go install github.com/kudobuilder/kuttl/cmd/kubectl-kuttl@v$(KUTTL_VERSION)

##@ E2E tests

.PHONY: test/ginkgo
test/ginkgo: deps/ginkgo ## Run Ginkgo suite tests in test/e2e/suites.
	cd $(E2E_MOD_DIR) && E2E_RUN=1 $(BIN_DIR)/ginkgo -v -r --keep-going --timeout=20m ./suites/... $(if $(GINKGO_FOCUS),--focus="$(GINKGO_FOCUS)",)

.PHONY: test/kuttl
test/kuttl: deps/kuttl ## Run KUTTL declarative assertions in test/e2e/kuttl.
	@if [ "$(KUTTL_ENABLED)" != "1" ]; then \
		echo "KUTTL is disabled (set KUTTL_ENABLED=1 to run)"; \
		exit 0; \
	fi
	kubectl config use-context $(KIND_CONTEXT)
	cd $(REPO_ROOT) && E2E_RUN=1 $(BIN_DIR)/kubectl-kuttl test --config ./test/e2e/kuttl/kuttl-test.yaml --namespace $(E2E_NAMESPACE) --timeout $(KUTTL_TIMEOUT)

.PHONY: test/e2e
test/e2e: test/ginkgo test/kuttl ## Run all e2e suites (Ginkgo + KUTTL).

##@ E2E environment (Kind + ipfs-cluster + csi-driver-ipfs)

.PHONY: env/up
env/up: deps/kind deps/helm env/down ## Create Kind cluster and deploy ipfs-cluster + csi-driver-ipfs.
	kind create cluster --name $(KIND_CLUSTER) --config $(E2E_ENV_DIR)/kind-config.yaml
	kubectl --context $(KIND_CONTEXT) wait --for=condition=Ready nodes --all --timeout=120s
	$(MAKE) env/install-snapshot-crds
	$(MAKE) env/deploy-ipfs env/deploy-csi
	$(MAKE) env/install-snapshot-controller

.PHONY: env/down
env/down: ## Delete Kind e2e cluster.
	kind delete cluster --name $(KIND_CLUSTER) 2>/dev/null || true

.PHONY: env/deploy-ipfs
env/deploy-ipfs: deps/helm ## Install ipfs-cluster chart (single peer) in e2e cluster.
	kubectl create ns ipfs 2>/dev/null || true
	helm upgrade --install ipfs-cluster $(CHARTS_DIR)/ipfs-cluster \
		-f $(E2E_ENV_DIR)/ipfs-values.yaml -n ipfs --wait --timeout=600s
	kubectl --context $(KIND_CONTEXT) wait --for=condition=ready pod -l app.kubernetes.io/name=ipfs-cluster -n ipfs --timeout=120s

.PHONY: env/deploy-csi
env/deploy-csi: deps/helm env/install-snapshot-crds env/load-image ## Install csi-driver-ipfs chart in e2e cluster.
	kubectl create ns csi-ipfs 2>/dev/null || true
	helm upgrade --install csi-driver-ipfs $(CHARTS_DIR)/csi-driver-ipfs \
		-f $(E2E_ENV_DIR)/csi-values.yaml -n csi-ipfs --wait --timeout=600s
	kubectl --context $(KIND_CONTEXT) wait --for=condition=ready pod -l app=csi-ipfs-controller -n csi-ipfs --timeout=120s
	kubectl --context $(KIND_CONTEXT) wait --for=condition=ready pod -l app=csi-ipfs-node -n csi-ipfs --timeout=120s

.PHONY: env/install-snapshot-crds
env/install-snapshot-crds: ## Install/refresh external-snapshotter CRDs required by VolumeSnapshotClass.
	kubectl --context $(KIND_CONTEXT) apply -f $(SNAPSHOTTER_CRDS_BASE_URL)/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
	kubectl --context $(KIND_CONTEXT) apply -f $(SNAPSHOTTER_CRDS_BASE_URL)/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
	kubectl --context $(KIND_CONTEXT) apply -f $(SNAPSHOTTER_CRDS_BASE_URL)/snapshot.storage.k8s.io_volumesnapshots.yaml
	kubectl --context $(KIND_CONTEXT) apply -f $(SNAPSHOTTER_CRDS_BASE_URL)/groupsnapshot.storage.k8s.io_volumegroupsnapshotclasses.yaml
	kubectl --context $(KIND_CONTEXT) apply -f $(SNAPSHOTTER_CRDS_BASE_URL)/groupsnapshot.storage.k8s.io_volumegroupsnapshotcontents.yaml
	kubectl --context $(KIND_CONTEXT) apply -f $(SNAPSHOTTER_CRDS_BASE_URL)/groupsnapshot.storage.k8s.io_volumegroupsnapshots.yaml

.PHONY: env/install-snapshot-controller
env/install-snapshot-controller: env/install-snapshot-crds ## Install/refresh snapshot-controller deployment for all namespaces.
	kubectl --context $(KIND_CONTEXT) apply -f $(SNAPSHOTTER_DEPLOY_BASE_URL)/snapshot-controller/rbac-snapshot-controller.yaml
	kubectl --context $(KIND_CONTEXT) apply -f $(SNAPSHOTTER_DEPLOY_BASE_URL)/snapshot-controller/setup-snapshot-controller.yaml
	kubectl --context $(KIND_CONTEXT) rollout status deployment/snapshot-controller -n kube-system --timeout=180s

.PHONY: env/load-image
env/load-image: ## Build driver image and load into e2e Kind cluster.
	@image_commit="$$(docker image inspect $(IMAGE_NAME):latest --format '{{ index .Config.Labels "org.opencontainers.image.commit" }}' 2>/dev/null || true)"; \
	if [ "$$image_commit" = "$(IMAGE_GIT_COMMIT)" ]; then \
		echo "Skipping docker build: $(IMAGE_NAME):latest already has commit $(IMAGE_GIT_COMMIT)"; \
	else \
		echo "Building docker image: expected commit $(IMAGE_GIT_COMMIT), found '$$image_commit'"; \
		$(MAKE) -C $(REPO_ROOT) build/docker; \
	fi
	kind load docker-image $(IMAGE_NAME):latest --name $(KIND_CLUSTER)

.PHONY: env/status
env/status: ## Show pods, PVCs and PVs in e2e cluster.
	kubectl --context $(KIND_CONTEXT) get pods -n $(E2E_NAMESPACE) -o wide
	kubectl --context $(KIND_CONTEXT) get pvc -n $(E2E_NAMESPACE)
	kubectl --context $(KIND_CONTEXT) get pv


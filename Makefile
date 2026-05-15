ROOT_DIRECTORY := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

PROJECT_NAME := deployment-operator

IMAGE_REGISTRIES := ghcr.io
IMAGE_REPOSITORY := plural

ENVTEST ?= $(shell which setup-envtest)
CRDDOCS ?= $(shell which crd-ref-docs)

VELERO_CHART_VERSION := 5.2.2 # It should be kept in sync with Velero chart version from console/charts/velero
VELERO_CHART_URL := https://github.com/vmware-tanzu/helm-charts/releases/download/velero-$(VELERO_CHART_VERSION)/velero-$(VELERO_CHART_VERSION).tgz

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

ENVTEST_K8S_VERSION := 1.28.3
CONTROLLER_GEN ?= $(shell which controller-gen)
MOCKERY ?= $(shell which mockery)
include tools.mk

ifndef GOPATH
GOPATH := $(shell go env GOPATH)
endif

PRE = --ensure

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: crd-docs
crd-docs: tools ##generate docs from the CRDs
	$(CRDDOCS) --source-path=./api --renderer=markdown --output-path=./docs/api.md --config=config.yaml

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	@$(MAKE) -s codegen-chart-crds

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: genmock
genmock: mockery ## generates mocks before running tests
	$(MOCKERY)

.PHONY: codegen-chart-crds
codegen-chart-crds: ## copy CRDs to the helm chart
	@cp -a config/crd/bases/. charts/deployment-operator/crds

.PHONY: velero-crds
velero-crds: ## download velero CRDs
	@curl -L $(VELERO_CHART_URL) --output velero.tgz
	@tar zxvf velero.tgz velero/crds
	@mv velero/crds/* charts/deployment-operator/crds
	@rm -r velero.tgz velero

##@ Release

release-vsn: ## tags and pushes a new release
	@read -p "Version: " tag; \
	git checkout main; \
	git pull --rebase; \
	git tag -a $$tag -m "new release"; \
	git push origin $$tag

delete-tag:  ## deletes a tag from git locally and upstream
	@read -p "Version: " tag; \
	git tag -d $$tag; \
	git push origin :$$tag

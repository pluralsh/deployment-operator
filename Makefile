SHELL := /bin/bash

CHART_PATH := charts/deployment-operator
VELERO_CHART_VERSION := 5.2.2
VELERO_CHART_URL := https://github.com/vmware-tanzu/helm-charts/releases/download/velero-$(VELERO_CHART_VERSION)/velero-$(VELERO_CHART_VERSION).tgz

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make <target>\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  %-22s %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Helm Chart

.PHONY: helm-lint
helm-lint: ## Run helm lint for deployment-operator chart
	helm lint $(CHART_PATH)

.PHONY: helm-template
helm-template: ## Render chart templates with required test values
	helm template deployment-operator-test $(CHART_PATH) \
		--set secrets.deployToken=test-token \
		--set fullnameOverride=deployment-operator-test > /dev/null

.PHONY: helm-test-install
helm-test-install: ## Validate chart installation using Kind
	./test/helm/test-chart-install.sh

.PHONY: velero-crds
velero-crds: ## Download Velero CRDs into chart CRD directory
	@curl -L $(VELERO_CHART_URL) --output velero.tgz
	@tar zxvf velero.tgz velero/crds
	@mv velero/crds/* $(CHART_PATH)/crds
	@rm -r velero.tgz velero

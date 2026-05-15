SHELL := /bin/bash

CHART_PATH := charts/deployment-operator
CONSOLE_REPO_TARBALL_URL := https://codeload.github.com/pluralsh/console/tar.gz/refs/heads/master
CONSOLE_CRD_ARCHIVE_PATH := console-master/go/deployment-operator/config/crd/bases
VELERO_CHART_VERSION := 5.2.2
VELERO_CHART_URL := https://github.com/vmware-tanzu/helm-charts/releases/download/velero-$(VELERO_CHART_VERSION)/velero-$(VELERO_CHART_VERSION).tgz

##@ General

.PHONY: help
help: ## display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make <target>\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  %-22s %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Helm Chart

.PHONY: helm-lint
helm-lint: ## run helm lint for deployment-operator chart
	helm lint $(CHART_PATH)

.PHONY: helm-template
helm-template: ## render chart templates with required test values
	helm template deployment-operator-test $(CHART_PATH) \
		--set secrets.deployToken=test-token \
		--set fullnameOverride=deployment-operator-test > /dev/null

.PHONY: helm-test-install
helm-test-install: ## validate chart installation using kind
	./test/helm/test-chart-install.sh

.PHONY: codegen-chart-crds
codegen-chart-crds: ## sync deployment-operator CRDs from console repository
	@tmp_dir=$$(mktemp -d); \
	trap 'rm -rf "$$tmp_dir"' EXIT; \
	curl -sSL "$(CONSOLE_REPO_TARBALL_URL)" --output "$$tmp_dir/console.tar.gz"; \
	tar -xzf "$$tmp_dir/console.tar.gz" -C "$$tmp_dir" "$(CONSOLE_CRD_ARCHIVE_PATH)"; \
	rm -f "$(CHART_PATH)"/crds/deployments.plural.sh_*.yaml; \
	cp -a "$$tmp_dir/$(CONSOLE_CRD_ARCHIVE_PATH)"/. "$(CHART_PATH)/crds/"

.PHONY: velero-crds
velero-crds: ## download Velero CRDs into chart CRD directory
	@curl -L $(VELERO_CHART_URL) --output velero.tgz
	@tar zxvf velero.tgz velero/crds
	@mv velero/crds/* $(CHART_PATH)/crds
	@rm -r velero.tgz velero

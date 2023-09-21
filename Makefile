ROOT_DIRECTORY := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
API_DIRECTORY = $(ROOT_DIRECTORY)/api
COMMON_DIRECTORY = $(ROOT_DIRECTORY)/common
OPERATOR_DIRECTORY = $(ROOT_DIRECTORY)/operator
PROVIDER_DIRECTORY = $(ROOT_DIRECTORY)/providers
ARGOCD_PROVIDER_DIRECTORY = $(PROVIDER_DIRECTORY)/argocd
FAKE_PROVIDER_DIRECTORY = $(PROVIDER_DIRECTORY)/fake
PROVISIONER_DIRECTORY = $(ROOT_DIRECTORY)/provisioner
TOOLS_DIRECTORY = $(ROOT_DIRECTORY)/tools
MODULES := $(API_DIRECTORY) $(COMMON_DIRECTORY) $(OPERATOR_DIRECTORY) $(ARGOCD_PROVIDER_DIRECTORY) $(FAKE_PROVIDER_DIRECTORY) $(PROVISIONER_DIRECTORY)

# List of targets that should be executed before other targets
PRE = --ensure-tools

.PHONY: --ensure-tools
--ensure-tools:
	@$(MAKE) --no-print-directory -C $(TOOLS_DIRECTORY) ensure

.PHONY: --run $(MODULES)
--run: $(MODULES)

$(MODULES):
	@$(MAKE) --directory=$@ $(TARGET)

##@ General

.PHONY: help
help: ## show help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

.PHONY: build
build: ## build all modules
	@$(MAKE) --no-print-directory -C $(ROOT_DIRECTORY) TARGET=build

.PHONY: build-api
build-api: ## build api module
	@$(MAKE) -C $(API_DIRECTORY) build

##@ Tests and checks

.PHONY: test
test: $(PRE) ## test workspace modules
	go work edit -json | jq -r '.Use[].DiskPath'  | xargs -I{} go test {}/...

.PHONY: lint
lint: $(PRE) ## lint workspace code
	go work edit -json | jq -r '.Use[].DiskPath'  | xargs -I{} golangci-lint run {}/...

.PHONY: fix
fix: $(PRE) ## fix workspace code
	go work edit -json | jq -r '.Use[].DiskPath'  | xargs -I{} golangci-lint run --fix {}/...
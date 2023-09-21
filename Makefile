ROOT_DIRECTORY := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
API_DIRECTORY = $(MODULES_DIRECTORY)/api
COMMON_DIRECTORY = $(MODULES_DIRECTORY)/common
OPERATOR_DIRECTORY = $(MODULES_DIRECTORY)/operator
PROVIDER_DIRECTORY = $(MODULES_DIRECTORY)/providers
ARGOCD_PROVIDER_DIRECTORY = $(PROVIDER_DIRECTORY)/argocd
FAKE_PROVIDER_DIRECTORY = $(PROVIDER_DIRECTORY)/fake
PROVISIONER_DIRECTORY = $(MODULES_DIRECTORY)/provisioner
TOOLS_DIRECTORY = $(MODULES_DIRECTORY)/tools
MODULES := $(API_DIRECTORY) $(COMMON_DIRECTORY) $(OPERATOR_DIRECTORY) $(ARGOCD_PROVIDER_DIRECTORY) $(FAKE_PROVIDER_DIRECTORY) $(PROVISIONER_DIRECTORY)

MAKEFLAGS += -j2

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
build: build-api ## build all modules

.PHONY: build-api
build-api: ## build API module
	@$(MAKE) -C $(API_DIRECTORY) build

##@ Tests and checks

# TODO: test target is not defined for all modules at the moment.
.PHONY: test
test: ## test all modules
	@$(MAKE) --no-print-directory -C $(MODULES_DIRECTORY) TARGET=test

.PHONY: lint
lint: $(PRE) ## lint all code
	go work edit -json | jq -r '.Use[].DiskPath'  | xargs -I{} golangci-lint run {}/...

.PHONY: fix
fix: $(PRE) ## fix all code
	go work edit -json | jq -r '.Use[].DiskPath'  | xargs -I{} golangci-lint run --fix {}/...
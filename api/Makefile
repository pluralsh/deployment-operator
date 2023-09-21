ROOT_DIRECTORY := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
TOOLS_DIRECTORY = $(ROOT_DIRECTORY)/../tools
CRD_OPTIONS ?= "crd"

# List of targets that should be executed before other targets
PRE = --ensure-tools

.PHONY: --ensure-tools
--ensure-tools:
	@$(MAKE) --no-print-directory -C $(TOOLS_DIRECTORY) ensure

##@ General

.PHONY: help
help: ## show help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

.PHONY: manifests
manifests: $(PRE) ## generate manifests
	controller-gen $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: $(PRE) ## generate code
	go generate ./pkg/...
	controller-gen object:headerFile="../boilerplate.go.txt" paths="./..."

.PHONY: build
build: manifests generate ## build module

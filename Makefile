ROOT_DIRECTORY := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

include $(ROOT_DIRECTORY)/config.mk

PRE = --ensure-tools

.PHONY: --ensure-tools
--ensure-tools:
	@$(MAKE) --no-print-directory -C $(TOOLS_DIRECTORY) ensure

.PHONY: --run $(MODULE_DIRECTORIES)
--run: $(MODULE_DIRECTORIES)

$(MODULE_DIRECTORIES):
	@$(MAKE) --directory=$@ $(TARGET)

##@ General

.PHONY: help
help: ## show help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

.PHONY: build
build: ## build all modules
	@$(MAKE) --no-print-directory -C $(MODULE_DIRECTORIES) build

##@ Tests and checks

.PHONY: test
test: $(PRE) ## test workspace modules
	go work edit -json | jq -r '.Use[].DiskPath'  | xargs -I{} go test {}/... -v

.PHONY: lint
lint: $(PRE) ## lint workspace code
	go work edit -json | jq -r '.Use[].DiskPath'  | xargs -I{} golangci-lint run --timeout=10m {}/...

.PHONY: fix
fix: $(PRE) ## fix workspace code
	go work edit -json | jq -r '.Use[].DiskPath'  | xargs -I{} golangci-lint run --timeout=10m --fix {}/...
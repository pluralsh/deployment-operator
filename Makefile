MODULES_DIRECTORY := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
API_DIRECTORY = $(MODULES_DIRECTORY)/api
COMMON_DIRECTORY = $(MODULES_DIRECTORY)/common
OPERATOR_DIRECTORY = $(MODULES_DIRECTORY)/operator
PROVIDER_DIRECTORY = $(MODULES_DIRECTORY)/providers
ARGOCD_PROVIDER_DIRECTORY = $(PROVIDER_DIRECTORY)/argocd
FAKE_PROVIDER_DIRECTORY = $(PROVIDER_DIRECTORY)/fake
PROVISIONER_DIRECTORY = $(MODULES_DIRECTORY)/provisioner
MODULES := $(API_DIRECTORY) $(COMMON_DIRECTORY) $(OPERATOR_DIRECTORY) $(ARGOCD_PROVIDER_DIRECTORY) $(FAKE_PROVIDER_DIRECTORY) $(PROVISIONER_DIRECTORY)

MAKEFLAGS += -j2

.PHONY: --run $(MODULES)
--run: $(MODULES)

$(MODULES):
	@$(MAKE) --directory=$@ $(TARGET)

##@ General

.PHONY: help
help: ## show help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

# TODO: build target is not defined for all modules at the moment.
.PHONY: build
build: ## build all modules
	@$(MAKE) --no-print-directory -C $(MODULES_DIRECTORY) TARGET=build

##@ Code quality

.PHONY: check
check: fmt vet lint ## run all code quality checks

.PHONY: fmt
fmt: ## format code
	@$(MAKE) --no-print-directory -C $(MODULES_DIRECTORY) TARGET=fmt

.PHONY: vet
vet: ## examine code to find potential errors and suspicious constructs
	@$(MAKE) --no-print-directory -C $(MODULES_DIRECTORY) TARGET=vet

# TODO: It doesn't seem to work when running with make. Should we remove vet since it already includes it?
.PHONY: lint
lint: ## lint code
	docker run -t --rm -v $$(pwd):/app -v ~/.cache/golangci-lint/v1.54.2:/root/.cache -w /app golangci/golangci-lint:v1.54.2 golangci-lint run -v --fix
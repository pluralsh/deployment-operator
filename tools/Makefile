JQ_BINARY := $(shell which jq)
GOLANGCI_LINT_BINARY := $(shell which golangci-lint)

##@ General

.PHONY: help
help: ## show help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Tools

.PHONY: ensure
ensure: ensure-jq ensure-golangci-lint ## ensure all tools availability

.PHONY: ensure-jq
ensure-jq: ## ensure jq availability
ifndef JQ_BINARY
	@echo "jq is required, see how to install it at: https://jqlang.github.io/jq/download/"
	@exit 1
else
	@echo "jq is already installed"
endif

.PHONY: ensure-golangci-lint
ensure-golangci-lint: ## ensure golangci-lint availability
ifndef GOLANGCI_LINT_BINARY
	@echo "installing golangci-lint..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint
else
	@echo "golangci-lint is already installed"
endif

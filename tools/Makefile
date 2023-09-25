JQ_BINARY := $(shell which jq)
GOLANGCI_LINT_BINARY := $(shell which golangci-lint)
PROTOC_GEN_GO_GRPC_BINARY := $(shell which protoc-gen-go-grpc)
PROTOC_GEN_GO_BINARY := $(shell which protoc-gen-go)
CONTROLLER_GEN_BINARY := $(shell which controller-gen)

##@ General

.PHONY: help
help: ## show help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Tools

.PHONY: ensure
ensure: ensure-jq ensure-golangci-lint ensure-protoc-gen-go-grpc ensure-protoc-gen-go ensure-controller-gen ## ensure all tools availability

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

.PHONY: ensure-protoc-gen-go-grpc
ensure-protoc-gen-go-grpc: ## ensure protoc-gen-go-grpc availability
ifndef PROTOC_GEN_GO_GRPC_BINARY
	@echo "installing protoc-gen-go-grpc..."
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
else
	@echo "protoc-gen-go-grpc is already installed"
endif

.PHONY: ensure-protoc-gen-go
ensure-protoc-gen-go: ## ensure protoc-gen-go availability
ifndef PROTOC_GEN_GO_BINARY
	@echo "installing protoc-gen-go..."
	@go install google.golang.org/grpc/cmd/protoc-gen-go
else
	@echo "protoc-gen-go is already installed"
endif


.PHONY: ensure-controller-gen
ensure-controller-gen: ## ensure controller-gen availability
ifndef CONTROLLER_GEN_BINARY
	@echo "installing controller-gen..."
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen
else
	@echo "controller-gen is already installed"
endif

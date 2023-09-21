DEPLOYMENT_PROTO := proto/deployment.proto

##@ General

.PHONY: help
help: ## show help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

build: build-proto ## build module

build-proto: ## parse proto files and generate output files
	protoc "$(DEPLOYMENT_PROTO)" \
        --go_out=. \
        --go_opt=paths=source_relative \
        --go-grpc_out=require_unimplemented_servers=false:. \
        --go-grpc_opt=paths=source_relative \
        --proto_path=.

# Image URL to use all building/pushing image targets
IMG ?= deployment-argocd-provider:latest

##@ General

help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

build:
	go build -o bin/deployment-argocd-provider cmd/driver/main.go cmd/driver/cmd.go
	go build -o bin/status-updater-argocd-provider cmd/status-updater/main.go

docker-build:
	docker build -t ${IMG} .

docker-push:
	docker push ${IMG}


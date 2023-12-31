ROOT_DIRECTORY := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

PROJECT_NAME := deployment-operator

IMAGE_REGISTRIES := ghcr.io
IMAGE_REPOSITORY := plural

IMG ?= deployment-agent:latest

ENVTEST ?= $(shell which setup-envtest)

ENVTEST_K8S_VERSION := 1.28.3

include tools.mk

ifndef GOPATH
$(error $$GOPATH environment variable not set)
endif

PRE = --ensure

##@ General

.PHONY: help
help: ## show help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Run

.PHONY: run
run: ## run
	go run cmd/main.go

##@ Build

.PHONY: build
build: ## build
	go build -o bin/deployment-agent cmd/main.go

docker-build: ## build image
	docker build -t ${IMG} .

docker-push: ## push image
	docker push ${IMG}

##@ Tests

.PHONY: test
test: envtest ## run tests
	@KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(GOPATH)/bin -p path)" go test $$(go list ./... | grep -v /e2e) -v

.PHONY: lint
lint: $(PRE) ## run linters
	golangci-lint run ./...

.PHONY: fix
fix: $(PRE) ## fix issues found by linters
	golangci-lint run --fix ./...

release-vsn: # tags and pushes a new release
	@read -p "Version: " tag; \
	git checkout main; \
	git pull --rebase; \
	git tag -a $$tag -m "new release"; \
	git push origin $$tag

delete-tag:  ## deletes a tag from git locally and upstream
	@read -p "Version: " tag; \
	git tag -d $$tag; \
	git push origin :$$tag


.PHONY: tools
tools: ## install required tools
tools: --tool

.PHONY: --tool
%--tool: TOOL = .*
--tool: # INTERNAL: installs tool with name provided via $(TOOL) variable or all tools.
	@cat tools.go | grep _ | awk -F'"' '$$2 ~ /$(TOOL)/ {print $$2}' | xargs -I {} go install {}

.PHONY: envtest
envtest: TOOL = setup-envtest
envtest: --tool ## Download and install setup-envtest in the $GOPATH/bin

.PHONY: mockery
mockery: TOOL = mockery
mockery: --tool

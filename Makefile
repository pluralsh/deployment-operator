ROOT_DIRECTORY := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

PROJECT_NAME := deployment-operator

IMAGE_REGISTRIES := ghcr.io
IMAGE_REPOSITORY := plural

IMG ?= deployment-agent:latest

ENVTEST ?= $(shell which setup-envtest)
CRDDOCS ?= $(shell which crd-ref-docs)

VELERO_CHART_VERSION := 5.2.2 # It should be kept in sync with Velero chart version from console/charts/velero
VELERO_CHART_URL := https://github.com/vmware-tanzu/helm-charts/releases/download/velero-$(VELERO_CHART_VERSION)/velero-$(VELERO_CHART_VERSION).tgz

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

ENVTEST_K8S_VERSION := 1.28.3
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
MOCKERY ?= $(shell which mockery)
include tools.mk

ifndef GOPATH
$(error $$GOPATH environment variable not set)
endif

PRE = --ensure

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.11.3)

.PHONY: crd-docs
crd-docs: ##generate docs from the CRDs
	$(CRDDOCS) --source-path=./api --renderer=markdown --output-path=./docs/api.md --config=config.yaml

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: genmock
genmock: mockery ## generates mocks before running tests
	$(MOCKERY)

##@ Run

.PHONY: agent-run
agent-run: ## run agent
	go run cmd/agent/**

##@ Build

.PHONY: agent
agent: ## build agent
	go build -o bin/deployment-agent cmd/agent/**

.PHONY: harness
harness: ## build stack run harness
	go build -o bin/stack-run-harness cmd/harness/main.go

docker-build: ## build image
	docker build -t ${IMG} .

docker-push: ## push image
	docker push ${IMG}

.PHONY: docker-build-harness-base
docker-build-harness-base: ## build base docker harness image
	docker build \
			--build-arg=VERSION="0.0.0-dev" \
    	  	-t harness-base \
    		-f dockerfiles/harness/base.Dockerfile \
    		.

.PHONY: docker-build-harness-terraform
docker-build-harness-terraform: docker-build-harness-base ## build terraform docker harness image
	docker build \
		  	--build-arg=HARNESS_IMAGE_TAG="latest" \
    	  	-t harness \
    		-f dockerfiles/harness/terraform.Dockerfile \
    		.

.PHONY: docker-build-harness-ansible
docker-build-harness-ansible: docker-build-harness-base ## build terraform docker harness image
	docker build \
		  	--build-arg=HARNESS_IMAGE_TAG="latest" \
    	  	-t harness \
    		-f dockerfiles/harness/ansible.Dockerfile \
    		.

.PHONY: docker-run-harness
docker-run-harness: docker-build-harness-terraform docker-build-harness-ansible ## build and run terraform docker harness image
	docker run \
			harness:latest \
			--v=5 \
			--console-url=${PLURAL_CONSOLE_URL}/ext/gql \
			--console-token=${PLURAL_DEPLOY_TOKEN} \
			--stack-run-id=${PLURAL_STACK_RUN_ID}

velero-crds:
	@curl -L $(VELERO_CHART_URL) --output velero.tgz
	@tar zxvf velero.tgz velero/crds
	@mv velero/crds/* charts/deployment-operator/crds
	@rm -r velero.tgz velero

##@ Tests

.PHONY: test
test: envtest ## run tests
	@KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(GOPATH)/bin -p path)" go test $$(go list ./... | grep -v /e2e) -v -vet=off

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

.PHONY: crd-ref-docs
crd-ref-docs: TOOL = crd-ref-docs
crd-ref-docs: --tool

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

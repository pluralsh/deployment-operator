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
CONTROLLER_GEN ?= $(shell which controller-gen)
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

.PHONY: crd-docs
crd-docs: tools ##generate docs from the CRDs
	$(CRDDOCS) --source-path=./api --renderer=markdown --output-path=./docs/api.md --config=config.yaml

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	@$(MAKE) -s codegen-chart-crds

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: genmock
genmock: mockery ## generates mocks before running tests
	$(MOCKERY)

.PHONY: codegen-chart-crds
codegen-chart-crds: ## copy CRDs to the helm chart
	@cp -a config/crd/bases/. charts/deployment-operator/crds

##@ Run

.PHONY: agent-run
agent-run: agent ## run agent
	OPERATOR_NAMESPACE=plrl-deploy-operator \
	go run cmd/agent/*.go \
		--console-url=${PLURAL_CONSOLE_URL}/ext/gql \
        --enable-helm-dependency-update=false \
        --disable-helm-dry-run-server=false \
        --cluster-id=${PLURAL_CLUSTER_ID} \
        --local \
        --refresh-interval=30s \
        --resource-cache-ttl=60s \
        --max-concurrent-reconciles=20 \
        --v=1 \
        --deploy-token=${PLURAL_DEPLOY_TOKEN}

##@ Build

.PHONY: agent
agent: ## build agent
	go build -o bin/deployment-agent cmd/agent/*.go

.PHONY: harness
harness: ## build stack run harness
	go build -o bin/stack-run-harness cmd/harness/main.go

.PHONY: agent-harness
agent-harness: ## build agent harness
	go build -o bin/agent-harness cmd/agent-harness/*.go

.PHONY: docker-build-agent-harness-base
docker-build-agent-harness-base: ## build base docker agent harness image
	docker build \
		--build-arg=VERSION="0.0.0-dev" \
		-t ghcr.io/pluralsh/agent-harness-base \
		-f dockerfiles/agent-harness/base.Dockerfile \
		.

.PHONY: docker-build-agent-harness-gemini
docker-build-agent-harness-gemini: docker-build-agent-harness-base ## build gemini docker agent harness image
	docker build \
		--build-arg=AGENT_HARNESS_BASE_IMAGE_TAG="latest" \
		-t ghcr.io/pluralsh/agent-harness-gemini \
		-f dockerfiles/agent-harness/gemini.Dockerfile \
		.

.PHONY: docker-build-agent-harness-claude
docker-build-agent-harness-claude: docker-build-agent-harness-base ## build claude docker agent harness image
	docker build \
		--build-arg=AGENT_HARNESS_BASE_IMAGE_TAG="latest" \
		-t ghcr.io/pluralsh/agent-harness-claude \
		-f dockerfiles/agent-harness/claude.Dockerfile \
		.

.PHONY: docker-build-agent-harness-opencode
docker-build-agent-harness-opencode: docker-build-agent-harness-base ## build opencode docker agent harness image
	docker build \
		--build-arg=AGENT_HARNESS_BASE_IMAGE_TAG="latest" \
		-t ghcr.io/pluralsh/agent-harness-opencode \
		-f dockerfiles/agent-harness/opencode.Dockerfile \
		.

.PHONY: docker-push-agent-harness-base  
docker-push-agent-harness-base: docker-build-agent-harness-base ## push agent harness base image
	docker push ghcr.io/pluralsh/agent-harness-base:latest

.PHONY: docker-push-agent-harness-gemini
docker-push-agent-harness-gemini: docker-build-agent-harness-gemini ## push gemini agent harness image
	docker push ghcr.io/pluralsh/agent-harness-gemini:latest

.PHONY: docker-push-agent-harness-claude
docker-push-agent-harness-claude: docker-build-agent-harness-claude ## push claude agent harness image
	docker push ghcr.io/pluralsh/agent-harness-claude:latest

.PHONY: docker-push-agent-harness-opencode
docker-push-agent-harness-opencode: docker-build-agent-harness-opencode ## push opencode agent harness image
	docker push ghcr.io/pluralsh/agent-harness-opencode:latest
	
docker-build: ## build image
	docker build -t ${IMG} .

docker-push: ## push image
	docker push ${IMG}

.PHONY: docker-build-harness-base-fips
docker-build-harness-base-fips: ## build fips base docker harness image
	docker build \
			--no-cache \
			--build-arg=VERSION="0.0.0-dev" \
    	  	-t harness-base-fips \
    		-f dockerfiles/harness/base.fips.Dockerfile \
    		.

.PHONY: docker-build-harness-ansible-fips
docker-build-harness-ansible-fips: docker-build-harness-base-fips ## build fips ansible docker harness image
	docker build \
			--no-cache \
		  	--build-arg=HARNESS_IMAGE_TAG="latest" \
    	  	-t harness-fips \
    		-f dockerfiles/harness/ansible.fips.Dockerfile \
    		.

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

.PHONY: docker-build-agent-fips
docker-build-agent-fips: ## build docker fips agent image
	docker build \
    	  	-t deployment-agent-fips \
    		-f dockerfiles/agent/fips.Dockerfile \
    		.

velero-crds:
	@curl -L $(VELERO_CHART_URL) --output velero.tgz
	@tar zxvf velero.tgz velero/crds
	@mv velero/crds/* charts/deployment-operator/crds
	@rm -r velero.tgz velero

##@ Tests

.PHONY: test
test: envtest ## run tests
	@KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(GOPATH)/bin -p path)" go test $$(go list ./... | grep -v /e2e) -race -v -tags="cache"

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

.PHONY: controller-gen
controller-gen: TOOL = controller-gen
controller-gen: --tool

.PHONY: discovery
discovery: TOOL = discovery
discovery: --tool

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

.PHONY: mcpserver
mcpserver: ## build mcp server
	go build -o bin/mcpserver cmd/mcpserver/main.go

.PHONY: mcpserver-run
mcpserver-run: mcpserver ## run mcp server locally
	PLURAL_ACCESS_TOKEN=${PLURAL_ACCESS_TOKEN} \
	PLURAL_CONSOLE_URL=${PLURAL_CONSOLE_URL} \
	./bin/mcpserver

.PHONY: docker-build-mcpserver
docker-build-mcpserver: ## build mcp server docker image
	docker build \
		--build-arg=VERSION="0.0.0-dev" \
		-t ghcr.io/pluralsh/mcpserver:latest \
		-f dockerfiles/mcpserver/Dockerfile \
		.

.PHONY: docker-push-mcpserver
docker-push-mcpserver: docker-build-mcpserver ## push mcp server image
	docker push ghcr.io/pluralsh/mcpserver:latest

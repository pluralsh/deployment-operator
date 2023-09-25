### Common application/container details
PROJECT_NAME := deployment-operator
# Container registry details
IMAGE_REGISTRIES := ghcr.io
IMAGE_REPOSITORY := plural

API_DIRECTORY = $(ROOT_DIRECTORY)/api
COMMON_DIRECTORY = $(ROOT_DIRECTORY)/common
OPERATOR_DIRECTORY = $(ROOT_DIRECTORY)/operator
PROVIDER_DIRECTORY = $(ROOT_DIRECTORY)/providers
ARGOCD_PROVIDER_DIRECTORY = $(PROVIDER_DIRECTORY)/argocd
FAKE_PROVIDER_DIRECTORY = $(PROVIDER_DIRECTORY)/fake
PROVISIONER_DIRECTORY = $(ROOT_DIRECTORY)/provisioner
SYNCHRONIZER_DIRECTORY = $(ROOT_DIRECTORY)/synchronizer
TOOLS_DIRECTORY = $(ROOT_DIRECTORY)/tools
MODULE_DIRECTORIES := $(API_DIRECTORY) $(COMMON_DIRECTORY) $(OPERATOR_DIRECTORY) $(ARGOCD_PROVIDER_DIRECTORY) $(FAKE_PROVIDER_DIRECTORY) $(PROVISIONER_DIRECTORY) $(SYNCHRONIZER_DIRECTORY)

ifndef GOPATH
$(error $$GOPATH environment variable not set)
endif

ifeq (,$(findstring $(GOPATH)/bin,$(PATH)))
$(error $$GOPATH/bin directory is not in your $$PATH)
endif
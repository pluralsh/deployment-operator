### Common application/container details
PROJECT_NAME := deployment-operator
# Container registry details
IMAGE_REGISTRIES := ghcr.io
IMAGE_REPOSITORY := plural

AGENT_DIRECTORY = $(ROOT_DIRECTORY)/agent
COMMON_DIRECTORY = $(ROOT_DIRECTORY)/common
TOOLS_DIRECTORY = $(ROOT_DIRECTORY)/tools
MODULE_DIRECTORIES := $(AGENT_DIRECTORY) $(COMMON_DIRECTORY)

ifndef GOPATH
$(error $$GOPATH environment variable not set)
endif

ifeq (,$(findstring $(GOPATH)/bin,$(PATH)))
$(error $$GOPATH/bin directory is not in your $$PATH)
endif
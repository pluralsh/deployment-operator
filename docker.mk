# Docker build target for the official version
# Usage:
# 	- `make build` - deploys both latest and tagged version
#   - `make build APP_VERSION="latest"` - deploys only the latest version
#	- `make build APP_VERSION="xyz"` - deploys both latest and provided version
.PHONY: --build
--build: --ensure-variables-set
	@VERSIONS="latest $(APP_VERSION)" ; \
	if [ "$(APP_VERSION)" = "latest" ] ; then \
	  VERSIONS="latest" ; \
	fi ; \
	for REGISTRY in $(IMAGE_REGISTRIES) ; do \
  	for VERSION in $${VERSIONS} ; do \
  		echo "Building '$(APP_NAME):$${VERSION}' for the '$${REGISTRY}/$(IMAGE_REPOSITORY)' registry" ; \
		docker build \
			-f $(DOCKERFILE) \
			-t $${REGISTRY}/$(IMAGE_REPOSITORY)/$(APP_NAME):$${VERSION} \
			$(ROOT_DIRECTORY) ; \
  	done ; \
  done

.PHONY: --ensure-variables-set
--ensure-variables-set:
	@if [ -z "$(DOCKERFILE)" ]; then \
  	echo "DOCKERFILE variable not set" ; \
  	exit 1 ; \
  fi ; \
	if [ -z "$(APP_NAME)" ]; then \
		echo "APP_NAME variable not set" ; \
		exit 1 ; \
	fi ; \
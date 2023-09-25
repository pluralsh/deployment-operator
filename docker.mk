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
  	  if [ "$${VERSION}" != "latest" ] ; then \
      	echo "Checking if '$(APP_NAME):$${VERSION}' exists in the '$${REGISTRY}/$(IMAGE_REPOSITORY)' registry" ; \
      	docker pull $${REGISTRY}/$(IMAGE_REPOSITORY)/$(APP_NAME):$${VERSION} > /dev/null 2>&1 ; \
      	if [ $$? -eq 0 ] ; then \
      	   echo "This image already exists" ; \
      	   exit; \
      	fi ; \
      fi ; \
  		echo "Deploying '$(APP_NAME):$${VERSION}' to the '$${REGISTRY}/$(IMAGE_REPOSITORY)' registry" ; \
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
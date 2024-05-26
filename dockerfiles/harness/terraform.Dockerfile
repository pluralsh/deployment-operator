ARG TERRAFORM_IMAGE_TAG=1.8.2
ARG TERRAFORM_IMAGE=hashicorp/terraform:$TERRAFORM_IMAGE_TAG

ARG HARNESS_BASE_IMAGE_TAG=latest
ARG HARNESS_BASE_IMAGE_REPO=harness-base
ARG HARNESS_BASE_IMAGE=$HARNESS_BASE_IMAGE_REPO:$HARNESS_BASE_IMAGE_TAG

FROM $TERRAFORM_IMAGE as terraform
FROM $HARNESS_BASE_IMAGE as final

COPY --from=terraform /bin/terraform /bin/terraform
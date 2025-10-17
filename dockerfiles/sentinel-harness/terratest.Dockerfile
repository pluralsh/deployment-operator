ARG SENTINEL_HARNESS_BASE_IMAGE_TAG=latest
ARG SENTINEL_HARNESS_BASE_IMAGE_REPO=ghcr.io/pluralsh/sentinel-harness-base
ARG SENTINEL_HARNESS_BASE_IMAGE=$SENTINEL_HARNESS_BASE_IMAGE_REPO:$SENTINEL_HARNESS_BASE_IMAGE_TAG

ARG GO_VERSION=1.25

# Use sentinel harness base image
FROM ${SENTINEL_HARNESS_BASE_IMAGE} AS harness

FROM golang:${GO_VERSION}-alpine AS final
RUN apk update --no-cache

COPY --from=harness /sentinel-harness /usr/local/bin/sentinel-harness
# Change ownership of the harness binary to UID/GID 65532
RUN chown -R 65532:65532 /usr/local/bin/sentinel-harness

COPY dockerfiles/sentinel-harness/terratest /plural

# Switch to the nonroot user
USER 65532:65532

WORKDIR /plural

ENTRYPOINT ["sentinel-harness", "--working-dir=/plural"]
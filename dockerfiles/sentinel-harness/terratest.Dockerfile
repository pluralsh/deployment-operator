ARG SENTINEL_HARNESS_BASE_IMAGE_TAG=latest
ARG SENTINEL_HARNESS_BASE_IMAGE_REPO=ghcr.io/pluralsh/sentinel-harness-base
ARG SENTINEL_HARNESS_BASE_IMAGE=$SENTINEL_HARNESS_BASE_IMAGE_REPO:$SENTINEL_HARNESS_BASE_IMAGE_TAG

ARG GO_VERSION=1.25

# Use sentinel harness base image
FROM ${SENTINEL_HARNESS_BASE_IMAGE} AS harness

FROM golang:${GO_VERSION}-alpine AS final

# Define build arguments for multi-arch support
ARG TARGETOS
ARG TARGETARCH
ARG VERSION

ENV CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GOCACHE=/sentinel/.cache

# Create directories and fix permissions
RUN mkdir -p /sentinel/.cache && chown -R 65532:65532 /sentinel

COPY --from=harness /sentinel-harness /usr/local/bin/sentinel-harness
COPY --from=harness /kubectl /usr/local/bin/kubectl

# Ensure permissions are correct
RUN chmod +x /usr/local/bin/sentinel-harness /usr/local/bin/kubectl && \
    chown -R 65532:65532 /usr/local/bin/sentinel-harness

# Copy test files
COPY dockerfiles/sentinel-harness/terratest /sentinel

# Switch to the nonroot user
USER 65532:65532

WORKDIR /sentinel

ENTRYPOINT ["sentinel-harness", "--test-dir=/sentinel", "--output-dir=/plural"]
FROM golang:1.22-alpine3.19 as builder

ARG TARGETARCH
ARG TARGETOS
ARG VERSION

WORKDIR /workspace

# Retrieve application dependencies.
# This allows the container build to reuse cached dependencies.
# Expecting to copy go.mod and if present go.sum.
COPY go.* ./
RUN go mod download

COPY cmd/harness ./cmd/harness
COPY pkg ./pkg
COPY internal ./internal
COPY api ./api

RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build \
    -trimpath \
    -ldflags="-s -w -X github.com/pluralsh/deployment-operator/pkg/harness/environment.Version=${VERSION}" \
    -o /plural/harness \
    cmd/harness/main.go

FROM busybox:1.35.0-uclibc as environment

RUN mkdir /plural
RUN mkdir /tmp/plural

FROM hashicorp/terraform:1.8.2 as terraform

FROM gcr.io/distroless/base-debian12:nonroot as final

# Switch to the nonroot user
USER nonroot:nonroot

# Set up the environment
# 1. copy plural and tmp directories with proper permissions for the nonroot user
# 2. copy the static shell into base image <- TODO: shell should not be required for prod image
# 3. copy the harness binary
# 4. copy the terraform binary
COPY --chown=nonroot --from=environment /plural /plural
COPY --chown=nonroot --from=environment /tmp/plural /tmp
COPY --chown=nonroot --from=environment /bin/sh /bin/sh
COPY --from=builder /plural/harness /harness
COPY --from=terraform /bin/terraform /bin/terraform

ENTRYPOINT ["/harness", "--working-dir=plural"]

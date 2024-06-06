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

FROM cgr.dev/chainguard/wolfi-base:latest as final

RUN apk update --no-cache && apk add git

# Switch to the nonroot user
USER 65532:65532

# Set up the environment
# 3. copy the harness binary
# 4. copy the terraform binary
COPY --from=builder /plural/harness /harness

WORKDIR /plural

ENTRYPOINT ["/harness", "--working-dir=/plural"]

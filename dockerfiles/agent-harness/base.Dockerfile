FROM golang:1.21-alpine AS builder

ARG TARGETARCH
ARG TARGETOS  
ARG VERSION

WORKDIR /workspace

# Retrieve application dependencies
COPY go.* ./
RUN go mod download

COPY cmd/agent-harness ./cmd/agent-harness
COPY pkg ./pkg
COPY internal ./internal
COPY api ./api

# Build agent-harness binary
RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build \
    -trimpath \
    -ldflags="-s -w -X github.com/pluralsh/deployment-operator/pkg/harness/environment.Version=${VERSION}" \
    -o /agent-harness \
    cmd/agent-harness/main.go

FROM cgr.dev/chainguard/wolfi-base:latest

RUN apk update --no-cache && apk add --no-cache git curl jq

# Copy binaries before switching user to ensure proper permissions
COPY --from=builder /agent-harness /agent-harness
COPY --from=aquasec/trivy:latest /usr/local/bin/trivy /usr/local/bin/trivy
COPY --from=ghcr.io/pluralsh/mcpserver:latest /root/mcpserver /usr/local/bin/mcpserver

# Switch to the nonroot user
USER 65532:65532

WORKDIR /plural

ENTRYPOINT ["/agent-harness", "--working-dir=/plural"]
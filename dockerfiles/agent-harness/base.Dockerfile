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

# Switch to the nonroot user
USER 65532:65532

# Copy the agent-harness binary
COPY --from=builder /agent-harness /agent-harness
COPY --from=aquasec/trivy:latest /usr/local/bin/trivy /usr/local/bin/trivy

# TODO: Add MCP server binary when implemented  
# COPY --from=builder /plural-agent-mcp-server /usr/local/bin/

WORKDIR /plural

ENTRYPOINT ["/agent-harness", "--working-dir=/plural"]
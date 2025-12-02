FROM golang:1.25-alpine AS builder

ARG TARGETARCH
ARG TARGETOS  
ARG VERSION

WORKDIR /workspace

# Retrieve application dependencies
COPY go.* ./
RUN go mod download

COPY cmd/ ./cmd
COPY pkg ./pkg
COPY internal ./internal
COPY api ./api

# Build agent-harness binary
RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build \
    -trimpath \
    -ldflags="-s -w -X github.com/pluralsh/deployment-operator/pkg/agentrun-harness/environment.Version=${VERSION}" \
    -o /agent-harness \
    cmd/agent-harness/main.go

# Build the MCP server binary
RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build \
    -trimpath \
    -ldflags="-s -w -X github.com/pluralsh/deployment-operator/cmd/mcpserver/agent.Version=${VERSION}" \
    -o /mcpserver \
    cmd/mcpserver/agent/main.go

FROM debian:13-slim

RUN apt update && apt install -y git curl jq tar

# Copy binaries before switching user to ensure proper permissions
COPY --from=builder /agent-harness /agent-harness
COPY --from=builder /mcpserver /usr/local/bin/mcpserver

# Create the nonroot user with UID 65532
RUN groupadd -g 65532 nonroot && \
    useradd -u 65532 -g 65532 -m -s /bin/bash nonroot

WORKDIR /plural

COPY dockerfiles/agent-harness/.opencode /plural/.opencode
COPY dockerfiles/agent-harness/.claude /plural/.claude

RUN printf "#!/bin/sh\necho \${GIT_ACCESS_TOKEN}" > /plural/.git-askpass && \
    chmod +x /plural/.git-askpass && \
    git config --global core.askPass /plural/.git-askpass && \
    chown -R 65532:65532 /plural

# Switch to the nonroot user
USER 65532:65532

WORKDIR /plural

ENTRYPOINT ["/bin/sh", "-c", "GIT_ASKPASS=/plural/.git-askpass /agent-harness --working-dir=/plural \"$@\"", "--"]
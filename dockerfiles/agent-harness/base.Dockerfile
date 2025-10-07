FROM golang:1.25-alpine AS builder

ARG TARGETARCH
ARG TARGETOS  
ARG VERSION
ARG ARGS

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

FROM debian:13-slim

RUN apt update && apt install -y git curl jq tar

# Copy binaries before switching user to ensure proper permissions
COPY --from=builder /agent-harness /agent-harness
# TODO: use official release version
COPY --from=ghcr.io/pluralsh/mcpserver:sha-ea119c3 /root/mcpserver /usr/local/bin/mcpserver

WORKDIR /plural

RUN echo -e "#!/bin/sh\necho \$GIT_ACCESS_TOKEN" > /plural/.git-askpass && \
    chmod +x /plural/.git-askpass && \
    git config --global core.askPass /plural/.git-askpass && \
    chown -R 65532:65532 /plural

# Switch to the nonroot user
USER 65532:65532

WORKDIR /plural

ENTRYPOINT ["/bin/sh", "-c", "GIT_ASKPASS=/plural/.git-askpass /agent-harness --working-dir=/plural ${ARGS}"]
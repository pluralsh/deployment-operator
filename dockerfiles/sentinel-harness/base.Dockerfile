FROM golang:1.25-alpine AS builder

ARG TARGETARCH
ARG TARGETOS  
ARG VERSION

WORKDIR /workspace

# Retrieve application dependencies
COPY go.* ./
RUN go mod download

COPY cmd/sentinel-harness ./cmd/sentinel-harness
COPY pkg ./pkg
COPY internal ./internal
COPY api ./api

# Build agent-harness binary
RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build \
    -trimpath \
    -ldflags="-s -w -X github.com/pluralsh/deployment-operator/pkg/sentinel-harness/environment.Version=${VERSION}" \
    -o /sentinel-harness \
    cmd/sentinel-harness/main.go

FROM golang:1.25-alpine AS final

RUN apk update --no-cache
# Switch to the nonroot user
USER 65532:65532

# Set up the environment
# - copy the harness binary
COPY --from=builder /sentinel-harness /sentinel-harness

WORKDIR /plural

ENTRYPOINT ["/sentinel-harness", "--working-dir=/plural"]
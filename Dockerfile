FROM golang:1.23-alpine3.20 AS builder

ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY /cmd/agent cmd/agent
COPY /pkg pkg/
COPY /api api/
COPY /internal internal/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} GO111MODULE=on go build -a -o deployment-agent cmd/agent/*.go

FROM alpine:3.20
WORKDIR /workspace

RUN mkdir /.kube && chown 65532:65532 /.kube

COPY --from=builder /workspace/deployment-agent .
USER 65532:65532


ENTRYPOINT ["/workspace/deployment-agent"]

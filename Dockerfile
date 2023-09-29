FROM golang:1.21.1-alpine3.17 as builder

ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY /cmd/main.go main.go
COPY /pkg pkg/

ENV HELM_VERSION=v3.10.3

RUN apk add --update --no-cache curl ca-certificates unzip wget openssl build-base && \
    curl -L https://get.helm.sh/helm-${HELM_VERSION}-linux-${TARGETARCH}.tar.gz | tar xvz && \
    mv linux-${TARGETARCH}/helm /usr/local/bin/helm

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} GO111MODULE=on go build -a -o deployment-agent main.go

FROM alpine:3.18
WORKDIR /workspace

COPY --from=builder /usr/local/bin/helm /usr/local/bin/helm 
COPY --from=builder /workspace/deployment-agent .
USER 65532:65532
ENTRYPOINT ["/workspace/deployment-agent"]
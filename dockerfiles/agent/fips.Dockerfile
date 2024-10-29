ARG GO_FIPS_IMAGE_TAG=latest
ARG GO_FIPS_IMAGE_REPO=go-fips
ARG GO_FIPS_BASE_IMAGE=$GO_FIPS_IMAGE_REPO:$GO_FIPS_IMAGE_TAG

FROM ${GO_FIPS_BASE_IMAGE} AS builder

# Set environment variables for FIPS compliance
ENV OPENSSL_FIPS=1
ENV FIPS_MODE=true

# Set up Go environment
ENV CGO_ENABLED=1
ENV CC=gcc

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

RUN go install github.com/acardace/fips-detect@latest

# Build
RUN GOOS=linux GOARCH=${TARGETARCH} GO111MODULE=on go build -a -o deployment-agent cmd/agent/*.go


FROM registry.access.redhat.com/ubi8/ubi
WORKDIR /workspace

# Set environment variables for FIPS
ENV OPENSSL_FIPS=1
ENV FIPS_MODE=true

# Install required packages, including openssl and fips-initramfs
RUN yum install -y openssl podman && \
    yum clean all

# Enable FIPS mode
RUN fips-mode-setup --enable
RUN mkdir /.kube && chown 65532:65532 /.kube
COPY --from=builder /workspace/deployment-agent .
USER 65532:65532
ENTRYPOINT ["/workspace/deployment-agent"]
FROM golang:1.19.3-alpine3.15 as builder

ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY /cmd/driver/main.go deployment-argocd-provider.go
COPY /cmd/driver/cmd.go cmd.go
COPY /cmd/status-updater/main.go status-updater-argocd-provider.go
COPY /pkg pkg/



# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} GO111MODULE=on go build -a -o deployment-argocd-provider deployment-argocd-provider.go cmd.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} GO111MODULE=on go build -a -o status-updater-argocd-provider status-updater-argocd-provider.go
FROM gcr.io/distroless/static:nonroot
WORKDIR /

COPY --from=builder /workspace/deployment-argocd-provider .
COPY --from=builder /workspace/status-updater-argocd-provider .
USER 65532:65532
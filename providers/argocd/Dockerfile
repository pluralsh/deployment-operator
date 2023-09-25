FROM golang:1.21-alpine3.18  as builder

ARG TARGETARCH

WORKDIR /workspace

# Copy all the modules
COPY . .

WORKDIR providers/argocd
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -a -o deployment-argocd-provider cmd/driver/*
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -a -o status-updater-argocd-provider cmd/status-updater/*

FROM gcr.io/distroless/static:nonroot
WORKDIR /

COPY --from=builder /workspace/providers/argocd/deployment-argocd-provider /usr/bin
COPY --from=builder /workspace/providers/argocd/status-updater-argocd-provider /usr/bin
USER 65532:65532
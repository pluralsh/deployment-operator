# This Dockerfile builds Go FIPS with OpenSSL

ARG UBI_MINIMAL_VERSION="latest"
FROM registry.access.redhat.com/ubi8/ubi-minimal:${UBI_MINIMAL_VERSION} AS go
ARG GO_VERSION=1.23.2
ARG TARGETARCH
ARG PLATFORM_ARCH=amd64

WORKDIR /workspace

# Install FIPS-compliant OpenSSL
RUN microdnf --nodocs install yum && yum --nodocs -q update -y
RUN yum install --nodocs -y git openssl-devel glibc-devel tar gzip gcc make && yum clean all

# Set environment variables for FIPS compliance
ENV OPENSSL_FIPS=1
ENV FIPS_MODE=true


RUN curl -LO https://go.dev/dl/go${GO_VERSION}.linux-${PLATFORM_ARCH}.tar.gz && \
    tar -C /usr/ -xzf go${GO_VERSION}.linux-${PLATFORM_ARCH}.tar.gz

ENV PATH="$PATH:/usr/go/bin"

ARG GO_RELEASE_VERSION=${GO_VERSION}-2
RUN git clone \
    https://github.com/golang-fips/go \
    --branch go${GO_RELEASE_VERSION}-openssl-fips \
    --single-branch \
    --depth 1 \
    /tmp/go

RUN cd /tmp/go && \
    chmod +x scripts/* && \
    git config --global user.email "plural@plural.sh" && \
    git config --global user.name "plural" && \
    scripts/full-initialize-repo.sh && \
    pushd go/src && \
    CGO_ENABLED=1 ./make.bash && \
    popd && \
    mv go /usr/local/

RUN cd /usr/local/go/src && \
    rm -rf \
        /usr/local/go/pkg/*/cmd \
        /usr/local/go/pkg/bootstrap \
        /usr/local/go/pkg/obj \
        /usr/local/go/pkg/tool/*/api \
        /usr/local/go/pkg/tool/*/go_bootstrap \
        /usr/local/go/src/cmd/dist/dist \
        /usr/local/go/.git*

FROM registry.access.redhat.com/ubi8/ubi-minimal:${UBI_MINIMAL_VERSION}

RUN microdnf --nodocs install yum && yum --nodocs -q update -y
RUN yum install --nodocs -y openssl-devel glibc-devel tar gzip gcc make && yum clean all

COPY --from=go /usr/local/go /usr/local/go
ENV OPENSSL_FIPS=1
ENV FIPS_MODE=true
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH" && go install std
WORKDIR $GOPATH

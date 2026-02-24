ARG HARNESS_BASE_IMAGE_TAG=latest
ARG HARNESS_BASE_IMAGE_REPO=harness-base
ARG HARNESS_BASE_IMAGE=$HARNESS_BASE_IMAGE_REPO:$HARNESS_BASE_IMAGE_TAG
ARG PYTHON_VERSION=3.12

# Use harness base image
FROM ${HARNESS_BASE_IMAGE} AS harness

# Build Ansible from Python Image
FROM python:${PYTHON_VERSION}-alpine AS final
ARG ANSIBLE_VERSION=9.0.1

# Copy Harness bin from the Harness Image
COPY --from=harness /harness /usr/local/bin/harness

# Change ownership of the harness binary to UID/GID 65532
RUN chown -R 65532:65532 /usr/local/bin/harness

# Install runtime dependencies
RUN apk add --no-cache openssh-client

# Install build dependencies, Ansible
RUN apk add --no-cache --virtual .build-deps \
    gcc \
    musl-dev \
    libffi-dev \
    openssl-dev \
    make \
    build-base && \
    pip install --no-cache-dir ansible==${ANSIBLE_VERSION} && \
    apk del .build-deps

# Create nonroot group and user with a home directory
RUN addgroup --gid 65532 nonroot && \
    adduser --uid 65532 --ingroup nonroot --home /home/nonroot --shell /bin/sh --disabled-password --gecos "" nonroot && \
    mkdir -p /home/nonroot/.ansible && \
    chown -R 65532:65532 /home/nonroot

# Switch to the non-root user
USER 65532:65532

WORKDIR /plural

ENTRYPOINT ["harness", "--working-dir=/plural"]

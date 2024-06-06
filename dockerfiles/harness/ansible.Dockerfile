ARG HARNESS_BASE_IMAGE_TAG=latest
ARG HARNESS_BASE_IMAGE_REPO=harness-base
ARG HARNESS_BASE_IMAGE=$HARNESS_BASE_IMAGE_REPO:$HARNESS_BASE_IMAGE_TAG

ARG PYTHON_VERSION=3.10

# Use harness base image
FROM ${HARNESS_BASE_IMAGE} as harness

# Build Ansible from Python Image
FROM python:${PYTHON_VERSION}-alpine as final

<<<<<<< HEAD
RUN mkdir /plural
RUN mkdir /tmp/plural

=======
>>>>>>> origin/main
# Copy Harness bin from the Harness Image
COPY --from=harness /harness /usr/local/bin/harness

# Install build dependencies, Ansible, and openssh-client
ARG ANSIBLE_VERSION=9.0.0
RUN apk add --no-cache --virtual .build-deps \
    gcc \
    musl-dev \
    libffi-dev \
    openssl-dev \
    make \
    build-base && \
    pip install --no-cache-dir ansible==${ANSIBLE_VERSION} && \
    apk add --no-cache openssh-client && \
    apk del .build-deps

ARG PYTHON_VERSION

WORKDIR /plural

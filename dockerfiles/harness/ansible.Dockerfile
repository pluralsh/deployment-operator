ARG HARNESS_BASE_IMAGE_TAG=latest
ARG HARNESS_BASE_IMAGE_REPO=harness-base
ARG HARNESS_BASE_IMAGE=$HARNESS_BASE_IMAGE_REPO:$HARNESS_BASE_IMAGE_TAG

ARG PYTHON_VERSION=3.10

# Use harness base image
FROM ${HARNESS_BASE_IMAGE} as harness

# Build Ansible from Python Image
FROM python:${PYTHON_VERSION}-slim as final

# Copy Harness bin from the Harness Image
COPY --from=harness /harness /usr/local/bin/harness

# Install Ansible and openssh-client
ARG ANSIBLE_VERSION=9.0.0
RUN pip install --no-cache-dir ansible==${ANSIBLE_VERSION}
RUN apt-get update && apt-get install -y openssh-client && rm -rf /var/lib/apt/lists/*

ARG PYTHON_VERSION



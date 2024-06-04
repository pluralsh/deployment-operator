ARG HARNESS_BASE_IMAGE_TAG=latest
ARG HARNESS_BASE_IMAGE_REPO=harness-base
ARG HARNESS_BASE_IMAGE=$HARNESS_BASE_IMAGE_REPO:$HARNESS_BASE_IMAGE_TAG

ARG PYTHON_VERSION=3.10

# Use harness base image
FROM ${HARNESS_BASE_IMAGE} as harness

# Build Ansible from Python base image
FROM python:${PYTHON_VERSION}-slim as final
ARG ANSIBLE_VERSION=9.0.0

RUN pip install --no-cache-dir ansible==${ANSIBLE_VERSION}

ARG PYTHON_VERSION

# Copy harness bin from the harness image
COPY --from=harness /harness /usr/local/bin/harness


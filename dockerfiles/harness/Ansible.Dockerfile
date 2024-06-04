ARG ANSIBLE_VERSION=2.12
ARG PYTHON_VERSION=3.9

ARG HARNESS_BASE_IMAGE_TAG=latest
ARG HARNESS_BASE_IMAGE_REPO=harness-base
ARG HARNESS_BASE_IMAGE=$HARNESS_BASE_IMAGE_REPO:$HARNESS_BASE_IMAGE_TAG

# Build Ansible from Python base image
FROM python:${PYTHON_VERSION}-slim as ansible

RUN pip install --no-cache-dir ansible==${ANSIBLE_VERSION}

# Use harness base image
FROM ${HARNESS_BASE_IMAGE} as final

# Copy Ansible from the Python image
COPY --from=ansible /usr/local/bin/ansible /usr/local/bin/ansible
COPY --from=ansible /usr/local/lib/python${PYTHON_VERSION}/site-packages/ansible /usr/local/lib/python${PYTHON_VERSION}/site-packages/ansible

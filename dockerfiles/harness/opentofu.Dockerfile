ARG TOFU_IMAGE_TAG=1.7.1
ARG TOFU_IMAGE=ghcr.io/opentofu/opentofu:$TOFU_IMAGE_TAG

ARG HARNESS_BASE_IMAGE_TAG=sha-1eca71e
ARG HARNESS_BASE_IMAGE_REPO=harness-base
ARG HARNESS_BASE_IMAGE=$HARNESS_BASE_IMAGE_REPO:$HARNESS_BASE_IMAGE_TAG

FROM $TOFU_IMAGE AS tofu
FROM $HARNESS_BASE_IMAGE AS final

COPY --from=tofu /usr/local/bin/tofu /bin/terraform

USER root
ENV TF_CLI_CONFIG_FILE=/usr/local/etc/plrl.tfrc
COPY dockerfiles/harness/plrl.tfrc /usr/local/etc/plrl.tfrc
RUN chown 65532:65532 /usr/local/etc/plrl.tfrc

# Switch to the non-root user
USER 65532:65532

# Node image version is controlled in the agent-harness images themselves.

ARG AGENT_HARNESS_IMAGE_TAG=0.6.11-claude-1.0.128
ARG AGENT_HARNESS_IMAGE_REPO=docker.io/pluralsh/agent-harness
ARG AGENT_HARNESS_IMAGE=$AGENT_HARNESS_IMAGE_REPO:$AGENT_HARNESS_IMAGE_TAG

FROM $AGENT_HARNESS_IMAGE AS final

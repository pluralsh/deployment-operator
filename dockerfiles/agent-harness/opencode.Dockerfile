ARG NODE_IMAGE_TAG=latest
ARG NODE_IMAGE=cgr.dev/chainguard/node:$NODE_IMAGE_TAG
ARG AGENT_VERSION=latest

ARG AGENT_HARNESS_BASE_IMAGE_TAG=latest
ARG AGENT_HARNESS_BASE_IMAGE_REPO=ghcr.io/pluralsh/agent-harness-base
ARG AGENT_HARNESS_BASE_IMAGE=$AGENT_HARNESS_BASE_IMAGE_REPO:$AGENT_HARNESS_BASE_IMAGE_TAG

# Stage 1: Install OpenCode CLI from npm in Chainguard Node image
FROM $NODE_IMAGE AS node

# Switch to root temporarily to install global packages
USER root

# Install OpenCode CLI globally using npm
RUN npm install -g opencode-ai@$AGENT_VERSION

# Verify installation
RUN opencode --version

# Stage 2: Copy OpenCode CLI into agent-harness base
FROM $AGENT_HARNESS_BASE_IMAGE AS final

# Copy the OpenCode CLI from the Node.js image
COPY --from=node /usr/local/bin/opencode /usr/local/bin/opencode
COPY --from=node /usr/local/lib/node_modules/opencode-ai /usr/local/lib/node_modules/opencode-ai

# Copy Node.js runtime (needed to run the CLI)
COPY --from=node /usr/bin/node /usr/bin/node

# Ensure proper ownership for nonroot user
USER root
RUN chown -R 65532:65532 /usr/local/bin/opencode /usr/local/lib/node_modules/opencode-ai /usr/bin/node

# Switch back to nonroot user
USER 65532:65532

# The entrypoint remains the agent-harness binary
# The agent-harness will call the opencode CLI as needed

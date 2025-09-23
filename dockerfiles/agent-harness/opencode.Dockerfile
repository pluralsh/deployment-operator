ARG OPENCODE_NODE_IMAGE_TAG=latest
ARG OPENCODE_NODE_IMAGE=cgr.dev/chainguard/node:$OPENCODE_NODE_IMAGE_TAG

ARG AGENT_HARNESS_BASE_IMAGE_TAG=latest
ARG AGENT_HARNESS_BASE_IMAGE_REPO=ghcr.io/pluralsh/agent-harness-base
ARG AGENT_HARNESS_BASE_IMAGE=$AGENT_HARNESS_BASE_IMAGE_REPO:$AGENT_HARNESS_BASE_IMAGE_TAG

# Stage 1: Install OpenCode CLI from npm in Chainguard Node image
FROM $OPENCODE_NODE_IMAGE as opencode

# Switch to root temporarily to install global packages
USER root

# Install OpenCode CLI globally using npm
RUN npm install -g opencode-ai@latest

# Verify installation
RUN opencode --version

# Stage 2: Copy OpenCode CLI into agent-harness base
FROM $AGENT_HARNESS_BASE_IMAGE as final

# Copy the OpenCode CLI from the Node.js image
COPY --from=opencode /usr/local/bin/opencode /usr/local/bin/opencode
COPY --from=opencode /usr/local/lib/node_modules/opencode-ai /usr/local/lib/node_modules/opencode-ai

# Copy Node.js runtime (needed to run the CLI)
COPY --from=opencode /usr/bin/node /usr/bin/node

# Ensure proper ownership for nonroot user
USER root
RUN chown -R 65532:65532 /usr/local/bin/opencode /usr/local/lib/node_modules/opencode-ai /usr/bin/node

# Switch back to nonroot user
USER 65532:65532

# The entrypoint remains the agent-harness binary
# The agent-harness will call the opencode CLI as needed

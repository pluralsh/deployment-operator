ARG NODE_IMAGE_TAG=24
ARG NODE_IMAGE=node:${NODE_IMAGE_TAG}-slim
ARG AGENT_VERSION=latest

ARG AGENT_HARNESS_BASE_IMAGE_TAG=latest
ARG AGENT_HARNESS_BASE_IMAGE_REPO=ghcr.io/pluralsh/agent-harness-base
ARG AGENT_HARNESS_BASE_IMAGE=$AGENT_HARNESS_BASE_IMAGE_REPO:$AGENT_HARNESS_BASE_IMAGE_TAG

# Stage 1: Install Gemini CLI from npm in Chainguard Node image
FROM $NODE_IMAGE AS node

# Switch to root temporarily to install global packages
USER root

# Install Gemini CLI globally using npm
RUN yarn global add @google/gemini-cli@0.19.4 # TODO: Use $AGENT_VERSION once latest version will be fixed.

# Verify installation
RUN gemini --version

# Stage 2: Copy Gemini CLI into agent-harness base
FROM $AGENT_HARNESS_BASE_IMAGE AS final

# Copy the Gemini CLI from the Node.js image
COPY --from=node /usr/local/share/.config/yarn/global /usr/local/share/.config/yarn/global

# Copy Node.js runtime (needed to run the CLI)
COPY --from=node /usr/local/bin/node /usr/local/bin/node

# Ensure proper ownership for nonroot user
USER root
RUN ln -s /usr/local/share/.config/yarn/global/node_modules/@google/gemini-cli/dist/index.js /usr/local/bin/gemini
RUN chown -R 65532:65532 /usr/local/share/.config/yarn/global /usr/local/bin/gemini /usr/local/bin/node

# Switch back to nonroot user
USER 65532:65532

# The entrypoint remains the agent-harness binary
# The agent-harness will call the gemini CLI as needed

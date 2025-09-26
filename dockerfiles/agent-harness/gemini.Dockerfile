ARG GEMINI_NODE_IMAGE_TAG=latest
ARG GEMINI_NODE_IMAGE=cgr.dev/chainguard/node:$GEMINI_NODE_IMAGE_TAG

ARG AGENT_HARNESS_BASE_IMAGE_TAG=latest
ARG AGENT_HARNESS_BASE_IMAGE_REPO=ghcr.io/pluralsh/agent-harness-base
ARG AGENT_HARNESS_BASE_IMAGE=$AGENT_HARNESS_BASE_IMAGE_REPO:$AGENT_HARNESS_BASE_IMAGE_TAG

# Stage 1: Install Gemini CLI from npm in Chainguard Node image
FROM $GEMINI_NODE_IMAGE AS gemini

# Switch to root temporarily to install global packages
USER root

# Install Gemini CLI globally using npm
RUN npm install -g @google/gemini-cli@latest

# Verify installation
RUN gemini --version

# Stage 2: Copy Gemini CLI into agent-harness base
FROM $AGENT_HARNESS_BASE_IMAGE AS final

# Copy the Gemini CLI from the Node.js image
COPY --from=gemini /usr/local/bin/gemini /usr/local/bin/gemini
COPY --from=gemini /usr/local/lib/node_modules/@google/gemini-cli /usr/local/lib/node_modules/@google/gemini-cli

# Copy Node.js runtime (needed to run the CLI)
COPY --from=gemini /usr/bin/node /usr/bin/node

# Ensure proper ownership for nonroot user
USER root
RUN chown -R 65532:65532 /usr/local/bin/gemini /usr/local/lib/node_modules/@google/gemini-cli /usr/bin/node

# Switch back to nonroot user
USER 65532:65532

# The entrypoint remains the agent-harness binary
# The agent-harness will call the gemini CLI as needed

# Workstation container for E2E tests
# Contains: Ubuntu + Node.js + Claude Code + Confab CLI

FROM ubuntu:22.04

# Avoid interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install base dependencies
RUN apt-get update && apt-get install -y \
    curl \
    ca-certificates \
    git \
    jq \
    python3 \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js 24
RUN curl -fsSL https://deb.nodesource.com/setup_24.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code globally
RUN npm install -g @anthropic-ai/claude-code

# Install Python test dependencies
COPY e2e/requirements.txt /tmp/requirements.txt
RUN pip3 install --no-cache-dir -r /tmp/requirements.txt

# Copy confab binary (built for linux by Makefile)
COPY e2e/containers/workstation/confab /usr/local/bin/confab
RUN chmod +x /usr/local/bin/confab

# Create a non-root user for running tests
RUN useradd -m -s /bin/bash testuser
USER testuser
WORKDIR /home/testuser

# Create Claude directory structure
RUN mkdir -p /home/testuser/.claude/projects

# Copy scripts
COPY --chown=testuser:testuser e2e/containers/workstation/scripts/ /opt/e2e/
RUN chmod +x /opt/e2e/*.sh

# Set entrypoint
ENTRYPOINT ["bash", "/opt/e2e/entrypoint.sh"]
CMD ["bash"]

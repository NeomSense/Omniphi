# Omniphi Validator Node Docker Image
# This container runs a single validator node for the Omniphi blockchain

FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    jq \
    curl \
    bash

# Create validator user
RUN addgroup -g 1000 validator && \
    adduser -D -u 1000 -G validator validator

# Copy posd binary and entrypoint script
COPY posd /usr/local/bin/posd
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/posd && \
    chmod +x /usr/local/bin/entrypoint.sh

# Create directories
RUN mkdir -p /home/validator/.omniphi && \
    chown -R validator:validator /home/validator

# Set working directory
WORKDIR /home/validator

# Switch to validator user
USER validator

# Expose ports
# 26656: P2P
# 26657: RPC
# 26660: Prometheus
# 9090: gRPC
# 1317: REST API
EXPOSE 26656 26657 26660 9090 1317

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD curl -f http://localhost:26657/health || exit 1

# Use entrypoint script
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

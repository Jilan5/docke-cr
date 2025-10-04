# Multi-stage build for docker-cr
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN make build

# Final stage - runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    criu \
    docker-cli \
    ca-certificates \
    && rm -rf /var/cache/apk/*

# Create non-root user (though CRIU operations need root)
RUN addgroup -S dockercr && adduser -S dockercr -G dockercr

# Copy binary from builder
COPY --from=builder /app/bin/docker-cr /usr/local/bin/docker-cr

# Create working directory
WORKDIR /work

# Make sure binary is executable
RUN chmod +x /usr/local/bin/docker-cr

# Set up volumes for checkpoints
VOLUME ["/checkpoints"]

# Default command
ENTRYPOINT ["docker-cr"]
CMD ["--help"]

# Metadata
LABEL maintainer="Docker-CR Team"
LABEL description="Simple Docker Checkpoint/Restore Tool"
LABEL version="1.0.0"
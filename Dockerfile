FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy module definitions
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o /nexus-weaver ./cmd/nexus-weaver

# Use a lightweight alpine image for runtime
FROM alpine:latest

# Install necessary runtime dependencies
# - git: used by fs/git.go for AutoCommit and Rollback
# - curl: useful for debugging HTTP requests (e.g. LocalQwen test)
# Note: For Gemini CLI, you would need to install it in your image or mount it.
RUN apk add --no-cache ca-certificates git curl

WORKDIR /app

# Configure git to avoid errors when the runner executes auto-commit
RUN git config --global user.email "nexus-weaver@example.com" && \
    git config --global user.name "Nexus Weaver Auto"

# Copy the binary from the builder stage
COPY --from=builder /nexus-weaver /usr/local/bin/

ENTRYPOINT ["nexus-weaver"]

FROM golang:1.24-alpine AS builder

# テスト実行時に git を使うため、ビルダーステージにもインストール
RUN apk add --no-cache git && \
    git config --global user.email "nexus-weaver@test.local" && \
    git config --global user.name "Nexus Weaver Test" && \
    git config --global --add safe.directory /app

# バージョン情報（docker build --build-arg で上書き可能）
ARG VERSION=dev
ARG COMMIT=unknown

WORKDIR /app

# Copy module definitions
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with version info
RUN go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /nexus-weaver ./cmd/nexus-weaver

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

# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mcp-mariadb ./cmd/...

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /root

# Copy the binary from builder
COPY --from=builder /app/mcp-mariadb .

# Expose default port for SSE/HTTP transport
EXPOSE 9001

# Default to stdio transport for Claude Desktop
ENTRYPOINT ["./mcp-mariadb"]
CMD ["--transport", "stdio"]

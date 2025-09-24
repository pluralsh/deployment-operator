FROM golang:1.25.1-alpine AS builder

WORKDIR /workspace

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the MCP server binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mcpserver cmd/mcpserver/main.go

# Use a minimal base image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /workspace/mcpserver .

# Set environment variables for Plural credentials
ENV PLURAL_ACCESS_TOKEN=""
ENV PLURAL_CONSOLE_URL=""

# Expose stdio for MCP communication
# The MCP server uses stdio transport, so no network ports needed

# Run the MCP server
CMD ["./mcpserver"]

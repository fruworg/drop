# Build stage
FROM golang:1.24.1-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Generate templ templates
RUN go run github.com/a-h/templ/cmd/templ generate

# Build the application with static linking
# CGO_ENABLED=0: Disables CGO to create a statically linked binary
# -ldflags="-s -w": Strips debug information to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /go/bin/drop ./cmd/drop

# Final stage using scratch (minimal base image)
FROM scratch

# Copy the binary from builder stage
COPY --from=builder /go/bin/drop /app

# Copy configuration file
COPY config/config.json /config/config.json

# Copy CA certificates for HTTPS requests (if needed)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Create necessary directories
WORKDIR /

# Declare volumes for uploads and config
VOLUME ["/uploads", "/config"]

# Expose the port the server runs on
EXPOSE 8080

# Run the binary
ENTRYPOINT ["/app"]

# Build stage
FROM golang:1.24.1-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with static linking
# CGO_ENABLED=0: Disables CGO to create a statically linked binary
# -ldflags="-s -w": Strips debug information to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /go/bin/app

# Final stage using scratch (minimal base image)
FROM scratch

# Copy the binary from builder stage
COPY --from=builder /go/bin/app /app

# Copy configuration file
COPY config/config.json /config/config.json

# Copy CA certificates for HTTPS requests (if needed)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# No need to create the uploads directory as it will be
# created automatically when the volume is mounted
WORKDIR /

# Declare volumes for uploads and config
VOLUME ["/uploads", "/config"]

# Run the binary
ENTRYPOINT ["/app"]

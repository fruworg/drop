# Build stage
FROM golang:1.24.1-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Use go tool to run templ generator
RUN go run github.com/a-h/templ/cmd/templ generate
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /go/bin/drop ./cmd/drop

# Final stage using distroless
FROM gcr.io/distroless/static-debian11

# Define build arguments with defaults
ARG PUID=1000
ARG PGID=1000

COPY --from=builder /go/bin/drop /app
COPY config/config.json /config/config.json
WORKDIR /
VOLUME ["/uploads", "/config"]
EXPOSE 8080

USER ${PUID}:${PGID}
ENTRYPOINT ["/app"]

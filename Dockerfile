# Build stage
FROM golang:1.24.1-bullseye AS builder
WORKDIR /app

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential pkg-config libsqlite3-dev ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go run github.com/a-h/templ/cmd/templ generate

RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /app/drop ./cmd/drop
RUN chmod +x /app/drop && ls -la /app/drop



# Final stage using Debian slim
FROM debian:bullseye-slim
ARG PUID=1000
ARG PGID=1000
ARG PORT=8080

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        libsqlite3-0 ca-certificates && \
    rm -rf /var/lib/apt/lists/*

RUN mkdir -p /data /uploads /config && chown -R ${PUID}:${PGID} /data /uploads /config

WORKDIR /app
COPY --from=builder /app/drop /app/drop
COPY config/config.json /app/config/config.json

WORKDIR /
VOLUME ["/uploads", "/config", "/data"]
EXPOSE ${PORT}
USER ${PUID}:${PGID}
ENTRYPOINT ["/app/drop"]

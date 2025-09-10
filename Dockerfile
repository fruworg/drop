FROM golang:1.24.1-bullseye AS builder
WORKDIR /app

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential pkg-config libsqlite3-dev ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/client/ ./cmd/client/
RUN mkdir -p /app/binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/binaries/drop-linux-amd64 ./cmd/client
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o /app/binaries/drop-linux-arm64 ./cmd/client
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o /app/binaries/drop-darwin-amd64 ./cmd/client
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o /app/binaries/drop-darwin-arm64 ./cmd/client
RUN CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o /app/binaries/drop-windows-amd64.exe ./cmd/client
RUN chmod +x /app/binaries/*

COPY . .
RUN go run github.com/a-h/templ/cmd/templ generate
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /app/drop ./cmd/drop
RUN chmod +x /app/drop

FROM debian:bullseye-slim
ARG PUID=1000
ARG PGID=1000
ARG PORT=3000

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        libsqlite3-0 ca-certificates curl netcat-openbsd && \
    rm -rf /var/lib/apt/lists/*

RUN mkdir -p /data /uploads /config && chown -R ${PUID}:${PGID} /data /uploads /config

WORKDIR /app
COPY --from=builder /app/drop /app/drop
COPY --from=builder /app/binaries /app/binaries
COPY config/config.yaml /app/config/config.yaml

WORKDIR /
VOLUME ["/uploads", "/config", "/data"]
EXPOSE ${PORT}

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 3000 || exit 1

USER ${PUID}:${PGID}
ENTRYPOINT ["/app/drop"]

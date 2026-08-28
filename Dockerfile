# Build stage
FROM golang:1.27-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
ENV CGO_ENABLED=0 GOOS=linux
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build args for version info (CI/release)
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE
RUN BUILD_DATE=${BUILD_DATE:-$(date +%FT%T%z)} && \
    go build -ldflags "-w -s \
      -X main.Version=${VERSION} \
      -X main.Commit=${COMMIT} \
      -X main.BuildDate=${BUILD_DATE}" \
      -o stargate-suite ./cmd/suite

# Runtime stage
FROM alpine:3.22
# curl is used by the container HEALTHCHECK and by compose-level health probes.
# Upgrade the base packages before installing runtime dependencies so a cached
# Alpine point-release layer cannot retain security fixes published after it.
RUN apk upgrade --no-cache && \
    apk add --no-cache ca-certificates curl
COPY --from=builder /app/stargate-suite /bin/stargate-suite
# Config and the canonical compose file are embedded in the binary (go:embed),
# so the runtime image is self-contained and needs no source tree mounted.
EXPOSE 8085
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8085/ >/dev/null || exit 1
CMD ["stargate-suite", "serve"]

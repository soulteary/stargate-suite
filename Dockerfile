# Build stage
FROM golang:1.25-alpine3.22 AS builder
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
    go build -ldflags "-w -s" -o stargate-suite ./cmd/suite

# Runtime stage
FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/stargate-suite /bin/stargate-suite
EXPOSE 8085
CMD ["stargate-suite", "serve"]

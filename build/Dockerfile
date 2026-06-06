# Stage 1: Build
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
    -o /app/bin/api \
    ./cmd/api

# Stage 2: Runtime
FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -g 65532 -S nonroot && \
    adduser  -u 65532 -S nonroot -G nonroot

COPY --from=builder --chown=nonroot:nonroot /app/bin/api ./api

USER 65532:65532

EXPOSE 8080 6060 2112

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["./api"]

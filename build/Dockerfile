# Stage 1: Build
FROM golang:1.25-alpine AS builder

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

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/bin/api ./api

EXPOSE 8080 6060 2112

ENTRYPOINT ["./api"]

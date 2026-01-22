# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache \
    gcc \
    g++ \
    musl-dev \
    librdkafka-dev \
    pkgconfig \
    git

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY internal/ ./internal/

COPY services/image-compressor/ ./services/image-compressor/

RUN CGO_ENABLED=1 GOOS=linux go build \
    -tags musl \
    -ldflags='-w -s' \
    -o image-compressor \
    ./services/image-compressor/main.go

# Final stage
FROM alpine:3.19

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /build/image-compressor /image-compressor

ENTRYPOINT ["/image-compressor"]
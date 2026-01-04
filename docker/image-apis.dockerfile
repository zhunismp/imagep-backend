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

COPY services/image-apis/ ./services/image-apis/

RUN CGO_ENABLED=1 GOOS=linux go build \
    -tags musl \
    -ldflags='-w -s' \
    -o image-apis \
    ./services/image-apis/main.go

# Final stage
FROM alpine:3.19

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /build/image-apis /image-apis

EXPOSE 8080

ENTRYPOINT ["/image-apis"]
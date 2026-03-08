# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go monorepo for **ImageP** — an asynchronous image compression service with two microservices communicating via Kafka, using Redis for state and GCS for blob storage.

## Build & Run

```bash
# Start dependencies (Kafka, Redis, Kafdrop, Redis Insight)
docker compose up -d

# Run image-apis service (HTTP API on :8080)
cd services/image-apis && go run main.go

# Run image-compressor service (Kafka consumer worker)
cd services/image-compressor && go run main.go
```

Local dev tools: Kafdrop at `localhost:9090`, Redis Insight at `localhost:5540`.

Both services require librdkafka (confluent-kafka-go uses cgo). On macOS: `brew install librdkafka`.

## Architecture

Two services in `services/`:

- **image-apis** — Fiber HTTP server handling file uploads to GCS, enqueuing Kafka messages, and polling Redis for compression status. Uses ants goroutine pool (50 workers) for parallel uploads.
- **image-compressor** — Kafka consumer (group: `cg-compressor`, topic: `process-image`) that downloads originals from GCS, compresses (JPEG quality:60, PNG encoder), uploads compressed files back, and updates Redis status. Parallelism based on `runtime.NumCPU()` with manual offset commits every 5 seconds.

Shared code lives in `internal/errors/` (custom AppError with HTTP status mapping).

### API Endpoints (image-apis)

- `POST /api/v1/upload` — Upload files, creates new task
- `POST /api/v1/upload/:taskId` — Upload to existing task
- `GET /api/v1/process/:taskId` — Trigger compression via Kafka
- `GET /api/v1/downloads/:taskId` — Poll task status/results
- `DELETE /api/v1/delete/:taskId/:img` — Delete a file

### Redis Key Patterns

- `task:{taskId}:files` — List of file IDs
- `task:{taskId}:file:{fileId}` — Hash (status, file_id, original_name, server_name, signed_url)
- `task:{taskId}` — Hash with completed/failed counters (set by compressor)

### GCS Path Convention

- Originals: `{taskId}/{fileId}`
- Compressed: `{taskId}/compressed/{fileId}`

## Code Patterns

- **Interface-driven**: All external dependencies (cache, blob, pubsub) defined as interfaces in `ports.go` files
- **Config via env**: Uses `sethvargo/go-envconfig` with struct tags
- **Structured logging**: `slog` throughout
- **Graceful shutdown**: Both services handle context cancellation and cleanup

## Service Internal Structure (both follow same layout)

```
services/{service}/
├── config/       # Env-based configuration structs
├── transport/    # HTTP server + handlers (image-apis only)
├── service/      # Business logic
├── pubsub/       # Kafka producer or consumer
└── store/
    ├── cache/    # Redis operations
    └── blob/     # GCS operations
```

## CI/CD

GitHub Actions workflows per service (`.github/workflows/`), triggered on changes to the service's directory or `internal/`. Builds Docker images tagged with git SHA, pushes to Docker Hub, then updates k8s deployment manifests (GitOps with ArgoCD).

## Infrastructure

- `infra/terraform/` — GCP resources (cluster, storage, IAM)
- `infra/k8s/` — Kubernetes manifests, ArgoCD app sets, Kafka operator, Redis, cert-manager

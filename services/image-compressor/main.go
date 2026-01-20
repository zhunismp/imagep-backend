package main

import (
	"context"
	"log/slog"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/zhunismp/imagep-backend/services/image-compressor/config"
	"github.com/zhunismp/imagep-backend/services/image-compressor/pubsub"
	"github.com/zhunismp/imagep-backend/services/image-compressor/service"
	"github.com/zhunismp/imagep-backend/services/image-compressor/store/blob"
	"github.com/zhunismp/imagep-backend/services/image-compressor/store/cache"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadCfg(ctx)
	if err != nil {
		slog.Error("Failed to read config", "error", err)
		return
	}

	redis, err := cache.NewRedisCache(cfg.CacheCfg)
	if err != nil {
		slog.Error("Failed to initialized redis cache")
		return
	}

	blobStorage, err := blob.NewGoogleCloudStorage("process-image")
	if err != nil {
		slog.Error("Failed to connect to blob storage", "error", err)
		return
	}

	compressorSvc := service.NewCompressorService(redis, blobStorage)
	consumer, err := pubsub.NewConsumerWorker(cfg, compressorSvc)
	if err != nil {
		slog.Error("Failed to start consumer", "error", err)
		return
	}
	consumer.Start(ctx, runtime.NumCPU())

	// Gracefully shutdown
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cancel()

	consumer.Shutdown(shutdownCtx)

	if err := blobStorage.Shutdown(shutdownCtx); err != nil {
		slog.Error("Failed to shutdown blob storage")
	}

	if err := redis.Shutdown(shutdownCtx); err != nil {
		slog.Error("Failed to shutdown redis cache")
	}
}

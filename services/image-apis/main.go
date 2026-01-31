package main

import (
	"context"
	"log/slog"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/zhunismp/imagep-backend/services/image-apis/config"
	"github.com/zhunismp/imagep-backend/services/image-apis/pubsub"
	"github.com/zhunismp/imagep-backend/services/image-apis/service"
	"github.com/zhunismp/imagep-backend/services/image-apis/store/blob"
	"github.com/zhunismp/imagep-backend/services/image-apis/store/cache"
	"github.com/zhunismp/imagep-backend/services/image-apis/transport"
)

var (
	pool     *ants.Pool
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadCfg(ctx)
	if err != nil {
		slog.Error("Failed to read config", "error", err)
	}

	if err := initialWorkerPool(); err != nil {
		slog.Error("Failed to initialize worker pool", "error", err)
	}

	redis, err := cache.NewRedisCache(cfg.CacheCfg)
	if err != nil {
		slog.Error("Failed to initialized redis cache")
		return
	}

	kafka, err := pubsub.NewProducer(cfg)
	if err != nil {
		slog.Error("Failed to initialize kafka", "error", err)
		return
	}

	blobStorage, err := blob.NewGoogleCloudStorage("process-image")
	if err != nil {
		slog.Error("Failed to connect to blob storage", "error", err)
		return
	}

	fps := service.NewFileProcessorService(kafka, redis, blobStorage, pool, cfg.PollingInterval, "process-image")
	ph := transport.NewProcessHandler(fps)

	httpServer := transport.NewHttpServer(cfg)
	httpServer.SetupRoute(ph)
	httpServer.Start()

	// Gracefully shutdown
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("Failed to shutdown http server", "error", err)
	}

	pool.Release()

	kafka.Shutdown()

	if err := blobStorage.Shutdown(shutdownCtx); err != nil {
		slog.Error("Failed to shutdown blob storage", "error", err)
	}

	if err := redis.Shutdown(shutdownCtx); err != nil {
		slog.Error("Failed to shutdown cache", "error", err)
	}
}

func initialWorkerPool() error {
	var once sync.Once
	var initErr error

	once.Do(func() {
		pool, initErr = ants.NewPool(
			50,
			ants.WithOptions(ants.Options{
				// Pre-allocate goroutines on startup for better performance
				PreAlloc: true,

				// Maximum capacity for blocking mode
				// If exceeded, Submit() will block until a worker is free
				MaxBlockingTasks: 1000,

				// Non-blocking mode: return error if pool is full
				// Set to true for critical paths where you can't afford to wait
				Nonblocking: false,

				// Panic handler - prevent one task from crashing entire pool
				PanicHandler: func(err any) {
					slog.Error("[POOL-PANIC] task crash with panic: %v", err)
				},

				// Worker idle timeout - kill idle workers after this duration
				ExpiryDuration: 10 * time.Second,
			}),
		)

		if initErr != nil {
			return
		}
	})

	return initErr
}

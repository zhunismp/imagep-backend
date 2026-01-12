package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"cloud.google.com/go/storage"
	"github.com/redis/go-redis/v9"
	"github.com/zhunismp/imagep-backend/services/image-compressor/config"
	"github.com/zhunismp/imagep-backend/services/image-compressor/pubsub"
	"github.com/zhunismp/imagep-backend/services/image-compressor/service"
)

var (
	shutdown []func(context.Context) error
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadCfg(ctx)
	if err != nil {
		slog.Error("Failed to read config", "error", err)
		return
	}

	redis := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	shutdown = append(shutdown, func(ctx context.Context) error { return redis.Close() })

	rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redis.Ping(rctx).Err(); err != nil {
		slog.Error("Failed to start redis", "error", err)
		gracefulShutdown()
		return
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		slog.Error("Failed to connect to blob storage", "error", err)
		gracefulShutdown()
		return
	}
	shutdown = append(shutdown, func(ctx context.Context) error { return client.Close() })
	bucket := client.Bucket("process-image")

	compressorSvc := service.NewCompressorService(redis, bucket)

	consumer, err := pubsub.NewConsumerWorker(cfg, compressorSvc)
	if err != nil {
		slog.Error("Failed to start consumer", "error", err)
		gracefulShutdown()
		return
	}
	shutdown = append(shutdown, consumer.Shutdown)
	consumer.Start(ctx, 50)

	// Gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	gracefulShutdown()
}

func gracefulShutdown() {
	var once sync.Once

	once.Do(func() {
		var wg sync.WaitGroup
		errCh := make(chan error, len(shutdown))

		// logging shutdown err
		done := make(chan struct{})
		go func() {
			for err := range errCh {
				if err != nil {
					slog.Error("Failed to shutdown", "error", err)
				}
			}

			close(done)
		}()

		for i := len(shutdown) - 1; i >= 0; i-- {
			idx := i
			wg.Go(func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				errCh <- shutdown[idx](shutdownCtx)
			})
		}

		wg.Wait()
		close(errCh)
		<-done

		slog.Info("Shutdown completed")
	})
}

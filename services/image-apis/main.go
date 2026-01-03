package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/panjf2000/ants"
	"github.com/redis/go-redis/v9"
	"github.com/zhunismp/imagep-backend/internal/kafka"
	"github.com/zhunismp/imagep-backend/services/image-apis/config"
	"github.com/zhunismp/imagep-backend/services/image-apis/service"
	"github.com/zhunismp/imagep-backend/services/image-apis/transport"
)

var (
	pool     *ants.Pool
	shutdown []func(context.Context) error
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadCfg(ctx)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := initialWorkerPool(); err != nil {
		log.Fatalf("Failed to initialize worker pool: %v", err)
	}
	shutdown = append(shutdown, releaseWorkerPool)

	redis := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	shutdown = append(shutdown, func(ctx context.Context) error { return redis.Close() })

	rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redis.Ping(rctx).Err(); err != nil {
		gracefulShutdown()
		return
	}

	kafka, err := kafka.NewKafkaProducer(cfg.KafkaAddress, "process-image")
	if err != nil {
		slog.Error("Failed to initialize kafka", "error", err)
		gracefulShutdown()
		return
	}
	shutdown = append(shutdown, kafka.Close)

	fps := service.NewFileProcessorService(kafka, redis, pool, cfg.PollingInterval)
	ph := transport.NewProcessHandler(fps, cfg.FrontendHost)

	httpServer := transport.NewHttpServer(cfg)
	httpServer.SetupRoute(ph)
	httpServer.Start()
	shutdown = append(shutdown, httpServer.Shutdown)

	// Gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	_ = <-quit
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

// Release are just fast, no need timeout handle logic here
func releaseWorkerPool(ctx context.Context) error {
	pool.Release()
	return nil
}

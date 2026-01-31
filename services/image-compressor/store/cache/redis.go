package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	apperrors "github.com/zhunismp/imagep-backend/internal/errors"
	"github.com/zhunismp/imagep-backend/services/image-compressor/config"
)

type redisCache struct {
	redisClient *redis.Client
}

func NewRedisCache(cfg config.CacheCfg) (FileCache, error) {
	redis := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx, cacnel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cacnel()
	if err := redis.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &redisCache{redisClient: redis}, nil
}

func (r *redisCache) GetFileById(ctx context.Context, taskId, fileId string) (File, error) {
	k := fileKey(taskId, fileId)

	m, err := r.redisClient.HGetAll(ctx, k).Result()
	if err != nil {
		return File{}, apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}
	if len(m) == 0 {
		return File{}, apperrors.New(apperrors.ErrCodeNotFound, "file not found", nil)
	}

	return File{
		Status:       FileStatus(m["status"]),
		FileID:       pick(m["file_id"], fileId),
		OriginalName: m["original_name"],
		ServerName:   m["server_name"],
		SignedURL:    m["signed_url"],
	}, nil
}

func (r *redisCache) PostProcessFailed(ctx context.Context, taskId, fileId string) error {
	fk := fileKey(taskId, fileId)
	tk := taskKey(taskId)

	exists, err := r.redisClient.Exists(ctx, fk).Result()
	if err != nil {
		return apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}
	if exists == 0 {
		return apperrors.New(apperrors.ErrCodeNotFound, "file not found", nil)
	}

	pipe := r.redisClient.TxPipeline()
	pipe.HSet(ctx, fk, "status", string(FileFailed))
	pipe.HIncrBy(ctx, tk, "failed", 1)

	_, err = pipe.Exec(ctx)
	
	return apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
}

func (r *redisCache) PostProcessCompleted(ctx context.Context, taskId, fileId, signedURL string) error {
	fk := fileKey(taskId, fileId)
	tk := taskKey(taskId)

	exists, err := r.redisClient.Exists(ctx, fk).Result()
	if err != nil {
		return apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}
	if exists == 0 {
		return apperrors.New(apperrors.ErrCodeNotFound, "file not found", nil)
	}

	pipe := r.redisClient.TxPipeline()

	pipe.HSet(ctx, fk,
		"status", string(FileCompleted),
		"signed_url", signedURL,
	)
	pipe.HIncrBy(ctx, tk, "completed", 1)

	_, err = pipe.Exec(ctx)

	return apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
}

func (r *redisCache) Shutdown(ctx context.Context) error {
	return r.redisClient.Close()
}

/* ----------------------- Helpers ----------------------- */

func fileKey(taskId, fileId string) string {
	return fmt.Sprintf("task:%s:file:%s", taskId, fileId)
}

func taskKey(taskId string) string {
	return fmt.Sprintf("task:%s", taskId)
}

func pick(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

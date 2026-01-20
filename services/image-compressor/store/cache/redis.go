package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	apperrors "github.com/zhunismp/imagep-backend/internal/errors"
	"github.com/zhunismp/imagep-backend/services/image-compressor/config"
)

type redisCache struct {
	redisClient *redis.Client
}

func NewRedisCache(cfg config.CacheCfg) (TaskStateCache, error) {
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

func (r *redisCache) GetTaskState(ctx context.Context, taskId string) (TaskState, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cacheKey := generateCacheKey(taskId)
	res, err := r.redisClient.JSONGet(ctx, cacheKey, "$").Result()
	if errors.Is(err, redis.Nil) {
		return TaskState{}, apperrors.New(apperrors.ErrCodeNotFound, "task id not found", err)
	} else if err != nil {
		return TaskState{}, apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}

	results := make([]TaskState, 1)
	if err := json.Unmarshal([]byte(res), &results); err != nil {
		return TaskState{}, apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}
	if len(results) == 0 {
		return TaskState{}, apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}

	return results[0], nil
}

func (r *redisCache) SaveTaskState(ctx context.Context, taskId string, taskState TaskState) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	cacheKey := generateCacheKey(taskId)
	if err := r.redisClient.JSONSet(ctx, cacheKey, "$", taskState).Err(); err != nil {
		return apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}

	return nil
}

func (r *redisCache) UpdateTaskStatus(ctx context.Context, taskId string, status TaskStatus) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	cacheKey := generateCacheKey(taskId)

	if err := r.redisClient.JSONSet(ctx, cacheKey, "$.status", status).Err(); err != nil {
		return apperrors.New(apperrors.ErrCodeInternal, "failed to update status", err)
	}

	return nil
}

func (r *redisCache) PostUpdateProcessCompleted(ctx context.Context, taskId string, filePath string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	cacheKey := generateCacheKey(taskId)

	taskState, err := r.GetTaskState(ctx, taskId)
	if err != nil {
		return apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}

	pipe := r.redisClient.Pipeline()
	pipe.Do(ctx, "JSON.ARRAPPEND", cacheKey, "$.processed", toJsonString(filePath))
	pipe.Do(ctx, "JSON.NUMINCRBY", cacheKey, "$.completed", 1)

	if taskState.Completed+1 == taskState.Total {
		if len(taskState.Failed) == 0 {
			pipe.Do(ctx, "JSON.SET", cacheKey, "$.status", toJsonString(string(Completed)))
		} else {
			pipe.Do(ctx, "JSON.SET", cacheKey, "$.status", toJsonString(string(Failed)))
		}
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return apperrors.New(apperrors.ErrCodeInternal, "failed to update completed file", err)
	}

	return nil
}

func (r *redisCache) PostUpdateProcessFailed(ctx context.Context, taskId string, filePath string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	cacheKey := generateCacheKey(taskId)

	taskState, err := r.GetTaskState(ctx, taskId)
	if err != nil {
		return apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}

	pipe := r.redisClient.Pipeline()
	pipe.Do(ctx, "JSON.ARRAPPEND", cacheKey, "$.failed", filePath)
	pipe.Do(ctx, "JSON.NUMINCRBY", cacheKey, "$.completed", 1)

	if taskState.Completed+1 == taskState.Total {
		pipe.Do(ctx, "JSON.SET", cacheKey, "$.status", toJsonString(string(Failed)))
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return apperrors.New(apperrors.ErrCodeInternal, "failed to update completed file", err)
	}

	return nil
}

func (r *redisCache) Shutdown(ctx context.Context) error {
	return r.redisClient.Close()
}

func generateCacheKey(taskId string) string {
	return fmt.Sprintf("tasks:%s", taskId)
}

func toJsonString(s string) string {
	return fmt.Sprintf(`"%s"`, s)
}

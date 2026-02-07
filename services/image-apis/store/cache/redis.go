package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	apperrors "github.com/zhunismp/imagep-backend/internal/errors"
	"github.com/zhunismp/imagep-backend/services/image-apis/config"
)

type redisCache struct {
	redisClient *redis.Client
	ttl         time.Duration
}

func NewRedisCache(cfg config.CacheCfg) (TaskCache, error) {
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

	return &redisCache{redisClient: redis, ttl: time.Hour}, nil
}

func (r *redisCache) BatchSaveFiles(ctx context.Context, taskId string, files []File) error {
	if len(files) == 0 {
		return nil
	}

	// Ensure task exists
	exists, err := r.redisClient.Exists(ctx, taskKey(taskId)).Result()
	if err != nil {
		return apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}
	if exists == 0 {
		return apperrors.New(apperrors.ErrCodeNotFound, "task not found", nil)
	}

	fileIDs := make([]interface{}, 0, len(files))
	for _, f := range files {
		fileIDs = append(fileIDs, f.FileID)
	}

	pipe := r.redisClient.Pipeline()

	pipe.RPush(ctx, taskFilesKey(taskId), fileIDs...)

	for _, f := range files {
		fk := taskFileKey(taskId, f.FileID)
		pipe.HSet(ctx, fk,
			"status", string(f.Status),
			"file_id", f.FileID,
			"original_name", f.OriginalName,
			"server_name", f.ServerName,
			"signed_url", f.SignedURL,
		)
		pipe.Expire(ctx, fk, r.ttl)
	}

	// Keep TTL alive while still active
	pipe.Expire(ctx, taskFilesKey(taskId), r.ttl)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}

	return nil
}

func (r *redisCache) GetFilesByTaskId(ctx context.Context, taskId string) ([]File, error) {
	exists, err := r.redisClient.Exists(ctx, taskKey(taskId)).Result()
	if err != nil {
		return nil, apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}
	if exists == 0 {
		return nil, apperrors.New(apperrors.ErrCodeNotFound, "task not found", nil)
	}

	ids, err := r.redisClient.LRange(ctx, taskFilesKey(taskId), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []File{}, nil
	}

	pipe := r.redisClient.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(ids))

	for i, id := range ids {
		cmds[i] = pipe.HGetAll(ctx, taskFileKey(taskId, id))
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}

	out := make([]File, 0, len(ids))
	for i, cmd := range cmds {
		m, err := cmd.Result()
		if err != nil || len(m) == 0 {
			// file hash expired/evicted
			continue
		}

		out = append(out, File{
			Status:       FileStatus(m["status"]),
			FileID:       pick(m["file_id"], ids[i]),
			OriginalName: m["original_name"],
			ServerName:   m["server_name"],
			SignedURL:    m["signed_url"],
		})
	}

	return out, nil
}

func (r *redisCache) Shutdown(ctx context.Context) error {
	return r.redisClient.Close()
}

/* ----------------------- Helpers ----------------------- */

func taskKey(taskId string) string             { return fmt.Sprintf("task:%s", taskId) }
func taskFilesKey(taskId string) string        { return fmt.Sprintf("task:%s:files", taskId) }
func taskFileKey(taskId, fileId string) string { return fmt.Sprintf("task:%s:file:%s", taskId, fileId) }
func pick(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

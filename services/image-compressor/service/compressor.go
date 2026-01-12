package service

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"time"

	"cloud.google.com/go/storage"
	"github.com/redis/go-redis/v9"
	"github.com/zhunismp/imagep-backend/internal/models"
)

type CompressorService struct {
	rc     *redis.Client
	bucket *storage.BucketHandle
}

func NewCompressorService(rc *redis.Client, bucket *storage.BucketHandle) *CompressorService {
	return &CompressorService{rc: rc, bucket: bucket}
}

func (c *CompressorService) Compress(ctx context.Context, taskId, filePath string) error {
	var err error
	defer func() {
		if err != nil {
			c.updateRedisOnFailure(ctx, taskId, filePath)
		}
	}()

	img, format, err := c.downloadAndDecode(ctx, taskId, filePath)
	if err != nil {
		return err
	}

	data, contentType, err := c.runCompression(ctx, img, format)
	if err != nil {
		return err
	}

	if err := c.uploadCompressed(ctx, taskId, filePath, data, contentType); err != nil {
        return err
	}

	return c.updateRedisOnSuccess(ctx, taskId, filePath)
}

func (c *CompressorService) downloadAndDecode(ctx context.Context, taskId, path string) (image.Image, string, error) {
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	fullPath := fmt.Sprintf("%s/%s", taskId, path)
	reader, err := c.bucket.Object(fullPath).NewReader(rctx)
	if err != nil {
		return nil, "", fmt.Errorf("read error: %w", err)
	}
	defer reader.Close()

	img, format, err := image.Decode(reader)
	return img, format, err
}

func (c *CompressorService) runCompression(ctx context.Context, img image.Image, format string) ([]byte, string, error) {
	compressor, err := getCompressor(format)
	if err != nil {
		return nil, "", err
	}

	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	data, err := compressor.Compress(cctx, img)
	if err != nil {
		return nil, "", fmt.Errorf("compression failed: %w", err)
	}

	return data, compressor.ContentType(), nil
}

func (c *CompressorService) uploadCompressed(ctx context.Context, taskId, path string, data []byte, contentType string) error {
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	destPath := fmt.Sprintf("%s/compressed/%s", taskId, path)
	writer := c.bucket.Object(destPath).NewWriter(wctx)
	writer.ContentType = contentType

	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

func (c *CompressorService) updateRedisOnSuccess(ctx context.Context, taskId, filePath string) error {
	key := fmt.Sprintf("tasks:%s", taskId)

	pipe := c.rc.Pipeline()
	pipe.Do(ctx, "JSON.ARRAPPEND", key, "$.processed", fmt.Sprintf(`"%s"`, filePath))
	pipe.Do(ctx, "JSON.NUMINCRBY", key, "$.completed", 1)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update redis on success: %w", err)
	}

	result, err := c.rc.Do(ctx, "JSON.GET", key).Result()
	if err != nil {
		return fmt.Errorf("failed to get task status: %w", err)
	}

	var task models.CacheTaskStatus
	if err := json.Unmarshal([]byte(result.(string)), &task); err != nil {
		return fmt.Errorf("failed to unmarshal task: %w", err)
	}

	if task.Completed == task.Total && len(task.Failed) == 0 {
		_, err = c.rc.Do(ctx, "JSON.SET", key, "$.status", `"completed"`).Result()
		if err != nil {
			return fmt.Errorf("failed to update status to completed: %w", err)
		}
	}

	return nil
}

func (c *CompressorService) updateRedisOnFailure(ctx context.Context, taskId, filePath string) error {
	key := fmt.Sprintf("tasks:%s", taskId)

	pipe := c.rc.Pipeline()
	pipe.Do(ctx, "JSON.ARRAPPEND", key, "$.failed", fmt.Sprintf(`"%s"`, filePath))
	pipe.Do(ctx, "JSON.NUMINCRBY", key, "$.completed", 1)
	pipe.Do(ctx, "JSON.SET", key, "$.status", `"failed"`)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update redis on failure: %w", err)
	}

	return nil
}

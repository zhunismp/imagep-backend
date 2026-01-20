package service

import (
	"bytes"
	"context"
	"fmt"
	"image"

	"github.com/zhunismp/imagep-backend/services/image-compressor/store/blob"
	"github.com/zhunismp/imagep-backend/services/image-compressor/store/cache"
)

type CompressorService struct {
	TaskStateCache cache.TaskStateCache
	blobStorage    blob.BlobStorage
}

func NewCompressorService(cache cache.TaskStateCache, blob blob.BlobStorage) *CompressorService {
	return &CompressorService{TaskStateCache: cache, blobStorage: blob}
}

func (c *CompressorService) Compress(ctx context.Context, taskId, filePath string) error {
	img, format, err := c.blobStorage.DownloadBlob(ctx, taskId, filePath)
	if err != nil {
		c.TaskStateCache.PostUpdateProcessFailed(ctx, taskId, filePath)
		return err
	}

	data, _, err := c.runCompression(ctx, img, format)
	if err != nil {
		c.TaskStateCache.PostUpdateProcessFailed(ctx, taskId, filePath)
		return err
	}

	path := fmt.Sprintf("%s/compressed/%s", taskId, filePath)
	reader := bytes.NewReader(data)

	if err := c.blobStorage.UploadBlob(ctx, path, reader); err != nil {
		c.TaskStateCache.PostUpdateProcessFailed(ctx, taskId, filePath)
		return err
	}

	signedFilePath, err := c.blobStorage.GetPresignedURL(path)
	if err != nil {
		c.TaskStateCache.PostUpdateProcessFailed(ctx, taskId, filePath)
		return err
	}

	return c.TaskStateCache.PostUpdateProcessCompleted(ctx, taskId, signedFilePath)
}

func (c *CompressorService) runCompression(ctx context.Context, img image.Image, format string) ([]byte, string, error) {
	compressor, err := getCompressor(format)
	if err != nil {
		return nil, "", err
	}

	data, err := compressor.Compress(ctx, img)
	if err != nil {
		return nil, "", fmt.Errorf("compression failed: %w", err)
	}

	return data, compressor.ContentType(), nil
}

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
	fileCache   cache.FileCache
	blobStorage blob.BlobStorage
}

func NewCompressorService(cache cache.FileCache, blob blob.BlobStorage) *CompressorService {
	return &CompressorService{fileCache: cache, blobStorage: blob}
}

func (c *CompressorService) CompressAndUploadBlob(ctx context.Context, taskId, filePath, fileId string) (err error) {
	defer func() {
		if err != nil {
			c.fileCache.PostProcessFailed(ctx, taskId, fileId)
		}
	}()

	img, format, err := c.blobStorage.DownloadBlob(ctx, taskId, filePath)
	if err != nil {
		return err
	}

	data, _, err := c.runCompression(ctx, img, format)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("%s/compressed/%s", taskId, fileId)
	if err := c.blobStorage.UploadBlob(ctx, path, bytes.NewReader(data)); err != nil {
		return err
	}

	signedFilePath, err := c.blobStorage.GetPresignedURL(path)
	if err != nil {
		return err
	}

	return c.fileCache.PostProcessCompleted(ctx, taskId, fileId, signedFilePath)
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

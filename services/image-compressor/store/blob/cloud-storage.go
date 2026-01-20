package blob

import (
	"context"
	"fmt"
	"image"
	"io"
	"time"

	"cloud.google.com/go/storage"
	apperrors "github.com/zhunismp/imagep-backend/internal/errors"
)

type googleCloudStorage struct {
	storage *storage.Client
	bucket  *storage.BucketHandle
}

func NewGoogleCloudStorage(bucketName string) (BlobStorage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	storage, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return &googleCloudStorage{bucket: storage.Bucket(bucketName), storage: storage}, nil
}

func (b *googleCloudStorage) UploadBlob(ctx context.Context, path string, content io.Reader) error {
	// Ensure uploading file completed when context was cancelled.
	uploadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
	defer cancel()

	obj := b.bucket.Object(path)
	w := obj.NewWriter(uploadCtx)

	if _, err := io.Copy(w, content); err != nil {
		return apperrors.New(apperrors.ErrCodeInternal, "failed to stream to GCS", err)
	}

	if err := w.Close(); err != nil {
		return apperrors.New(apperrors.ErrCodeInternal, "failed to finalize GCS upload", err)
	}

	return nil
}

func (b *googleCloudStorage) DownloadBlob(ctx context.Context, taskId, path string) (image.Image, string, error) {
	fullPath := fmt.Sprintf("%s/%s", taskId, path)
	reader, err := b.bucket.Object(fullPath).NewReader(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("read error: %w", err)
	}
	defer reader.Close()

	img, format, err := image.Decode(reader)
	return img, format, err
}

func (b *googleCloudStorage) GetPresignedURL(path string) (string, error) {
	opts := &storage.SignedURLOptions{
		Scheme:         storage.SigningSchemeV4,
		Method:         "GET",
		Expires:        time.Now().Add(1 * time.Hour),
	}

	url, err := b.bucket.SignedURL(path, opts)
	if err != nil {
		return "", apperrors.New(apperrors.ErrCodeInternal, "failed to sign URL", err)
	}

	return url, nil
}

func (b *googleCloudStorage) Shutdown(ctx context.Context) error {
	return b.storage.Close()
}

package blob

import (
	"context"
	"io"
	"log/slog"
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
	defer w.Close()

	startCopy := time.Now()
	if _, err := io.Copy(w, content); err != nil {
		return apperrors.New(apperrors.ErrCodeInternal, "failed to stream to GCS", err)
	}
	endCopyMs := time.Since(startCopy)

	startClose := time.Now()
	if err := w.Close(); err != nil {
		slog.Error("failed to upload", "error", err)
		return apperrors.New(apperrors.ErrCodeInternal, "failed to finalize GCS upload", err)
	}
	endCloseMs := time.Since(startClose)

	// TODO: use otel
	slog.Info("Uploading time to blob",
		"path", path,
		"copySecond", endCopyMs.Seconds(),
		"closeSecond", endCloseMs.Seconds(),
		"totalSecond", endCopyMs.Seconds() + endCloseMs.Seconds(),
	)

	return nil
}

func (b *googleCloudStorage) GetPresignedURL(path string) (string, error) {
	opts := &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  "GET",
		Expires: time.Now().Add(15 * time.Minute),
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

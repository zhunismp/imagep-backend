package blob

import (
	"context"
	"image"
	"io"
)

type BlobStorage interface {
	UploadBlob(ctx context.Context, path string, content io.Reader,) error
	DownloadBlob(ctx context.Context, taskId, path string) (image.Image, string, error)
	GetPresignedURL(path string) (string, error)
	Shutdown(ctx context.Context) error
}

package blob

import (
	"context"
	"io"
)

type BlobStorage interface {
	UploadBlob(ctx context.Context, path string, content io.Reader) error
	GetPresignedURL(path string) (string, error)
	Shutdown(ctx context.Context) error
}

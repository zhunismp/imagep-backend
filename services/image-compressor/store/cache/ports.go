package cache

import (
	"context"
)

type FileCache interface {
	GetFileById(ctx context.Context, taskId, fileId string) (File, error)
	PostProcessFailed(ctx context.Context, taskId, fileId string) error
	PostProcessCompleted(ctx context.Context, taskId, fileId, signedURL string) error
	Shutdown(ctx context.Context) error
}

type FileStatus string

const (
	FileUploaded  FileStatus = "uploaded"
	FileCompleted FileStatus = "completed"
	FileFailed    FileStatus = "failed"
)

type File struct {
	Status       FileStatus
	FileID       string
	OriginalName string
	ServerName   string
	SignedURL    string
}

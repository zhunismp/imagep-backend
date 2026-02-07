package cache

import (
	"context"
)

type TaskCache interface {
	BatchSaveFiles(ctx context.Context, taskId string, files []File) error
	GetFilesByTaskId(ctx context.Context, taskId string) ([]File, error)
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

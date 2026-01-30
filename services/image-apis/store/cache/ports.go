package cache

import (
	"context"
)

type TaskCache interface {
	CreateTask(ctx context.Context, taskId string, t Task) error
	UpdateTaskStatus(ctx context.Context, taskId string, status TaskStatus) error
	BatchSaveFiles(ctx context.Context, taskId string, files []File) error
	GetTaskById(ctx context.Context, taskId string) (Task, error)
	GetFilesByTaskId(ctx context.Context, taskId string) ([]File, error)
	Shutdown(ctx context.Context) error
}

type TaskStatus string

const (
	TaskPending          TaskStatus = "pending"
	TaskProcessing       TaskStatus = "processing"
	TaskRetrying         TaskStatus = "retrying"
	TaskPartialCompleted TaskStatus = "partial_completed"
	TaskCompleted        TaskStatus = "completed"
	TaskFailed           TaskStatus = "failed"
)

type Task struct {
	Status       TaskStatus
	Total        int
	Uploaded     int
	Completed    int
	Failed       int
	RetryAttempt int
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

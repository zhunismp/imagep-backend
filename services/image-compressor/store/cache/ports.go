package cache

import (
	"context"
)

type TaskStateCache interface {
	GetTaskState(ctx context.Context, taskId string) (TaskState, error)
	SaveTaskState(ctx context.Context, taskId string, taskState TaskState) error
	UpdateTaskStatus(ctx context.Context, taskId string, status TaskStatus) error
	PostUpdateProcessCompleted(ctx context.Context, taskId string, filePath string) error
	PostUpdateProcessFailed(ctx context.Context, taskId string, filePath string) error
	Shutdown(ctx context.Context) error
}

type TaskStatus string

const (
	Pending    TaskStatus = "pending"
	Processing TaskStatus = "processing"
	Retrying   TaskStatus = "retrying"
	Completed  TaskStatus = "completed"
	Failed     TaskStatus = "failed"
)

type TaskState struct {
	Status       TaskStatus `json:"status"`
	Total        int        `json:"total"`
	Completed    int        `json:"completed"`
	RetryAttempt int        `json:"retry_attempt"`
	Uploaded     []string   `json:"uploaded"`
	Processed    []string   `json:"processed"`
	Failed       []string   `json:"failed"`
}

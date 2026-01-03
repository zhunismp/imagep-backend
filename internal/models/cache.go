package models

type TaskStatus string

const (
	Pending    TaskStatus = "pending"
	Processing TaskStatus = "processing"
	Retrying   TaskStatus = "retrying"
	Completed  TaskStatus = "completed"
	Failed     TaskStatus = "failed"
)

type CacheTaskStatus struct {
	Status       TaskStatus `json:"status"`
	Total        int        `json:"total"`
	Completed    int        `json:"completed"`
	RetryAttempt int        `json:"retry_attempt"`
	Processed    []string   `json:"processed"`
	Failed       []string   `json:"failed"`
}

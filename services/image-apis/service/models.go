package service

type ProcessingResult struct {
	Status       string   `json:"status"`
	Total        int      `json:"total"`
	Completed    int      `json:"completed"`
	RetryAttempt int      `json:"retry_attempt"`
	Processed    []string `json:"processed"`
	Failed       []string `json:"failed"`

	Next int `json:"next,omitempty"`
}

package service

import (
	"context"
	"fmt"
	"log/slog"
	"mime/multipart"
	"path/filepath"
	"sync"

	"github.com/panjf2000/ants/v2"
	"github.com/thanhpk/randstr"
	"github.com/zhunismp/imagep-backend/services/image-apis/pubsub"
	"github.com/zhunismp/imagep-backend/services/image-apis/store/blob"
	"github.com/zhunismp/imagep-backend/services/image-apis/store/cache"
)

type fileProcessorService struct {
	processImageProducer pubsub.ProcessImageProducer
	taskStateCache       cache.TaskStateCache
	blobStorage          blob.BlobStorage
	wp                   *ants.Pool
	pollingInterval      int
}

// type uploadBlobResult struct {
// 	fileName string
// 	path     string
// 	err      error
// }

func NewFileProcessorService(
	processImageProducer pubsub.ProcessImageProducer,
	taskStateCache cache.TaskStateCache,
	blobStorage blob.BlobStorage,
	wp *ants.Pool,
	pi int,
	bucketName string,
) *fileProcessorService {
	return &fileProcessorService{
		processImageProducer: processImageProducer,
		taskStateCache:       taskStateCache,
		blobStorage:          blobStorage,
		wp:                   wp,
		pollingInterval:      pi,
	}
}

type uploadFileResult struct {
	path string
	err  error
}

func (f *fileProcessorService) Upload(ctx context.Context, taskId string, files []*multipart.FileHeader) (string, error) {
	var wg sync.WaitGroup

	resultCh := make(chan uploadFileResult, len(files))
	for _, file := range files {
		wg.Add(1)
		f.wp.Submit(func() {
			defer wg.Done()

			uniqueness := randstr.Hex(8)
			ext := filepath.Ext(file.Filename)
			path := fmt.Sprintf("%s/%s-%s%s", taskId, file.Filename, uniqueness, ext)
			reader, _ := file.Open() // TODO: Handle open file error

			err := f.blobStorage.UploadBlob(ctx, path, reader)
			resultCh <- uploadFileResult{path: path, err: err}
		})
	}

	go func(ctx context.Context) {
		wg.Wait()
		close(resultCh)
	}(ctx)

	uploaded := make([]string, len(files))
	for result := range resultCh {
		if err := result.err; err != nil {
			slog.Error("Failed to upload file to storage", "error", err)
			continue
		}

		uploaded = append(uploaded, result.path)
	}

	state := cache.TaskState{
		Status:    cache.Pending,
		Total:     len(files),
		Uploaded:  uploaded,
		Processed: []string{},
		Failed:    []string{},
	}

	if err := f.taskStateCache.SaveTaskState(ctx, taskId, state); err != nil {
		return "", err
	}

	return taskId, nil
}

func (f *fileProcessorService) Process(ctx context.Context, taskId string) error {
	state, err := f.taskStateCache.GetTaskState(ctx, taskId)
	if err != nil {
		return err
	}

	// TODO: Handle partial success
	for _, filePath := range state.Uploaded {
		msg := pubsub.ProcessImageMessage{TaskId: taskId, ImagePath: filePath}
		_ = f.processImageProducer.Produce(ctx, msg)
	}

	if err := f.taskStateCache.UpdateTaskStatus(ctx, taskId, cache.Processing); err != nil {
		return err
	}

	return nil
}

type ProcessingResult struct {
	Status       string   `json:"status"`
	Total        int      `json:"total"`
	Completed    int      `json:"completed"`
	RetryAttempt int      `json:"retry_attempt"`
	Processed    []string `json:"processed"`
	Failed       []string `json:"failed"`

	Next int `json:"next,omitempty"`
}

func (f *fileProcessorService) Download(ctx context.Context, taskId string) (ProcessingResult, error) {

	c, err := f.taskStateCache.GetTaskState(ctx, taskId)
	if err != nil {
		return ProcessingResult{}, err
	}

	result := ProcessingResult{
		Status:       string(c.Status),
		Total:        c.Total,
		Completed:    c.Completed,
		RetryAttempt: c.RetryAttempt,
		Processed:    c.Processed,
		Failed:       c.Failed,
	}

	if c.Status == cache.Processing {
		result.Next = f.pollingInterval
	}

	return result, nil
}

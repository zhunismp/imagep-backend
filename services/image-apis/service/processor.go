package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/panjf2000/ants/v2"
	"github.com/thanhpk/randstr"
	"github.com/zhunismp/imagep-backend/services/image-apis/pubsub"
	"github.com/zhunismp/imagep-backend/services/image-apis/store/blob"
	"github.com/zhunismp/imagep-backend/services/image-apis/store/cache"
	"golang.org/x/sync/errgroup"
)

type fileProcessorService struct {
	processImageProducer pubsub.ProcessImageProducer
	taskCache            cache.TaskCache
	blobStorage          blob.BlobStorage
	wp                   *ants.Pool
	pollingInterval      int
}

func NewFileProcessorService(
	processImageProducer pubsub.ProcessImageProducer,
	taskCache cache.TaskCache,
	blobStorage blob.BlobStorage,
	wp *ants.Pool,
	pi int,
	bucketName string,
) *fileProcessorService {
	return &fileProcessorService{
		processImageProducer: processImageProducer,
		taskCache:            taskCache,
		blobStorage:          blobStorage,
		wp:                   wp,
		pollingInterval:      pi,
	}
}

type uploadFileResult struct {
	Id             string
	FileName       string
	ServerFileName string
	Err            error
}

type UploadResponse struct {
	TaskId   string             `json:"task_id"`
	Uploaded []uploadFileResult `json:"uploaded"`
	Failed   []uploadFileResult `json:"failed"`
}

func (fp *fileProcessorService) Upload(ctx context.Context, taskId string, files []*multipart.FileHeader) (UploadResponse, error) {

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(8)

	resultCh := make(chan uploadFileResult, len(files))

	for _, file := range files {
		f := file
		id := randstr.Hex(8)
		ext := filepath.Ext(file.Filename)
		base := strings.TrimSuffix(f.Filename, ext)

		serverFileName := fmt.Sprintf("%s-%s%s", base, id, ext)
		blobPath := fmt.Sprintf("%s/%s", taskId, serverFileName)

		// TODO: worker pool
		g.Go(func() error {
			reader, _ := f.Open()
			defer reader.Close()

			err := fp.blobStorage.UploadBlob(ctx, blobPath, reader)
			resultCh <- uploadFileResult{
				Id:             id,
				FileName:       f.Filename,
				ServerFileName: serverFileName,
				Err:            err,
			}

			return nil
		})
	}

	_ = g.Wait()
	close(resultCh)

	uploaded := make([]uploadFileResult, 0, len(files))
	failed := make([]uploadFileResult, 0, len(files))
	for r := range resultCh {
		if r.Err != nil {
			failed = append(failed, r)
		} else {
			uploaded = append(uploaded, r)
		}
	}

	t := cache.Task{
		Status:   cache.TaskPending,
		Uploaded: len(uploaded),
	}

	// This method is idempotance, if task created it won't create new one.
	fp.taskCache.CreateTask(ctx, taskId, t)

	cf := make([]cache.File, 0, len(uploaded))
	for _, file := range uploaded {
		f := cache.File{
			Status:       cache.FileUploaded,
			FileID:       file.Id,
			OriginalName: file.FileName,
			ServerName:   file.ServerFileName,
		}

		cf = append(cf, f)
	}

	fp.taskCache.BatchSaveFiles(ctx, taskId, cf)

	return UploadResponse{
		TaskId:   taskId,
		Uploaded: uploaded,
		Failed:   failed,
	}, nil
}

func (f *fileProcessorService) Process(ctx context.Context, taskId string) error {
	files, err := f.taskCache.GetFilesByTaskId(ctx, taskId)
	if err != nil {
		return err
	}

	for _, file := range files {
		msg := pubsub.ProcessImageMessage{
			TaskId:    taskId,
			ImageId:   file.FileID,
			ImagePath: file.ServerName,
		}
		f.processImageProducer.Produce(ctx, msg)
	}

	if err := f.taskCache.UpdateTaskStatus(ctx, taskId, cache.TaskProcessing); err != nil {
		return err
	}

	return nil
}

type Progress struct {
	Total     int
	Completed int
	Failed    int
}

type FileResult struct {
	ServerFileName string
	FileName       string
	signedURL      string
}

type ProcessingResponse struct {
	TaskId   string
	Status   string
	Progress Progress

	Completed []FileResult
	Failed    []FileResult

	Next int
}

func (f *fileProcessorService) Download(ctx context.Context, taskId string) (ProcessingResponse, error) {
	task, err := f.taskCache.GetTaskById(ctx, taskId)
	if err != nil {
		return ProcessingResponse{}, err
	}

	progress := Progress{
		Total:     task.Total,
		Completed: task.Completed,
		Failed:    task.Failed,
	}

	files, err := f.taskCache.GetFilesByTaskId(ctx, taskId)
	if err != nil {
		return ProcessingResponse{}, err
	}

	completed := make([]FileResult, 0, len(files))
	failed := make([]FileResult, 0, len(files))

	for _, file := range files {
		fileResult := FileResult{
			ServerFileName: file.ServerName,
			FileName:       file.OriginalName,
			signedURL:      file.SignedURL,
		}

		if file.Status == cache.FileCompleted {
			completed = append(completed, fileResult)
		} else {
			failed = append(failed, fileResult)
		}
	}

	response := ProcessingResponse{
		TaskId:    taskId,
		Status:    string(task.Status),
		Progress:  progress,
		Completed: completed,
		Failed:    failed,
	}

	if task.Status != cache.TaskCompleted {
		response.Next = f.pollingInterval
	}

	return response, nil
}

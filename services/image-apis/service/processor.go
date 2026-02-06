package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/panjf2000/ants/v2"
	"github.com/thanhpk/randstr"
	apperrors "github.com/zhunismp/imagep-backend/internal/errors"
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
	Id             string `json:"fileId"`
	FileName       string `json:"filename"`
	ServerFileName string `json:"serverFilename"`
	Err            error  `json:"errMsg"`
}

type UploadResponse struct {
	TaskId   string             `json:"taskId"`
	Uploaded []uploadFileResult `json:"uploaded"`
	Failed   []uploadFileResult `json:"failed"`
}

func (fp *fileProcessorService) Upload(ctx context.Context, taskId string, files []*multipart.FileHeader) (UploadResponse, error) {
	if len(files) == 0 {
		return UploadResponse{}, apperrors.New(apperrors.ErrCodeValidation, "files is empty", nil)
	}

	g, wctx := errgroup.WithContext(ctx)
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

			err := fp.blobStorage.UploadBlob(wctx, blobPath, reader)
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

	// This method is idempotance, if task created it won't create new one.
	if err := fp.taskCache.CreateTask(ctx, taskId, cache.Task{}); err != nil {
		return UploadResponse{}, err
	}

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

	if err := fp.taskCache.BatchSaveFiles(ctx, taskId, cf); err != nil {
		return UploadResponse{}, err
	}

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

	return nil
}

type Progress struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

type FileResult struct {
	FileId         string `json:"fileId"`
	ServerFileName string `json:"serverFilename"`
	FileName       string `json:"filename"`
	SignedURL      string `json:"signedUrl,omitempty"`
}

type ProcessingResponse struct {
	TaskId      string       `json:"taskId"`
	Progress    Progress     `json:"progress"`
	Completed   []FileResult `json:"completed"`
	Failed      []FileResult `json:"failed"`
	IsCompleted bool         `json:"isCompleted"`
	Next        int          `json:"next,omitempty"`
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
			FileId:         file.FileID,
			ServerFileName: file.ServerName,
			FileName:       file.OriginalName,
			SignedURL:      file.SignedURL,
		}

		// ignore upload file for displaying
		switch file.Status {
		case cache.FileCompleted:
			completed = append(completed, fileResult)
		case cache.FileFailed:
			failed = append(failed, fileResult)
		}
	}

	isCompleted := task.Completed+task.Failed >= task.Total

	response := ProcessingResponse{
		TaskId:      taskId,
		Progress:    progress,
		Completed:   completed,
		Failed:      failed,
		IsCompleted: isCompleted,
	}

	if !isCompleted {
		response.Next = f.pollingInterval
	}

	return response, nil
}

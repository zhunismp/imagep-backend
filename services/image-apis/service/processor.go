package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"sync"
	"time"

	"github.com/panjf2000/ants"
	"github.com/redis/go-redis/v9"
	apperrors "github.com/zhunismp/imagep-backend/internal/errors"
	"github.com/zhunismp/imagep-backend/internal/kafka"
	"github.com/zhunismp/imagep-backend/internal/models"
)

type fileProcessorService struct {
	kc *kafka.KafkaProducer
	rc *redis.Client
	wp *ants.Pool
	pi int
}

func NewFileProcessorService(
	kc *kafka.KafkaProducer,
	rc *redis.Client,
	wp *ants.Pool,
	pi int,
) *fileProcessorService {
	return &fileProcessorService{
		kc: kc,
		rc: rc,
		wp: wp,
		pi: pi,
	}
}

func (f *fileProcessorService) Upload(ctx context.Context, taskId string, files []*multipart.FileHeader) (string, *apperrors.AppError) {

	// create new redis status block
	cacheKey := fmt.Sprintf("tasks:%s", taskId)
	data := models.CacheTaskStatus{
		Status:    models.Pending,
		Total:     len(files),
		Processed: []string{},
		Failed:    []string{},
	}

	// TODO: upload image to blob storage

	if err := f.rc.JSONSet(ctx, cacheKey, "$", data).Err(); err != nil {
		return "", apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
		// TODO: retry
	}

	return taskId, nil
}

func (f *fileProcessorService) Process(ctx context.Context, taskId string) *apperrors.AppError {

	cacheKey := fmt.Sprintf("tasks:%s", taskId)

	// simulate get file path from blob storage
	fp := []string{
		"blob1.jpg",
		"blob2.jpg",
		"blob3.jpg",
	}

	// sending kafka message to worker
	// TODO: partially handle kafka error
	var wg sync.WaitGroup
	errors := make([]error, len(fp))
	for i, filePath := range fp {
		wg.Add(1)
		p, idx := filePath, i

		f.wp.Submit(func() {
			defer wg.Done()

			timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			message := models.ProcessImageMessage{TaskId: taskId, ImagePath: p}

			if err := f.kc.Publish(timeoutCtx, message, taskId); err != nil {
				errors[idx] = err
			}
		})
	}
	wg.Wait()

	if err := f.rc.JSONSet(ctx, cacheKey, "$.status", models.Processing).Err(); err != nil {
		return apperrors.New(apperrors.ErrCodeInternal, "failed to update status", err)
	}

	return nil
}

func (f *fileProcessorService) Download(ctx context.Context, taskId string) (ProcessingResult, *apperrors.AppError) {
	cacheKey := fmt.Sprintf("tasks:%s", taskId)

	c, err := f.getRedisState(ctx, cacheKey)
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

	if c.Status == models.Processing {
		result.Next = f.pi
	}

	return result, nil
}

func (f *fileProcessorService) getRedisState(ctx context.Context, cacheKey string) (models.CacheTaskStatus, *apperrors.AppError) {

	// obtain all file paths from cache
	res, err := f.rc.JSONGet(ctx, cacheKey, "$").Result()
	if errors.Is(err, redis.Nil) {
		return models.CacheTaskStatus{}, apperrors.New(apperrors.ErrCodeNotFound, "task id not found", err)
	} else if err != nil {
		return models.CacheTaskStatus{}, apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
		// TODO: retry and fallback to get file paths directly from blob
	}

	// marshalling
	var c []models.CacheTaskStatus
	if err := json.Unmarshal([]byte(res), &c); err != nil {
		return models.CacheTaskStatus{}, apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}
	if len(c) == 0 {
		return models.CacheTaskStatus{}, apperrors.New(apperrors.ErrCodeInternal, "something went wrong", err)
	}

	return c[0], nil
}

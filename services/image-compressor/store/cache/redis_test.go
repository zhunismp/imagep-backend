package cache

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	apperrors "github.com/zhunismp/imagep-backend/internal/errors"
)

func setupTestRedis(t *testing.T) (*redisCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return &redisCache{redisClient: client}, mr
}

func seedFile(t *testing.T, mr *miniredis.Miniredis, taskId, fId string) {
	t.Helper()
	k := fileKey(taskId, fId)
	mr.HSet(k, "status", "uploaded", "file_id", fId, "original_name", "test.jpg", "server_name", "test-abc.jpg", "signed_url", "")
}

func TestGetFileById_ReturnsFileWhenExists(t *testing.T) {
	rc, mr := setupTestRedis(t)
	seedFile(t, mr, "task1", "f1")

	f, err := rc.GetFileById(context.Background(), "task1", "f1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.FileID != "f1" {
		t.Errorf("expected fileId 'f1', got %s", f.FileID)
	}
	if f.Status != FileUploaded {
		t.Errorf("expected status 'uploaded', got %s", f.Status)
	}
}

func TestGetFileById_ReturnsNotFoundWhenMissing(t *testing.T) {
	rc, _ := setupTestRedis(t)
	_, err := rc.GetFileById(context.Background(), "task1", "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != apperrors.ErrCodeNotFound {
		t.Errorf("expected NOT_FOUND, got %s", appErr.Code)
	}
}

func TestPostProcessFailed_UpdatesStatusAndIncrementsFailed(t *testing.T) {
	rc, mr := setupTestRedis(t)
	seedFile(t, mr, "task1", "f1")

	err := rc.PostProcessFailed(context.Background(), "task1", "f1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := mr.HGet(fileKey("task1", "f1"), "status")
	if status != "failed" {
		t.Errorf("expected status 'failed', got %s", status)
	}

	failed := mr.HGet(taskKey("task1"), "failed")
	if failed != "1" {
		t.Errorf("expected failed count '1', got %s", failed)
	}
}

func TestPostProcessFailed_ReturnsNotFoundWhenFileMissing(t *testing.T) {
	rc, _ := setupTestRedis(t)
	err := rc.PostProcessFailed(context.Background(), "task1", "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != apperrors.ErrCodeNotFound {
		t.Errorf("expected NOT_FOUND, got %s", appErr.Code)
	}
}

func TestPostProcessCompleted_UpdatesStatusAndSignedURLAndIncrementsCompleted(t *testing.T) {
	rc, mr := setupTestRedis(t)
	seedFile(t, mr, "task1", "f1")

	err := rc.PostProcessCompleted(context.Background(), "task1", "f1", "https://signed/url")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := mr.HGet(fileKey("task1", "f1"), "status")
	if status != "completed" {
		t.Errorf("expected status 'completed', got %s", status)
	}

	signedURL := mr.HGet(fileKey("task1", "f1"), "signed_url")
	if signedURL != "https://signed/url" {
		t.Errorf("expected signed_url 'https://signed/url', got %s", signedURL)
	}

	completed := mr.HGet(taskKey("task1"), "completed")
	if completed != "1" {
		t.Errorf("expected completed count '1', got %s", completed)
	}
}

func TestPostProcessCompleted_ReturnsNotFoundWhenFileMissing(t *testing.T) {
	rc, _ := setupTestRedis(t)
	err := rc.PostProcessCompleted(context.Background(), "task1", "missing", "https://url")
	if err == nil {
		t.Fatal("expected error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != apperrors.ErrCodeNotFound {
		t.Errorf("expected NOT_FOUND, got %s", appErr.Code)
	}
}

func TestFileKey_FormatsCorrectly(t *testing.T) {
	if got := fileKey("t1", "f1"); got != "task:t1:file:f1" {
		t.Errorf("expected 'task:t1:file:f1', got %s", got)
	}
}

func TestTaskKey_FormatsCorrectly(t *testing.T) {
	if got := taskKey("t1"); got != "task:t1" {
		t.Errorf("expected 'task:t1', got %s", got)
	}
}

func TestPick_ReturnsValueWhenNonEmpty(t *testing.T) {
	if got := pick("hello", "fb"); got != "hello" {
		t.Errorf("expected 'hello', got %s", got)
	}
}

func TestPick_ReturnsFallbackWhenEmpty(t *testing.T) {
	if got := pick("", "fb"); got != "fb" {
		t.Errorf("expected 'fb', got %s", got)
	}
}

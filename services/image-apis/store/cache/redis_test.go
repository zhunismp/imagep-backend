package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*redisCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return &redisCache{redisClient: client, ttl: time.Hour}, mr
}

func TestBatchSaveFiles_DoesNothingWhenFilesEmpty(t *testing.T) {
	rc, _ := setupTestRedis(t)
	err := rc.BatchSaveFiles(context.Background(), "task1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBatchSaveFiles_StoresFileIDsInListAndHashFields(t *testing.T) {
	rc, mr := setupTestRedis(t)
	ctx := context.Background()

	files := []File{
		{FileID: "f1", Status: FileUploaded, OriginalName: "a.jpg", ServerName: "a-f1.jpg"},
		{FileID: "f2", Status: FileUploaded, OriginalName: "b.png", ServerName: "b-f2.png"},
	}

	if err := rc.BatchSaveFiles(ctx, "task1", files); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids, err := mr.List(taskFilesKey("task1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 file IDs in list, got %d", len(ids))
	}

	status := mr.HGet(taskFileKey("task1", "f1"), "status")
	if status != "uploaded" {
		t.Errorf("expected status 'uploaded', got %s", status)
	}
}

func TestGetFilesByTaskId_ReturnsAllSavedFiles(t *testing.T) {
	rc, _ := setupTestRedis(t)
	ctx := context.Background()

	files := []File{
		{FileID: "f1", Status: FileUploaded, OriginalName: "a.jpg", ServerName: "a-f1.jpg"},
		{FileID: "f2", Status: FileUploaded, OriginalName: "b.png", ServerName: "b-f2.png"},
	}
	_ = rc.BatchSaveFiles(ctx, "task1", files)

	got, err := rc.GetFilesByTaskId(ctx, "task1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 files, got %d", len(got))
	}
}

func TestGetFilesByTaskId_ReturnsEmptySliceForUnknownTask(t *testing.T) {
	rc, _ := setupTestRedis(t)
	got, err := rc.GetFilesByTaskId(context.Background(), "unknown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d files", len(got))
	}
}

func TestTaskKey_FormatsCorrectly(t *testing.T) {
	if got := taskKey("abc"); got != "task:abc" {
		t.Errorf("expected 'task:abc', got %s", got)
	}
}

func TestTaskFilesKey_FormatsCorrectly(t *testing.T) {
	if got := taskFilesKey("abc"); got != "task:abc:files" {
		t.Errorf("expected 'task:abc:files', got %s", got)
	}
}

func TestTaskFileKey_FormatsCorrectly(t *testing.T) {
	if got := taskFileKey("abc", "f1"); got != "task:abc:file:f1" {
		t.Errorf("expected 'task:abc:file:f1', got %s", got)
	}
}

func TestPick_ReturnsValueWhenNonEmpty(t *testing.T) {
	if got := pick("hello", "fallback"); got != "hello" {
		t.Errorf("expected 'hello', got %s", got)
	}
}

func TestPick_ReturnsFallbackWhenEmpty(t *testing.T) {
	if got := pick("", "fallback"); got != "fallback" {
		t.Errorf("expected 'fallback', got %s", got)
	}
}

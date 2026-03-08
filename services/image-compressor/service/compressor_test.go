package service

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"testing"

	"github.com/zhunismp/imagep-backend/services/image-compressor/store/cache"
)

// --- Mocks ---

type mockFileCache struct {
	file         map[string]map[string]string
	failedCalls  []string
	completeCalls []struct{ taskId, fileId, url string }
}

func newMockFileCache() *mockFileCache {
	return &mockFileCache{
		file: make(map[string]map[string]string),
	}
}

func (m *mockFileCache) GetFileById(_ context.Context, _, fileId string) (cache.File, error) {
	return cache.File{FileID: fileId}, nil
}

func (m *mockFileCache) PostProcessFailed(_ context.Context, taskId, fileId string) error {
	m.failedCalls = append(m.failedCalls, taskId+":"+fileId)
	return nil
}

func (m *mockFileCache) PostProcessCompleted(_ context.Context, taskId, fileId, signedURL string) error {
	m.completeCalls = append(m.completeCalls, struct{ taskId, fileId, url string }{taskId, fileId, signedURL})
	return nil
}

func (m *mockFileCache) Shutdown(_ context.Context) error { return nil }

type mockBlobStorageCompressor struct {
	downloadErr error
	uploadErr   error
	presignErr  error
	presignURL  string
}

func (m *mockBlobStorageCompressor) DownloadBlob(_ context.Context, _, _ string) (image.Image, string, error) {
	if m.downloadErr != nil {
		return nil, "", m.downloadErr
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	return img, "jpeg", nil
}

func (m *mockBlobStorageCompressor) UploadBlob(_ context.Context, _ string, _ io.Reader) error {
	return m.uploadErr
}

func (m *mockBlobStorageCompressor) GetPresignedURL(_ string) (string, error) {
	if m.presignErr != nil {
		return "", m.presignErr
	}
	url := m.presignURL
	if url == "" {
		url = "https://signed/compressed"
	}
	return url, nil
}

func (m *mockBlobStorageCompressor) Shutdown(_ context.Context) error { return nil }

// --- Tests ---

func TestCompressAndUploadBlob_CompletesSuccessfullyForJPEG(t *testing.T) {
	fc := newMockFileCache()
	bs := &mockBlobStorageCompressor{presignURL: "https://signed/task1/compressed/f1"}
	svc := NewCompressorService(fc, bs)

	err := svc.CompressAndUploadBlob(context.Background(), "task1", "img.jpg", "f1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fc.completeCalls) != 1 {
		t.Fatalf("expected 1 complete call, got %d", len(fc.completeCalls))
	}
	if len(fc.failedCalls) != 0 {
		t.Errorf("expected 0 failed calls, got %d", len(fc.failedCalls))
	}
}

func TestCompressAndUploadBlob_MarksFailedWhenDownloadFails(t *testing.T) {
	fc := newMockFileCache()
	bs := &mockBlobStorageCompressor{downloadErr: fmt.Errorf("download error")}
	svc := NewCompressorService(fc, bs)

	err := svc.CompressAndUploadBlob(context.Background(), "task1", "img.jpg", "f1")
	if err == nil {
		t.Fatal("expected error")
	}
	if len(fc.failedCalls) != 1 {
		t.Errorf("expected 1 failed call, got %d", len(fc.failedCalls))
	}
}

func TestCompressAndUploadBlob_MarksFailedWhenUploadFails(t *testing.T) {
	fc := newMockFileCache()
	bs := &mockBlobStorageCompressor{uploadErr: fmt.Errorf("upload error")}
	svc := NewCompressorService(fc, bs)

	err := svc.CompressAndUploadBlob(context.Background(), "task1", "img.jpg", "f1")
	if err == nil {
		t.Fatal("expected error")
	}
	if len(fc.failedCalls) != 1 {
		t.Errorf("expected 1 failed call, got %d", len(fc.failedCalls))
	}
}

func TestCompressAndUploadBlob_MarksFailedWhenPresignFails(t *testing.T) {
	fc := newMockFileCache()
	bs := &mockBlobStorageCompressor{presignErr: fmt.Errorf("presign error")}
	svc := NewCompressorService(fc, bs)

	err := svc.CompressAndUploadBlob(context.Background(), "task1", "img.jpg", "f1")
	if err == nil {
		t.Fatal("expected error")
	}
	if len(fc.failedCalls) != 1 {
		t.Errorf("expected 1 failed call, got %d", len(fc.failedCalls))
	}
}

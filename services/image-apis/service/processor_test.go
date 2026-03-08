package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"testing"

	apperrors "github.com/zhunismp/imagep-backend/internal/errors"
	"github.com/zhunismp/imagep-backend/services/image-apis/pubsub"
	"github.com/zhunismp/imagep-backend/services/image-apis/store/blob"
	"github.com/zhunismp/imagep-backend/services/image-apis/store/cache"
)

// --- Mocks ---

type mockBlobStorage struct {
	uploadErr error
	uploads   []string
	failAfter int // if > 0, fail on the (failAfter+1)th call
	callCount int
}

func (m *mockBlobStorage) UploadBlob(_ context.Context, path string, _ io.Reader) error {
	m.callCount++
	m.uploads = append(m.uploads, path)
	if m.failAfter > 0 && m.callCount > m.failAfter {
		return m.uploadErr
	}
	if m.failAfter == 0 {
		return m.uploadErr
	}
	return nil
}
func (m *mockBlobStorage) GetPresignedURL(path string) (string, error) {
	return "https://signed/" + path, nil
}
func (m *mockBlobStorage) Shutdown(_ context.Context) error { return nil }

type mockTaskCache struct {
	files     []cache.File
	getErr    error
	saved     []cache.File
	savedTask string
}

func (m *mockTaskCache) BatchSaveFiles(_ context.Context, taskId string, files []cache.File) error {
	m.savedTask = taskId
	m.saved = append(m.saved, files...)
	return nil
}
func (m *mockTaskCache) GetFilesByTaskId(_ context.Context, _ string) ([]cache.File, error) {
	return m.files, m.getErr
}
func (m *mockTaskCache) Shutdown(_ context.Context) error { return nil }

type mockProducer struct {
	messages []pubsub.ProcessImageMessage
}

func (m *mockProducer) Produce(_ context.Context, msg pubsub.ProcessImageMessage) error {
	m.messages = append(m.messages, msg)
	return nil
}
func (m *mockProducer) Shutdown() {}

// --- Helpers ---

func makeFileHeader(name string) *multipart.FileHeader {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, name))
	h.Set("Content-Type", "application/octet-stream")
	part, _ := w.CreatePart(h)
	part.Write([]byte("fake image data"))
	w.Close()

	r := multipart.NewReader(&buf, w.Boundary())
	form, _ := r.ReadForm(1 << 20)
	return form.File["file"][0]
}

func newService(tc cache.TaskCache, bs blob.BlobStorage, pp pubsub.ProcessImageProducer) *fileProcessorService {
	return NewFileProcessorService(pp, tc, bs, nil, 2000, "test-bucket")
}

// --- Upload Tests ---

func TestUpload_ReturnsValidationErrorWhenFilesEmpty(t *testing.T) {
	svc := newService(&mockTaskCache{}, &mockBlobStorage{}, &mockProducer{})
	_, err := svc.Upload(context.Background(), "task1", nil)
	if err == nil {
		t.Fatal("expected error for empty files")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != apperrors.ErrCodeValidation {
		t.Errorf("expected VALIDATION_ERROR, got %s", appErr.Code)
	}
}

func TestUpload_SavesUploadedFilesToCacheAfterBlobUpload(t *testing.T) {
	tc := &mockTaskCache{}
	bs := &mockBlobStorage{}
	svc := newService(tc, bs, &mockProducer{})

	files := []*multipart.FileHeader{makeFileHeader("photo.jpg")}
	resp, err := svc.Upload(context.Background(), "task1", files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Uploaded) != 1 {
		t.Fatalf("expected 1 uploaded, got %d", len(resp.Uploaded))
	}
	if len(tc.saved) != 1 {
		t.Fatalf("expected 1 saved to cache, got %d", len(tc.saved))
	}
	if tc.saved[0].OriginalName != "photo.jpg" {
		t.Errorf("expected original name 'photo.jpg', got %s", tc.saved[0].OriginalName)
	}
}

func TestUpload_SeparatesFailedUploadsFromSuccessful(t *testing.T) {
	// Succeed on the first call, fail on the second
	bs := &mockBlobStorage{uploadErr: fmt.Errorf("upload failed"), failAfter: 1}
	tc := &mockTaskCache{}
	svc := newService(tc, bs, &mockProducer{})

	files := []*multipart.FileHeader{
		makeFileHeader("good.jpg"),
		makeFileHeader("bad.jpg"),
	}
	resp, err := svc.Upload(context.Background(), "task1", files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	total := len(resp.Uploaded) + len(resp.Failed)
	if total != 2 {
		t.Errorf("expected 2 total results, got %d", total)
	}
	if len(resp.Failed) != 1 {
		t.Errorf("expected 1 failed, got %d", len(resp.Failed))
	}
}

// --- Process Tests ---

func TestProcess_ProducesMessageForEachFileInTask(t *testing.T) {
	tc := &mockTaskCache{
		files: []cache.File{
			{FileID: "f1", ServerName: "img1.jpg"},
			{FileID: "f2", ServerName: "img2.png"},
		},
	}
	pp := &mockProducer{}
	svc := newService(tc, &mockBlobStorage{}, pp)

	err := svc.Process(context.Background(), "task1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pp.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(pp.messages))
	}
	if pp.messages[0].TaskId != "task1" {
		t.Errorf("expected taskId 'task1', got %s", pp.messages[0].TaskId)
	}
}

func TestProcess_ReturnsErrorWhenCacheFails(t *testing.T) {
	tc := &mockTaskCache{getErr: fmt.Errorf("redis down")}
	svc := newService(tc, &mockBlobStorage{}, &mockProducer{})

	err := svc.Process(context.Background(), "task1")
	if err == nil {
		t.Fatal("expected error when cache fails")
	}
}

// --- Download Tests ---

func TestDownload_ReturnsPollingIntervalWhenNotAllFilesProcessed(t *testing.T) {
	tc := &mockTaskCache{
		files: []cache.File{
			{FileID: "f1", Status: cache.FileCompleted},
			{FileID: "f2", Status: cache.FileUploaded},
		},
	}
	svc := newService(tc, &mockBlobStorage{}, &mockProducer{})

	resp, err := svc.Download(context.Background(), "task1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.IsCompleted {
		t.Error("expected IsCompleted to be false")
	}
	if resp.Next != 2000 {
		t.Errorf("expected Next=2000, got %d", resp.Next)
	}
}

func TestDownload_SetsIsCompletedWhenAllFilesProcessed(t *testing.T) {
	tc := &mockTaskCache{
		files: []cache.File{
			{FileID: "f1", Status: cache.FileCompleted},
			{FileID: "f2", Status: cache.FileFailed},
		},
	}
	svc := newService(tc, &mockBlobStorage{}, &mockProducer{})

	resp, err := svc.Download(context.Background(), "task1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsCompleted {
		t.Error("expected IsCompleted to be true")
	}
	if resp.Next != 0 {
		t.Errorf("expected Next=0, got %d", resp.Next)
	}
}

func TestDownload_SeparatesCompletedAndFailedFiles(t *testing.T) {
	tc := &mockTaskCache{
		files: []cache.File{
			{FileID: "f1", Status: cache.FileCompleted, ServerName: "img1.jpg", OriginalName: "a.jpg", SignedURL: "https://url1"},
			{FileID: "f2", Status: cache.FileFailed, ServerName: "img2.png", OriginalName: "b.png"},
		},
	}
	svc := newService(tc, &mockBlobStorage{}, &mockProducer{})

	resp, err := svc.Download(context.Background(), "task1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Completed) != 1 {
		t.Errorf("expected 1 completed, got %d", len(resp.Completed))
	}
	if len(resp.Failed) != 1 {
		t.Errorf("expected 1 failed, got %d", len(resp.Failed))
	}
}

func TestDownload_ExcludesUploadedFilesFromResults(t *testing.T) {
	tc := &mockTaskCache{
		files: []cache.File{
			{FileID: "f1", Status: cache.FileUploaded},
			{FileID: "f2", Status: cache.FileCompleted},
		},
	}
	svc := newService(tc, &mockBlobStorage{}, &mockProducer{})

	resp, err := svc.Download(context.Background(), "task1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Completed) != 1 {
		t.Errorf("expected 1 completed, got %d", len(resp.Completed))
	}
	if len(resp.Failed) != 0 {
		t.Errorf("expected 0 failed, got %d", len(resp.Failed))
	}
	if resp.Progress.Total != 2 {
		t.Errorf("expected total=2, got %d", resp.Progress.Total)
	}
}

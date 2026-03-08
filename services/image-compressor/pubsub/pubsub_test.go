package pubsub

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// --- Mock Service ---

type mockCompressorSvc struct {
	calls []struct{ taskId, filePath, fileId string }
}

func (m *mockCompressorSvc) CompressAndUploadBlob(_ context.Context, taskId, filePath, fileId string) error {
	m.calls = append(m.calls, struct{ taskId, filePath, fileId string }{taskId, filePath, fileId})
	return nil
}

// --- Tests ---

func TestProcessMessage_CallsServiceWithParsedFields(t *testing.T) {
	svc := &mockCompressorSvc{}
	cw := &ConsumerWorker{svc: svc}

	msg := ProcessImageMessage{
		TaskId:    "task1",
		ImageId:   "img1",
		ImagePath: "task1/photo.jpg",
	}
	data, _ := json.Marshal(msg)

	topic := "process-image"
	kafkaMsg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic},
		Value:          data,
	}

	err := cw.processMessage(context.Background(), kafkaMsg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(svc.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(svc.calls))
	}
	if svc.calls[0].taskId != "task1" {
		t.Errorf("expected taskId 'task1', got %s", svc.calls[0].taskId)
	}
	if svc.calls[0].fileId != "img1" {
		t.Errorf("expected fileId 'img1', got %s", svc.calls[0].fileId)
	}
	if svc.calls[0].filePath != "task1/photo.jpg" {
		t.Errorf("expected filePath 'task1/photo.jpg', got %s", svc.calls[0].filePath)
	}
}

func TestProcessMessage_SkipsInvalidJSON(t *testing.T) {
	svc := &mockCompressorSvc{}
	cw := &ConsumerWorker{svc: svc}

	topic := "process-image"
	kafkaMsg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic},
		Value:          []byte("not valid json"),
	}

	err := cw.processMessage(context.Background(), kafkaMsg)
	if err != nil {
		t.Fatalf("expected nil error for invalid JSON (skipped), got %v", err)
	}
	if len(svc.calls) != 0 {
		t.Errorf("expected 0 service calls for invalid JSON, got %d", len(svc.calls))
	}
}

func TestDeref_ReturnsEmptyStringForNilPointer(t *testing.T) {
	if got := deref(nil); got != "" {
		t.Errorf("expected empty string, got %s", got)
	}
}

func TestDeref_ReturnsValueForNonNilPointer(t *testing.T) {
	s := "hello"
	if got := deref(&s); got != "hello" {
		t.Errorf("expected 'hello', got %s", got)
	}
}

package pubsub

import "context"

type ProcessImageProducer interface {
	Produce(ctx context.Context, msg ProcessImageMessage) error
	Shutdown()
}

type ProcessImageMessage struct {
	TaskId    string
	ImageId   string
	ImagePath string
}

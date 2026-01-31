package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/zhunismp/imagep-backend/services/image-compressor/config"
)

type imageCompressorSvc interface {
	CompressAndUploadBlob(ctx context.Context, taskId, filePath, fileId string) error
}

type ConsumerWorker struct {
	kc  *kafka.Consumer
	cfg config.Config
	svc imageCompressorSvc

	wg     sync.WaitGroup
	taskCh chan *kafka.Message
	ackCh  chan ack

	commitDone chan struct{}
	pollDone   chan struct{}
}

type ack struct {
	topic     string
	partition int32
	offset    kafka.Offset
}

func NewConsumerWorker(cfg config.Config, svc imageCompressorSvc) (*ConsumerWorker, error) {
	consumer, err := NewKafkaConsumer(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	if err := consumer.Subscribe(cfg.KafkaConsumeTopic, nil); err != nil {
		return nil, fmt.Errorf("failed to subscribe %s topic: %w", cfg.KafkaConsumeTopic, err)
	}

	return &ConsumerWorker{
		kc:         consumer,
		cfg:        cfg,
		svc:        svc,
		taskCh:     make(chan *kafka.Message, 1000),
		ackCh:      make(chan ack, 1000),
		commitDone: make(chan struct{}),
		pollDone:   make(chan struct{}),
	}, nil
}

func (c *ConsumerWorker) Start(ctx context.Context, parallelWorkers int) error {
	if parallelWorkers <= 0 {
		return errors.New("parallelWorkers must be > 0")
	}

	c.startWorkers(parallelWorkers)
	c.startBackgroundCommit()

	slog.Info("consumer started", "topic", c.cfg.KafkaConsumeTopic, "workers", parallelWorkers)
	go c.startPollLoop(ctx)

	return nil
}

func (c *ConsumerWorker) Shutdown(ctx context.Context) error {
	slog.Info("shutting down consumer")

	select {
	case <-c.pollDone:
		// ok
	case <-ctx.Done():
		return ctx.Err()
	}

	close(c.taskCh)

	workersDone := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(workersDone)
	}()

	select {
	case <-workersDone:
		slog.Info("All workers stop gracefully")
	case <-ctx.Done():
		slog.Error("shutdown timeout while waiting for workers; forcing close")
		_ = c.kc.Close()
		return ctx.Err()
	}

	close(c.ackCh)

	select {
	case <-c.commitDone:
		slog.Info("Background commit worker stop successfully")
	case <-ctx.Done():
		slog.Error("shutdown timeout while waiting for commit; forcing close")
		_ = c.kc.Close()
		return ctx.Err()
	}

	return c.kc.Close()
}

func (c *ConsumerWorker) startPollLoop(ctx context.Context) {
	defer close(c.pollDone)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Poll loop stopped by root context", "err", ctx.Err())
			return

		default:
			ev := c.kc.Poll(100)
			if ev == nil {
				continue
			}

			switch e := ev.(type) {

			case *kafka.Message:
				// blocks if buffer full
				select {
				case <-ctx.Done():
					return
				case c.taskCh <- e:
				}

			case kafka.Error:
				slog.Error("consumer error", "error", e)
				if e.Code() == kafka.ErrAllBrokersDown {
					return
				}

			case kafka.AssignedPartitions:
				slog.Info("partitions assigned", "count", len(e.Partitions))
				if err := c.kc.Assign(e.Partitions); err != nil {
					slog.Error("failed to assign partitions", "error", err)
				}

			case kafka.RevokedPartitions:
				slog.Info("partitions revoked", "count", len(e.Partitions))
				if err := c.kc.Unassign(); err != nil {
					slog.Error("failed to unassign partitions", "error", err)
				}

			default:
				// ignore
			}
		}
	}
}

func (c *ConsumerWorker) startWorkers(parallelWorkers int) {
	for i := 0; i < parallelWorkers; i++ {
		c.wg.Add(1)

		go func(workerID int) {
			defer c.wg.Done()
			slog.Info("worker started", "worker_id", workerID)

			for msg := range c.taskCh {
				msgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				err := c.processMessage(msgCtx, msg)
				cancel()

				if err != nil {
					slog.Error("failed to process message",
						"error", err,
						"topic", deref(msg.TopicPartition.Topic),
						"partition", msg.TopicPartition.Partition,
						"offset", msg.TopicPartition.Offset,
					)
					continue
				}

				select {
				case c.ackCh <- ack{
					topic:     deref(msg.TopicPartition.Topic),
					partition: msg.TopicPartition.Partition,
					offset:    msg.TopicPartition.Offset,
				}:
				default:
					// ack buffer full or closed/draining; commit loop will catch up
				}
			}

			slog.Info("worker exited", "worker_id", workerID)
		}(i)
	}
}

func (c *ConsumerWorker) startBackgroundCommit() {
	type key struct {
		topic     string
		partition int32
	}

	pending := make(map[key]kafka.Offset)

	commit := func() {
		if len(pending) == 0 {
			return
		}
		if _, err := c.kc.Commit(); err != nil {
			slog.Error("failed to commit offsets", "error", err)
			return
		}
		slog.Info("committed offsets", "partitions", len(pending))
		pending = make(map[key]kafka.Offset)
	}

	go func() {
		defer close(c.commitDone)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		defer commit()

		for {
			select {
			case a, ok := <-c.ackCh:
				if !ok {
					return
				}

				k := key{topic: a.topic, partition: a.partition}
				next := a.offset + 1

				// only move forward
				if cur, ok := pending[k]; ok && next <= cur {
					continue
				}
				pending[k] = next

				topic := a.topic
				tp := kafka.TopicPartition{
					Topic:     &topic,
					Partition: a.partition,
					Offset:    next,
				}

				if _, err := c.kc.StoreOffsets([]kafka.TopicPartition{tp}); err != nil {
					slog.Error("failed to store offset",
						"error", err, "topic", a.topic, "partition", a.partition, "offset", next)
				}

			case <-ticker.C:
				commit()
			}
		}
	}()
}

func (c *ConsumerWorker) processMessage(ctx context.Context, msg *kafka.Message) error {
	var parsedMsg ProcessImageMessage
	if err := json.Unmarshal(msg.Value, &parsedMsg); err != nil {
		slog.Error("invalid message json; skipping", "error", err)
		return nil
	}

	return c.svc.CompressAndUploadBlob(ctx, parsedMsg.TaskId, parsedMsg.ImagePath, parsedMsg.ImageId)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

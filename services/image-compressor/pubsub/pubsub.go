package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/zhunismp/imagep-backend/internal/models"
	"github.com/zhunismp/imagep-backend/services/image-compressor/config"
)

type imageCompressorSvc interface {
	Compress(ctx context.Context, taskId, filePath string) error
}

type ConsumerWorker struct {
	kc  *kafka.Consumer
	cfg config.Config
	svc imageCompressorSvc
	wg  sync.WaitGroup

	taskCh chan *kafka.Message
	ackCh  chan ack
}

type ack struct {
	tp  kafka.TopicPartition
	off kafka.Offset
}

func NewConsumerWorker(cfg config.Config, svc imageCompressorSvc) (*ConsumerWorker, error) {
	consumer, err := NewKafkaConsumer(cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to create consumer: %v", err)
	}

	if err := consumer.Subscribe(cfg.KafkaConsumeTopic, nil); err != nil {
		return nil, fmt.Errorf("Failed to subscribe %s topic: %w", cfg.KafkaConsumeTopic, err)
	}

	tasks := make(chan *kafka.Message, 1000)
	acks := make(chan ack, 1000)

	return &ConsumerWorker{
		kc:     consumer,
		svc:    svc,
		cfg:    cfg,
		taskCh: tasks,
		ackCh:  acks,
	}, nil
}

func (c *ConsumerWorker) Start(ctx context.Context, parallelWorkers int) error {
	c.startWoker(ctx, parallelWorkers)
	c.startBackgroundCommit(ctx)

	for {
		select {
		case <-ctx.Done():
			// After context was done, call Shutdown() function to free resources
			return nil

		default:
			// Poll for events (100ms timeout)
			ev := c.kc.Poll(100)
			if ev == nil {
				continue
			}

			switch e := ev.(type) {
			case *kafka.Message:
				select {
				case <-ctx.Done():
					return ctx.Err()
				case c.taskCh <- e:
					// Message queued for processing
				}

			case kafka.Error:
				slog.Error("Consumer error", "error", e)
				if e.Code() == kafka.ErrAllBrokersDown {
					return fmt.Errorf("all brokers down")
				}

			case kafka.AssignedPartitions:
				slog.Info("Partitions assigned", "message", e)
				err := c.kc.Assign(e.Partitions)
				if err != nil {
					slog.Error("Failed to assign partitions", "error", err)
				}

			case kafka.RevokedPartitions:
				slog.Info("Partitions revoked", "message", e)
				err := c.kc.Unassign()
				if err != nil {
					slog.Error("Failed to unassign partitions", "error", err)
				}

			default:
				slog.Info("Ignored event", "message", e)
			}
		}
	}
}

func (c *ConsumerWorker) Shutdown(ctx context.Context) error {
	slog.Info("Shutting down consumer")

	// Stop accepting new work
	close(c.taskCh)

	// Stop Ack message



	// Wait for workers to finish
	c.wg.Wait()

	// // Final commit
	// _, err := c.kc.Commit()
	// if err != nil {
	// 	slog.Error("Final commit failed", "error", err)
	// }

	return c.kc.Close()
}

func (c *ConsumerWorker) startWoker(ctx context.Context, parallelWorkers int) {
	for i := range parallelWorkers {
		c.wg.Add(1)
		go func(workerID int) {
			defer c.wg.Done()
			slog.Info("Worker started", "worker_id", workerID)

			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-c.taskCh:
					if !ok {
						return // Channel closed
					}

					if err := c.processMessage(msg); err != nil {
						slog.Error("Failed to process message", "error", err)
						// retry, DLQ, or skip
						// Don't send ack on failure for at-least-once delivery
						return
					}

					// Signal successful processing
					c.ackCh <- ack{
						tp:  msg.TopicPartition,
						off: msg.TopicPartition.Offset,
					}

				}
			}
		}(i)
	}
}

func (c *ConsumerWorker) startBackgroundCommit(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	pending := make(map[kafka.TopicPartition]kafka.Offset)

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case ack := <-c.ackCh:
				// Store offset for successful message (next offset to read)
				nextOffset := ack.off + 1
				tp := kafka.TopicPartition{
					Topic:     ack.tp.Topic,
					Partition: ack.tp.Partition,
					Offset:    nextOffset,
				}

				pending[ack.tp] = nextOffset

				// Store offset locally
				_, err := c.kc.StoreOffsets([]kafka.TopicPartition{tp})
				if err != nil {
					log.Printf("Failed to store offset: %v", err)
				}

			case <-ticker.C:
				// Periodically commit stored offsets
				if len(pending) > 0 {
					offsets, err := c.kc.Commit()
					if err != nil {
						slog.Error("Failed to commit to kafka", "error", err)
					} else {
						slog.Info("Successfully commited", "partitions", len(offsets))
					}
					pending = make(map[kafka.TopicPartition]kafka.Offset)
				}
			}
		}
	}(ctx)
}

func (c *ConsumerWorker) processMessage(msg *kafka.Message) error {
	var parsedMsg models.ProcessImageMessage
	if err := json.Unmarshal(msg.Value, &parsedMsg); err != nil {
		return fmt.Errorf("Failed to unmarshall message: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := c.svc.Compress(ctx, parsedMsg.TaskId, parsedMsg.ImagePath); err != nil {
		return err
	}

	return nil
}

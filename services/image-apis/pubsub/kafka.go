package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/zhunismp/imagep-backend/services/image-apis/config"
)

type kafkaProducer struct {
	producer *kafka.Producer
	topic    string
}

func newKafkaProducer(cfg config.Config) (*kafka.Producer, error) {
	kafkaCfg := &kafka.ConfigMap{
		"bootstrap.servers":        cfg.KafkaAddress,
		"enable.idempotence":       true,       // Prevents duplicates
		"acks":                     "all",      // Wait for all replicas
		"retries":                  2147483647, // Max retries
		"reconnect.backoff.max.ms": 30000,

		// Maintain ordering during retries
		"max.in.flight.requests.per.connection": 1,

		// Optional throughput tuning
		"linger.ms":          5,
		"batch.num.messages": 10000,
	}

	return kafka.NewProducer(kafkaCfg)
}

func NewProducer(cfg config.Config) (ProcessImageProducer, error) {
	p, err := newKafkaProducer(cfg)
	if err != nil {
		return nil, err
	}

	producer := &kafkaProducer{producer: p, topic: "process-image"}
	producer.startDrain()

	return producer, nil
}

func (p *kafkaProducer) Produce(ctx context.Context, msg ProcessImageMessage) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	mmsg, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("Failed to marshal message")
	}

	kafkaMsg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &p.topic,
			Partition: kafka.PartitionAny,
		},
		Key:   []byte(msg.TaskId),
		Value: mmsg,
	}

	return p.producer.Produce(kafkaMsg, nil)
}

func (p *kafkaProducer) Shutdown() {
	p.producer.Close()
}

func (p *kafkaProducer) startDrain() {
	defer slog.Info("Producer event loop was closed")

	go func() {
		for e := range p.producer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					slog.Error("Delivery failed", "error", ev.TopicPartition.Error)
				} else {
					slog.Info("Message delivered",
						"topic", *ev.TopicPartition.Topic,
						"partition", ev.TopicPartition.Partition,
						"offset", ev.TopicPartition.Offset,
					)
				}
			case kafka.Error:
				slog.Error("Producer error", "error", ev)
			}
		}
	}()
}

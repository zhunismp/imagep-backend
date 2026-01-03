package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type KafkaProducer struct {
	producer *kafka.Producer
	topic    string
}

func NewKafkaProducer(kafkaAddr string, topic string) (*KafkaProducer, error) {
	if topic == "" {
		return &KafkaProducer{}, fmt.Errorf("Topic is empty")
	}

	p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": kafkaAddr})
	if err != nil {
		return &KafkaProducer{}, err
	}

	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					fmt.Printf("Delivery failed: %v\n", ev.TopicPartition)
				} else {
					fmt.Printf("Delivered message to %v\n", ev.TopicPartition)
				}
			}
		}
	}()

	return &KafkaProducer{
		producer: p,
		topic:    topic,
	}, nil
}

func (p *KafkaProducer) Publish(ctx context.Context, msg any, partitionKey string) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	message := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &p.topic, Partition: kafka.PartitionAny},
		Key:            []byte(partitionKey),
		Value:          payload,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.producer.Produce(message, nil)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (p *KafkaProducer) Close(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		p.producer.Close()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

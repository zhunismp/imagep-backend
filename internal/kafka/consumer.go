package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type KafkaConsumer[T any] struct {
	consumer  *kafka.Consumer
	topic     string
	msgChan   chan<- T
	readyChan chan struct{}
	isReady   bool
}

func NewKafkaConsumer[T any](kafkaAddr, group, topic string, msgChan chan<- T) (*KafkaConsumer[T], error) {
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  kafkaAddr,
		"group.id":           group,
		"enable.auto.commit": true,
		"auto.offset.reset":  "latest",
	})
	if err != nil {
		return &KafkaConsumer[T]{}, fmt.Errorf("Failed to create kafka consumer: %v", err)
	}

	return &KafkaConsumer[T]{
		consumer:  consumer,
		topic:     topic,
		msgChan:   msgChan,
		readyChan: make(chan struct{}),
		isReady:   false,
	}, nil
}

func (kc *KafkaConsumer[T]) Start(ctx context.Context) error {
	if err := kc.consumer.Subscribe(kc.topic, nil); err != nil {
		return fmt.Errorf("Failed to subscribe topic %s: %v", kc.topic, err)
	}

	kc.isReady = true
	close(kc.readyChan)

	fmt.Printf("Consumer is ready for reading message for topic: %s", kc.topic)

	go kc.consumeLoop(ctx)
	return nil
}

func (kc *KafkaConsumer[T]) consumeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Context cancelled, stop consuming")
			return
		default:
			msg, err := kc.consumer.ReadMessage(time.Second)
			if err != nil {
				if kafkaErr, ok := err.(kafka.Error); ok {
					// Timeout is expected when no messages are available
					if kafkaErr.Code() == kafka.ErrTimedOut {
						continue
					}
					fmt.Printf("Consumer error: %v (%v)", err, kafkaErr.Code())
				} else {
					fmt.Printf("Error reading message: %v", err)
				}
				continue
			}

			kc.handleMessage(msg)
		}
	}
}

func (kc *KafkaConsumer[T]) handleMessage(msg *kafka.Message) {
	if msg == nil {
		return
	}

	var parsedMsg T
	if err := json.Unmarshal(msg.Value, &parsedMsg); err != nil {
		fmt.Println("Failed to parsed struct")
		return
	}

	fmt.Printf("Received message from topic %s [partition %d] at offset %d",
		*msg.TopicPartition.Topic,
		msg.TopicPartition.Partition,
		msg.TopicPartition.Offset,
	)

	select {
	case kc.msgChan <- parsedMsg:
	default:
		fmt.Println("Message channel is full")
	}
}

func (kc *KafkaConsumer[T]) WaitReady(timeout time.Duration) error {
	select {
	case <-kc.readyChan:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("Timeout waiting for consumer to be ready")
	}
}

func (kc *KafkaConsumer[T]) IsReady() bool {
	return kc.isReady
}

func (kc *KafkaConsumer[T]) Close() error {
	if kc.consumer == nil {
		fmt.Print("No consumer to shutdown")
		return nil
	}

	if err := kc.consumer.Close(); err != nil {
		return fmt.Errorf("Failed to close consumer")
	}

	fmt.Println("Consumer was closed successfully")
	return nil
}

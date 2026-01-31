package pubsub

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/zhunismp/imagep-backend/services/image-compressor/config"
)

type ProcessImageMessage struct {
	TaskId    string
	ImageId   string
	ImagePath string
}

func NewKafkaConsumer(cfg config.Config) (*kafka.Consumer, error) {
	kCfg := &kafka.ConfigMap{
		"bootstrap.servers":             cfg.KafkaAddress,
		"group.id":                      cfg.KafkaConsumerGroup,
		"auto.offset.reset":             "earliest",
		"enable.auto.commit":            false, // Manual offset control
		"enable.auto.offset.store":      false, // Store after success only
		"partition.assignment.strategy": "cooperative-sticky",
		"session.timeout.ms":            30000,
		"max.poll.interval.ms":          300000, // 5 minutes
	}

	consumer, err := kafka.NewConsumer(kCfg)
	if err != nil {
		return nil, err
	}

	if err := consumer.SubscribeTopics([]string{cfg.KafkaConsumeTopic}, nil); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := prepareTopic(ctx, cfg.Environment, cfg.KafkaAddress); err != nil {
		return nil, err
	}

	return consumer, nil
}

func prepareTopic(ctx context.Context, env string, kafkaAddr string) error {

	if env != "development" {
		return nil
	}

	admin, err := kafka.NewAdminClient(&kafka.ConfigMap{"bootstrap.servers": kafkaAddr})
	if err != nil {
		return fmt.Errorf("error while creating admin: %v", err)
	}

	res, err := admin.CreateTopics(ctx, []kafka.TopicSpecification{{
		Topic:             "process-image",
		NumPartitions:     3,
		ReplicationFactor: 1, // 1 node for dev
	}})

	for _, r := range res {
		if r.Error.Code() != kafka.ErrNoError {
			errors.Join(err, fmt.Errorf("result for topic %s: %v", r.Topic, r.Error))
		}
	}

	return err
}

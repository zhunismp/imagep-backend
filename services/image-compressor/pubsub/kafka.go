package pubsub

import (
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

	return consumer, nil
}
